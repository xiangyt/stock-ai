// Package mcp 提供基于 Streamable HTTP 的 MCP Server，
// 对外暴露持仓管理、策略查看与策略选股工具，供 AI Agent（Claude、Cherry Studio 等）调用。
//
// 传输方式固定为 Streamable HTTP（MCP HTTP 传输规范），
// 监听端口由配置 mcp.port 指定，端点路径固定为 EndpointPath。
//
// 持仓按用户（uid）隔离：调用方传入企业微信用户ID（wework_id），
// 服务端通过 users.wework_id 关联出系统用户后再操作其持仓。
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/service"
	"stock-ai/utils"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const (
	// EndpointPath MCP Streamable HTTP 端点路径
	EndpointPath = "/mcp"

	// HeaderWeworkID 请求头名称：未传 wework_id 入参时用于携带企业微信用户ID
	HeaderWeworkID = "X-Wework-Id"

	// roleAdmin 管理员角色标识（users.role）
	roleAdmin = "admin"

	// screenConcurrency 选股数据构建与指标评估的最大并发数
	screenConcurrency = 50

	defaultPageSize = 50
	maxPageSize     = 100
	dateLayout      = "2006-01-02"
)

// StockMCPServer MCP 股票服务（持仓管理 / 策略查看 / 策略选股）
type StockMCPServer struct {
	cfg        config.MCPConfig
	mcp        *mcpserver.MCPServer
	httpServer *mcpserver.StreamableHTTPServer
	portfolio  *service.PortfolioService
	screen     *service.ScreenService
	registry   *indicator.Registry
}

// NewStockMCPServer 创建 MCP 服务器并注册持仓、策略工具。
//
// screenSvc 与 registry 均为 nil 时不注册选股工具（其余工具不受影响）。
func NewStockMCPServer(
	cfg config.MCPConfig,
	portfolioSvc *service.PortfolioService,
	screenSvc *service.ScreenService,
	registry *indicator.Registry,
) *StockMCPServer {
	s := &StockMCPServer{
		cfg:       cfg,
		portfolio: portfolioSvc,
		screen:    screenSvc,
		registry:  registry,
	}

	s.mcp = mcpserver.NewMCPServer(
		cfg.Name,
		cfg.Version,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithInstructions("A 股持仓与策略 MCP 服务（查看持仓 / 建仓 / 加仓 / 减仓 / 清仓 / 删除持仓 / 查看策略 / 按策略选股），传输方式为 Streamable HTTP。"+
			"用户身份按企业微信ID识别：入参 wework_id 可省略，缺省时取请求头 X-Wework-Id。"),
	)
	s.registerTools()

	s.httpServer = mcpserver.NewStreamableHTTPServer(
		s.mcp,
		mcpserver.WithEndpointPath(EndpointPath),
	)
	return s
}

// Start 启动 Streamable HTTP 服务（阻塞，需在 goroutine 中调用）。
func (s *StockMCPServer) Start() error {
	return s.httpServer.Start(fmt.Sprintf(":%d", s.cfg.Port))
}

// Shutdown 优雅停止 HTTP 服务。
func (s *StockMCPServer) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Addr 返回监听地址（如 :9101）。
func (s *StockMCPServer) Addr() string {
	return fmt.Sprintf(":%d", s.cfg.Port)
}

// ============================================================================
//  工具注册
// ============================================================================

func (s *StockMCPServer) registerTools() {
	s.mcp.AddTool(newListPositionsTool(), s.handleListPositions)
	s.mcp.AddTool(newListStrategiesTool(), s.handleListStrategies)
	s.mcp.AddTool(newOpenPositionTool(), s.handleOpenPosition)
	s.mcp.AddTool(newAddPositionTool(), s.handleAddPosition)
	s.mcp.AddTool(newReducePositionTool(), s.handleReducePosition)
	s.mcp.AddTool(newClosePositionTool(), s.handleClosePosition)
	s.mcp.AddTool(newDeletePositionTool(), s.handleDeletePosition)

	if s.screen != nil && s.registry != nil {
		s.mcp.AddTool(newRunStrategyScreenTool(), s.handleRunStrategyScreen)
	}
}

func newListPositionsTool() gomcp.Tool {
	return gomcp.NewTool("list_positions",
		gomcp.WithDescription("查看我的持仓列表，返回每只持仓的股票名称、代码、成本价与股数（不含行情）。"),
		gomcp.WithString("status", gomcp.Description("筛选状态：holding=持仓中（默认）、closed=已清仓、all=全部")),
		gomcp.WithNumber("page", gomcp.Description("页码，默认 1")),
		gomcp.WithNumber("page_size", gomcp.Description("每页数量，默认 50")),
		gomcp.WithString("wework_id", gomcp.Description("企业微信用户ID（必填，由调用方传入）"), gomcp.Required()),
	)
}

func newListStrategiesTool() gomcp.Tool {
	return gomcp.NewTool("list_strategies",
		gomcp.WithDescription("查看我的策略列表，返回策略ID、策略名称与是否使用中。"),
		gomcp.WithBoolean("using", gomcp.Description("true=仅返回正在使用中的策略（已订阅且订阅为启用状态）；false=返回全部可见策略"),
			gomcp.Required()),
		gomcp.WithString("wework_id", gomcp.Description("企业微信用户ID（必填，由调用方传入）"), gomcp.Required()),
	)
}

func newOpenPositionTool() gomcp.Tool {
	return gomcp.NewTool("open_position",
		gomcp.WithDescription("建仓：买入一只股票并创建持仓记录。"),
		gomcp.WithString("stock_code", gomcp.Description("股票代码（6位数字），如 000001"), gomcp.Required()),
		gomcp.WithNumber("quantity", gomcp.Description("买入数量（股）"), gomcp.Required()),
		gomcp.WithNumber("price", gomcp.Description("成交价格（元/股）"), gomcp.Required()),
		gomcp.WithString("trade_date", gomcp.Description("交易日期 YYYY-MM-DD，缺省为今天")),
		gomcp.WithString("note", gomcp.Description("备注")),
		gomcp.WithString("wework_id", gomcp.Description("企业微信用户ID（必填，由调用方传入）"), gomcp.Required()),
	)
}

func newAddPositionTool() gomcp.Tool {
	return gomcp.NewTool("add_position",
		gomcp.WithDescription("加仓：对已有持仓追加买入，平均成本按加权平均重算。"),
		gomcp.WithNumber("position_id", gomcp.Description("持仓ID（与 stock_code 二选一）")),
		gomcp.WithString("stock_code", gomcp.Description("股票代码（与 position_id 二选一，仅持仓中的股票有效）")),
		gomcp.WithNumber("quantity", gomcp.Description("买入数量（股）"), gomcp.Required()),
		gomcp.WithNumber("price", gomcp.Description("成交价格（元/股）"), gomcp.Required()),
		gomcp.WithString("trade_date", gomcp.Description("交易日期 YYYY-MM-DD，缺省为今天")),
		gomcp.WithString("note", gomcp.Description("备注")),
		gomcp.WithString("wework_id", gomcp.Description("企业微信用户ID（必填，由调用方传入）"), gomcp.Required()),
	)
}

func newReducePositionTool() gomcp.Tool {
	return gomcp.NewTool("reduce_position",
		gomcp.WithDescription("减仓：部分卖出持仓，平均成本不变。卖出全部请使用 close_position。"),
		gomcp.WithNumber("position_id", gomcp.Description("持仓ID（与 stock_code 二选一）")),
		gomcp.WithString("stock_code", gomcp.Description("股票代码（与 position_id 二选一，仅持仓中的股票有效）")),
		gomcp.WithNumber("quantity", gomcp.Description("卖出数量（股），必须小于当前持仓数"), gomcp.Required()),
		gomcp.WithNumber("price", gomcp.Description("成交价格（元/股）"), gomcp.Required()),
		gomcp.WithString("trade_date", gomcp.Description("交易日期 YYYY-MM-DD，缺省为今天")),
		gomcp.WithString("note", gomcp.Description("备注")),
		gomcp.WithString("wework_id", gomcp.Description("企业微信用户ID（必填，由调用方传入）"), gomcp.Required()),
	)
}

func newClosePositionTool() gomcp.Tool {
	return gomcp.NewTool("close_position",
		gomcp.WithDescription("清仓：全部卖出持仓并标记为已清仓。"),
		gomcp.WithNumber("position_id", gomcp.Description("持仓ID（与 stock_code 二选一）")),
		gomcp.WithString("stock_code", gomcp.Description("股票代码（与 position_id 二选一，仅持仓中的股票有效）")),
		gomcp.WithNumber("price", gomcp.Description("清仓成交价格（元/股）"), gomcp.Required()),
		gomcp.WithString("trade_date", gomcp.Description("交易日期 YYYY-MM-DD，缺省为今天")),
		gomcp.WithString("note", gomcp.Description("备注")),
		gomcp.WithString("wework_id", gomcp.Description("企业微信用户ID（必填，由调用方传入）"), gomcp.Required()),
	)
}

func newRunStrategyScreenTool() gomcp.Tool {
	return gomcp.NewTool("run_strategy_screen",
		gomcp.WithDescription("按指定策略立刻执行一次选股，返回选中股票的代码、名称与当前价格。"+
			"数据量大时耗时较长（全市场扫描），请避免频繁调用。"),
		gomcp.WithNumber("strategy_id", gomcp.Description("策略ID（必填，可用 list_strategies 查看）"), gomcp.Required()),
		gomcp.WithString("date", gomcp.Description("选股日期 YYYY-MM-DD 或 YYYYMMDD，缺省为最近交易日")),
		gomcp.WithString("wework_id", gomcp.Description("企业微信用户ID（必填，由调用方传入）"), gomcp.Required()),
	)
}

func newDeletePositionTool() gomcp.Tool {
	return gomcp.NewTool("delete_position",
		gomcp.WithDescription("删除持仓记录（软删除，通常用于清理已清仓的持仓）。"),
		gomcp.WithNumber("position_id", gomcp.Description("持仓ID（与 stock_code 二选一）")),
		gomcp.WithString("stock_code", gomcp.Description("股票代码（与 position_id 二选一）")),
		gomcp.WithString("wework_id", gomcp.Description("企业微信用户ID（必填，由调用方传入）"), gomcp.Required()),
	)
}

// ============================================================================
//  查看持仓
// ============================================================================

type positionItem struct {
	ID           uint     `json:"id"`
	StockCode    string   `json:"stock_code"`
	StockName    string   `json:"stock_name,omitempty"`
	Quantity     int      `json:"quantity"`
	AvgCost      float64  `json:"avg_cost"`
	CurrentPrice float64  `json:"current_price"`
	TotalCost    float64  `json:"total_cost"`
	MarketValue  float64  `json:"market_value"`
	Profit       *float64 `json:"profit,omitempty"`
	ProfitRate   *float64 `json:"profit_rate,omitempty"`
	Status       string   `json:"status"`
	TradeCount   int      `json:"trade_count"`
	Note         string   `json:"note,omitempty"`
	CreatedAt    string   `json:"created_at,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

// positionBrief 持仓简要信息（不含行情，仅股票名称、代码、成本价与股数）
type positionBrief struct {
	StockCode string  `json:"stock_code"`
	StockName string  `json:"stock_name"`
	Quantity  int     `json:"quantity"`
	AvgCost   float64 `json:"avg_cost"`
}

// handleListPositions 查询持仓列表。
//
// 不拉取实时行情（不读行情缓存、不读日K收盘价），只返回股票名称、代码、成本价与股数。
func (s *StockMCPServer) handleListPositions(_ context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	uid, errRes := s.resolveUserID(req)
	if errRes != nil {
		return errRes, nil
	}

	status := strings.TrimSpace(req.GetString("status", ""))
	if status == "" {
		status = string(model.PositionHolding)
	}
	page := req.GetInt("page", 1)
	if page <= 0 {
		page = 1
	}
	pageSize := req.GetInt("page_size", defaultPageSize)
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	positions, total, err := db.ListPositions(uid, status, page, pageSize)
	if err != nil {
		return gomcp.NewToolResultError("查询持仓失败: " + err.Error()), nil
	}

	// 批量关联股票名称（失败不阻断查询，用代码兜底）
	codes := make([]string, 0, len(positions))
	for i := range positions {
		codes = append(codes, positions[i].StockCode)
	}
	nameMap, err := db.GetStockNamesByCodes(codes)
	if err != nil {
		nameMap = map[string]string{}
	}

	items := make([]positionBrief, 0, len(positions))
	for i := range positions {
		p := positions[i]
		name := nameMap[p.StockCode]
		if name == "" {
			name = p.StockCode
		}
		items = append(items, positionBrief{
			StockCode: p.StockCode,
			StockName: name,
			Quantity:  p.Quantity,
			AvgCost:   round(p.AvgCost, 4),
		})
	}

	return jsonResult(map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"positions": items,
	})
}

// ============================================================================
//  查看策略
// ============================================================================

// strategyBrief 策略简要信息
type strategyBrief struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Using bool   `json:"using"`
}

// handleListStrategies 查询我的策略列表。
//
// using=true 时仅返回已订阅且订阅处于启用状态的策略，false 时返回全部可见策略。
func (s *StockMCPServer) handleListStrategies(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	user, errRes := s.resolveUser(req)
	if errRes != nil {
		return errRes, nil
	}

	usingOnly := req.GetBool("using", false)

	strategies, err := db.ListUserStrategies(ctx, user.ID, user.Role == roleAdmin, usingOnly)
	if err != nil {
		return gomcp.NewToolResultError("查询策略失败: " + err.Error()), nil
	}

	activeIDs, err := db.ListSubscribedStrategyIDs(user.ID, true)
	if err != nil {
		return gomcp.NewToolResultError("查询订阅状态失败: " + err.Error()), nil
	}
	usingSet := make(map[uint]bool, len(activeIDs))
	for _, id := range activeIDs {
		usingSet[id] = true
	}

	items := buildStrategyBriefs(strategies, usingSet)
	return jsonResult(map[string]any{
		"total":      len(items),
		"using_only": usingOnly,
		"strategies": items,
	})
}

// buildStrategyBriefs 将策略模型列表转换为 MCP 输出项。
//
// usingSet 为"已订阅且启用中"的策略ID集合，命中则 Using 为 true。
func buildStrategyBriefs(strategies []model.Strategy, usingSet map[uint]bool) []strategyBrief {
	items := make([]strategyBrief, 0, len(strategies))
	for i := range strategies {
		item := strategies[i]
		items = append(items, strategyBrief{
			ID:    item.ID,
			Name:  item.Name,
			Using: usingSet[item.ID],
		})
	}
	return items
}

// ============================================================================
//  按策略选股
// ============================================================================

// screenStock 选股结果项
type screenStock struct {
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// handleRunStrategyScreen 按指定策略立刻执行一次全市场选股。
//
// 流程：定位策略 → 解析信号配置 → 构建股票数据源 → 并发评估 → 输出通过列表。
func (s *StockMCPServer) handleRunStrategyScreen(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	user, errRes := s.resolveUser(req)
	if errRes != nil {
		return errRes, nil
	}
	if s.screen == nil || s.registry == nil {
		return gomcp.NewToolResultError("选股能力未启用：服务端未初始化选股服务"), nil
	}

	strategy, errRes := s.resolveStrategy(ctx, req, user)
	if errRes != nil {
		return errRes, nil
	}

	configs, err := parseSignalConfigs(strategy.Conditions)
	if err != nil {
		return gomcp.NewToolResultError("解析策略条件失败: " + err.Error()), nil
	}

	date, err := normalizeScreenDate(req.GetString("date", ""))
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}

	stocks, err := s.screen.BuildAll(screenConcurrency, date)
	if err != nil {
		return gomcp.NewToolResultError("获取股票数据失败: " + err.Error()), nil
	}

	result := map[string]any{
		"strategy_id":   strategy.ID,
		"strategy_name": strategy.Name,
		"trade_date":    resolveScreenTradeDate(date),
		"scanned":       len(stocks),
		"stocks":        []screenStock{},
	}
	if len(stocks) > 0 {
		result["stocks"] = toScreenStocks(s.registry.Engine().Execute(stocks, configs, screenConcurrency))
	}
	return jsonResult(result)
}

// resolveStrategy 按 strategy_id 定位用户可见的策略。
func (s *StockMCPServer) resolveStrategy(
	ctx context.Context, req gomcp.CallToolRequest, user *model.User,
) (*model.Strategy, *gomcp.CallToolResult) {
	id := req.GetInt("strategy_id", 0)
	if id <= 0 {
		return nil, gomcp.NewToolResultError("strategy_id 必须为大于 0 的整数（可用 list_strategies 查看我的策略ID）")
	}

	strategy, err := db.GetVisibleStrategyByID(ctx, user.ID, user.Role == roleAdmin, uint(id))
	if errors.Is(err, db.ErrRecordNotFound) {
		return nil, gomcp.NewToolResultError(fmt.Sprintf(
			"未找到策略 %d（不存在或无权访问，可用 list_strategies 查看我的策略ID）", id))
	}
	if err != nil {
		return nil, gomcp.NewToolResultError("查询策略失败: " + err.Error())
	}
	return strategy, nil
}

// parseSignalConfigs 将策略 conditions JSON 解析为引擎所需的信号配置列表。
func parseSignalConfigs(conditionsJSON string) ([]*indicator.SignalConfig, error) {
	if conditionsJSON == "" {
		return nil, errors.New("策略条件为空")
	}
	var configs []*indicator.SignalConfig
	if err := json.Unmarshal([]byte(conditionsJSON), &configs); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	if len(configs) == 0 {
		return nil, errors.New("策略未配置任何信号")
	}
	return configs, nil
}

// toScreenStocks 提取评估结果为 Passed 的股票，按代码升序排列。
func toScreenStocks(evaluated []*indicator.EvaluatedStock) []screenStock {
	items := make([]screenStock, 0, len(evaluated))
	for _, stock := range evaluated {
		if stock.Result != indicator.ResultPassed {
			continue
		}
		items = append(items, screenStock{
			Code:  stock.Code,
			Name:  stock.Name,
			Price: round(stock.Price, 2),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Code < items[j].Code })
	return items
}

// normalizeScreenDate 校验并归一化选股日期，空值表示取最近交易日。
func normalizeScreenDate(raw string) (string, error) {
	date := strings.TrimSpace(raw)
	if date == "" {
		return "", nil
	}
	if len(date) == 8 && !strings.Contains(date, "-") {
		date = date[0:4] + "-" + date[4:6] + "-" + date[6:8]
	}
	if _, err := time.Parse(dateLayout, date); err != nil {
		return "", fmt.Errorf("选股日期格式错误，应为 YYYY-MM-DD 或 YYYYMMDD")
	}
	return date, nil
}

// resolveScreenTradeDate 解析实际使用的交易日 YYYYMMDD（仅用于回显，失败返回 0）。
func resolveScreenTradeDate(date string) int {
	if parsed, err := utils.ParseDateToTradeDate(date); err == nil {
		if nearest, err := db.FindNearestDailyKlineDate(parsed); err == nil {
			return nearest
		}
		return 0
	}
	latest, err := db.GetLatestDailyKlineDate()
	if err != nil {
		return 0
	}
	return latest
}

// ============================================================================
//  建仓 / 加仓 / 减仓
// ============================================================================

func (s *StockMCPServer) handleOpenPosition(_ context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	uid, errRes := s.resolveUserID(req)
	if errRes != nil {
		return errRes, nil
	}

	svcReq, errRes := s.buildOpenReq(req)
	if errRes != nil {
		return errRes, nil
	}

	detail, err := s.portfolio.OpenPosition(svcReq, uid)
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	return positionResult("建仓成功", detail)
}

func (s *StockMCPServer) handleAddPosition(_ context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	uid, errRes := s.resolveUserID(req)
	if errRes != nil {
		return errRes, nil
	}

	id, errRes := s.resolvePositionID(req, uid)
	if errRes != nil {
		return errRes, nil
	}

	svcReq, errRes := s.buildTradeReq(req)
	if errRes != nil {
		return errRes, nil
	}

	detail, err := s.portfolio.BuyMore(id, svcReq, uid)
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	return positionResult("加仓成功", detail)
}

func (s *StockMCPServer) handleReducePosition(_ context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	uid, errRes := s.resolveUserID(req)
	if errRes != nil {
		return errRes, nil
	}

	id, errRes := s.resolvePositionID(req, uid)
	if errRes != nil {
		return errRes, nil
	}

	svcReq, errRes := s.buildTradeReq(req)
	if errRes != nil {
		return errRes, nil
	}

	detail, err := s.portfolio.SellPartial(id, svcReq, uid)
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	return positionResult("减仓成功", detail)
}

// ============================================================================
//  清仓 / 删除持仓
// ============================================================================

func (s *StockMCPServer) handleClosePosition(_ context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	uid, errRes := s.resolveUserID(req)
	if errRes != nil {
		return errRes, nil
	}

	id, errRes := s.resolvePositionID(req, uid)
	if errRes != nil {
		return errRes, nil
	}

	price := req.GetFloat("price", 0)
	if price <= 0 {
		return gomcp.NewToolResultError("price 必须大于 0"), nil
	}
	tradeDate, err := parseTradeDate(req)
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}

	detail, err := s.portfolio.ClosePosition(id, price, tradeDate, req.GetString("note", ""), uid)
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	return positionResult("清仓成功", detail)
}

func (s *StockMCPServer) handleDeletePosition(_ context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	uid, errRes := s.resolveUserID(req)
	if errRes != nil {
		return errRes, nil
	}

	id, errRes := s.resolvePositionID(req, uid)
	if errRes != nil {
		return errRes, nil
	}

	if err := s.portfolio.DeletePosition(id, uid); err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"message": "删除成功", "position_id": id})
}

// ============================================================================
//  参数解析与校验
// ============================================================================

// resolveUser 解析调用用户：优先取入参 wework_id，缺省时取请求头 X-Wework-Id，
// 再按 users.wework_id 关联出系统用户。
//
// 两者都为空或关联不到启用用户时返回错误结果。
func (s *StockMCPServer) resolveUser(req gomcp.CallToolRequest) (*model.User, *gomcp.CallToolResult) {
	weworkID := strings.TrimSpace(req.GetString("wework_id", ""))
	if weworkID == "" {
		weworkID = strings.TrimSpace(req.Header.Get(HeaderWeworkID))
	}
	if weworkID == "" {
		return nil, gomcp.NewToolResultError(fmt.Sprintf(
			"缺少用户标识：请在入参传 wework_id，或在请求头传 %s（企业微信用户ID）", HeaderWeworkID))
	}

	user, err := db.GetUserByWeworkID(weworkID)
	if errors.Is(err, db.ErrRecordNotFound) {
		return nil, gomcp.NewToolResultError(
			fmt.Sprintf("企微ID %s 未关联到有效用户，请在 users.wework_id 中配置", weworkID))
	}
	if err != nil {
		return nil, gomcp.NewToolResultError("查询用户失败: " + err.Error())
	}
	return user, nil
}

// resolveUserID 解析调用用户ID（resolveUser 的便捷包装，只需 uid 时使用）。
func (s *StockMCPServer) resolveUserID(req gomcp.CallToolRequest) (uint, *gomcp.CallToolResult) {
	user, errRes := s.resolveUser(req)
	if errRes != nil {
		return 0, errRes
	}
	return user.ID, nil
}

// resolvePositionID 解析持仓ID：position_id 优先，其次按 stock_code 查持仓中的记录。
func (s *StockMCPServer) resolvePositionID(req gomcp.CallToolRequest, uid uint) (uint, *gomcp.CallToolResult) {
	if id := req.GetInt("position_id", 0); id > 0 {
		return uint(id), nil
	}

	code := strings.TrimSpace(req.GetString("stock_code", ""))
	if code == "" {
		return 0, gomcp.NewToolResultError("请提供 position_id 或 stock_code")
	}

	position, err := db.GetPositionByStockCodeAndUID(code, uid)
	if err != nil || position == nil {
		return 0, gomcp.NewToolResultError(fmt.Sprintf("未找到 %s 的持仓中记录（可用 list_positions 查看持仓ID）", code))
	}
	return position.ID, nil
}

func (s *StockMCPServer) buildOpenReq(req gomcp.CallToolRequest) (*service.OpenPositionReq, *gomcp.CallToolResult) {
	code := strings.TrimSpace(req.GetString("stock_code", ""))
	if err := validateStockCode(code); err != nil {
		return nil, gomcp.NewToolResultError(err.Error())
	}
	quantity := req.GetInt("quantity", 0)
	if quantity <= 0 {
		return nil, gomcp.NewToolResultError("quantity 必须大于 0")
	}
	price := req.GetFloat("price", 0)
	if price <= 0 {
		return nil, gomcp.NewToolResultError("price 必须大于 0")
	}
	tradeDate, err := parseTradeDate(req)
	if err != nil {
		return nil, gomcp.NewToolResultError(err.Error())
	}

	return &service.OpenPositionReq{
		StockCode: code,
		Quantity:  quantity,
		Price:     price,
		TradeDate: tradeDate,
		Note:      req.GetString("note", ""),
	}, nil
}

func (s *StockMCPServer) buildTradeReq(req gomcp.CallToolRequest) (*service.TradeReq, *gomcp.CallToolResult) {
	quantity := req.GetInt("quantity", 0)
	if quantity <= 0 {
		return nil, gomcp.NewToolResultError("quantity 必须大于 0")
	}
	price := req.GetFloat("price", 0)
	if price <= 0 {
		return nil, gomcp.NewToolResultError("price 必须大于 0")
	}
	tradeDate, err := parseTradeDate(req)
	if err != nil {
		return nil, gomcp.NewToolResultError(err.Error())
	}

	return &service.TradeReq{
		Quantity:  quantity,
		Price:     price,
		TradeDate: tradeDate,
		Note:      req.GetString("note", ""),
	}, nil
}

// parseTradeDate 解析交易日期，支持 YYYY-MM-DD 与 YYYYMMDD，缺省为今天。
func parseTradeDate(req gomcp.CallToolRequest) (string, error) {
	s := strings.TrimSpace(req.GetString("trade_date", ""))
	if s == "" {
		return time.Now().Format(dateLayout), nil
	}
	if len(s) == 8 && !strings.Contains(s, "-") {
		s = s[0:4] + "-" + s[4:6] + "-" + s[6:8]
	}
	if _, err := time.Parse(dateLayout, s); err != nil {
		return "", fmt.Errorf("交易日期格式错误，应为 YYYY-MM-DD")
	}
	return s, nil
}

func validateStockCode(code string) error {
	if len(code) != 6 {
		return fmt.Errorf("股票代码应为6位数字，如 000001")
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return fmt.Errorf("股票代码应为6位数字，如 000001")
		}
	}
	return nil
}

// ============================================================================
//  结果组装
// ============================================================================

func positionResult(action string, d *model.PositionDetail) (*gomcp.CallToolResult, error) {
	return jsonResult(map[string]any{
		"message":  action,
		"position": toPositionItem(d),
	})
}

func toPositionItem(d *model.PositionDetail) positionItem {
	item := positionItem{
		ID:           d.ID,
		StockCode:    d.StockCode,
		StockName:    d.StockName,
		Quantity:     d.Quantity,
		AvgCost:      round(d.AvgCost, 4),
		CurrentPrice: round(d.CurrentPrice, 4),
		TotalCost:    round(d.TotalCost, 2),
		Status:       d.Status,
		TradeCount:   d.TradeCount,
		Note:         d.Note,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
	if d.CurrentPrice > 0 && d.Quantity > 0 {
		item.MarketValue = round(d.CurrentPrice*float64(d.Quantity), 2)
		profit := round((d.CurrentPrice-d.AvgCost)*float64(d.Quantity), 2)
		item.Profit = &profit
		if d.AvgCost > 0 {
			rate := round((d.CurrentPrice-d.AvgCost)/d.AvgCost*100, 2)
			item.ProfitRate = &rate
		}
	}
	return item
}

func round(v float64, digits int) float64 {
	p := math.Pow10(digits)
	return math.Round(v*p) / p
}

func jsonResult(v any) (*gomcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return gomcp.NewToolResultError("序列化结果失败: " + err.Error()), nil
	}
	return gomcp.NewToolResultText(string(data)), nil
}
