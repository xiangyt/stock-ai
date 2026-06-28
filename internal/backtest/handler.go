package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler 回测 HTTP 处理器。
type Handler struct {
	factory   *EngineFactory
	dao       *DAO
	emAdapter *eastmoney.Adapter // 东财适配器（自选股 API，延迟注入）
}

// NewHandler 创建回测 Handler。
// factory 每次 Initiate 调用时创建独立的 Engine，保证并发安全。
func NewHandler(factory *EngineFactory) *Handler {
	return &Handler{factory: factory, dao: NewDAO()}
}

// SetEMAdapter 设置东财适配器（由 main 注入）。
func (h *Handler) SetEMAdapter(a *eastmoney.Adapter) {
	h.emAdapter = a
}

// ============================================================================
//  P0 API
// ============================================================================

// Initiate POST /api/v1/strategies/:id/backtest
func (h *Handler) Initiate(c *gin.Context) {
	var req struct {
		StockPool             []string       `json:"stock_pool"`
		StartDate             string         `json:"start_date" binding:"required"`
		EndDate               string         `json:"end_date" binding:"required"`
		InitialCapital        float64        `json:"initial_capital" binding:"required"`
		EntryConfigs          []SignalConfig `json:"entry_configs"`
		ExitRulesOverride     *struct {
			Rules       []ExitRule `json:"rules"`
			SlippagePct float64    `json:"slippage_pct"`
		} `json:"exit_rules_override"`
		PositionRulesOverride *PositionRules `json:"position_rules_override"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}
	strategyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid strategy id"})
		return
	}

	// 空 stock_pool 默认使用 start_date 当天有日K数据的所有股票
	if len(req.StockPool) == 0 {
		req.StockPool, err = loadDefaultStockPool(req.StartDate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
			return
		}
	}

	// 空 entry_configs 默认从策略的 conditions 字段解析
	if len(req.EntryConfigs) == 0 {
		req.EntryConfigs, err = loadStrategyEntryConfigs(strategyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
			return
		}
	}

	runRequest := RunRequest{
		StrategyID:     strategyID,
		StockPool:      req.StockPool,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		InitialCapital: req.InitialCapital,
		EntryConfigs:   req.EntryConfigs,
	}
	// 前端可能传 exit_rules_override / position_rules_override
	if req.ExitRulesOverride != nil {
		runRequest.ExitConfigs = req.ExitRulesOverride.Rules
	}
	if req.PositionRulesOverride != nil {
		runRequest.PositionRules = *req.PositionRulesOverride
	}
	if err := validateRunRequest(runRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}
	engine, err := h.factory.NewEngine(runRequest.ExitConfigs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	runID, err := engine.Initiate(c.Request.Context(), runRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"run_id": runID, "status": "pending"}})
}

// GetRun GET /api/v1/backtest/runs/:id
func (h *Handler) GetRun(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid run id"})
		return
	}
	run, err := h.dao.GetRun(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": run})
}

// GetRunStatus GET /api/v1/backtest/runs/:id/status
func (h *Handler) GetRunStatus(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid run id"})
		return
	}
	// 记录前端活跃时间（用于断连检测）
	CheckInRun(runID)

	run, err := h.dao.GetRun(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"status": run.Status, "progress_pct": run.ProgressPct}})
}

// Stop POST /api/v1/backtest/runs/:id/stop
func (h *Handler) Stop(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid run id"})
		return
	}
	CancelRun(runID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"status": "stopping"}})
}

// ============================================================================
//  BatchAddToFavorites POST /api/v1/backtest/batch/favorites
//  一键将选股结果加入东财自选分组
// ============================================================================

type batchAddFavoritesReq struct {
	StrategyID uint64   `json:"strategy_id" binding:"required"` // 策略 ID
	Date       string   `json:"date" binding:"required"`        // 日期 YYYYMMDD
	StockCodes []string `json:"stock_codes" binding:"required"` // 股票代码列表
}

func (h *Handler) BatchAddToFavorites(c *gin.Context) {
	if h.emAdapter == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "东财适配器未初始化"})
		return
	}

	var req batchAddFavoritesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	// 从 session 获取当前用户 ID
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": "未登录"})
		return
	}

	// 从 user_software_configs 获取东财 Cookie
	cookie, err := getUserCookie(uint(userID), eastmoney.AdapterName)
	if err != nil || cookie == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "未配置东财 Cookie，请在个人主页中配置后再试"})
		return
	}

	// 组合名: 策略{id}选{YYYYMMDD}（如 "策略13选20260628"）
	groupName := fmt.Sprintf("策略%d选%s", req.StrategyID, req.Date)

	ctx := c.Request.Context()
	adapter := h.emAdapter

	// 1. 查找现有分组，同名直接复用
	gid, err := findOrCreateGroup(ctx, adapter, cookie, groupName)
	if err != nil {
		log.Printf("[Favor] findOrCreateGroup failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": fmt.Sprintf("创建/查找分组失败: %v", err)})
		return
	}

	// 2. 逐只添加股票，失败跳过
	var failed []string
	for _, code := range req.StockCodes {
		if err := adapter.AddToGroup(ctx, cookie, gid, code); err != nil {
			log.Printf("[Favor] 添加 %s 到分组 %s 失败: %v", code, gid, err)
			failed = append(failed, code)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"gid":    gid,
			"gname":  groupName,
			"total":  len(req.StockCodes),
			"failed": failed,
		},
	})
}

// getUserCookie 从 user_software_configs 获取指定用户、指定软件的 Cookie。
func getUserCookie(userID uint, softwareName string) (string, error) {
	var cfg model.UserSoftwareConfig
	if err := db.GetDB().Where("user_id = ? AND software_name = ? AND enabled = ?",
		userID, softwareName, true).First(&cfg).Error; err != nil {
		return "", err
	}
	return cfg.Cookie, nil
}

// findOrCreateGroup 查找同名分组，不存在则新建。返回 gid。
func findOrCreateGroup(ctx context.Context, adapter *eastmoney.Adapter, cookie, groupName string) (string, error) {
	groups, err := adapter.ListGroups(ctx, cookie)
	if err != nil {
		return "", fmt.Errorf("获取分组列表失败: %w", err)
	}

	trimmedName := strings.TrimSpace(groupName)
	for _, g := range groups {
		if strings.TrimSpace(g.GName) == trimmedName {
			return g.GID, nil
		}
	}

	result, err := adapter.CreateGroup(ctx, cookie, trimmedName)
	if err != nil {
		return "", fmt.Errorf("创建分组失败: %w", err)
	}
	return result.GID, nil
}

// GetTrades GET /api/v1/backtest/runs/:id/trades
func (h *Handler) GetTrades(c *gin.Context) {
	runID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	trades, total, err := h.dao.GetTradesByRun(runID, (page-1)*pageSize, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"total": total, "items": trades}})
}

// snapshotItem 快照响应 DTO（格式化 snap_date）。
type snapshotItem struct {
	ID              uint64   `json:"id"`
	RunID           uint64   `json:"run_id"`
	SnapDate        string   `json:"snap_date"`
	TotalEquity     float64  `json:"total_equity"`
	Cash            float64  `json:"cash"`
	MarketValue     float64  `json:"market_value"`
	PositionCount   int      `json:"position_count"`
	DailyReturn     *float64 `json:"daily_return"`
	CumulativeReturn *float64 `json:"cumulative_return"`
}

// GetSnapshots GET /api/v1/backtest/runs/:id/snapshots?after_id=0
func (h *Handler) GetSnapshots(c *gin.Context) {
	runID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	afterID, _ := strconv.ParseUint(c.DefaultQuery("after_id", "0"), 10, 64)
	snapshots, err := h.dao.GetSnapshotsByRun(runID, afterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	items := make([]snapshotItem, len(snapshots))
	for i, s := range snapshots {
		items[i] = snapshotItem{
			ID:              s.ID,
			RunID:           s.RunID,
			SnapDate:        s.SnapDate[:10],
			TotalEquity:     s.TotalEquity,
			Cash:            s.Cash,
			MarketValue:     s.MarketValue,
			PositionCount:   s.PositionCount,
			DailyReturn:     s.DailyReturn,
			CumulativeReturn: s.CumulativeReturn,
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"snapshots": items}})
}

// GetRuns GET /api/v1/strategies/:id/backtest/runs
func (h *Handler) GetRuns(c *gin.Context) {
	strategyID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	runs, err := h.dao.GetRunsByStrategy(strategyID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": runs})
}

// DeleteRun DELETE /api/v1/backtest/runs/:id
func (h *Handler) DeleteRun(c *gin.Context) {
	runID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.dao.DeleteRun(runID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"deleted": true}})
}

// ============================================================================
//  入参校验
// ============================================================================

// conditionItem 策略 conditions JSON 中的单条记录结构。
type conditionItem struct {
	SignalID string         `json:"signal_id"`
	Operator string         `json:"operator"`
	Params   map[string]any `json:"params"`
}

// loadStrategyEntryConfigs 从 DB 加载策略的 conditions JSON，解析为 SignalConfig 列表。
func loadStrategyEntryConfigs(strategyID uint64) ([]SignalConfig, error) {
	var st model.Strategy
	if err := db.GetDB().First(&st, strategyID).Error; err != nil {
		return nil, fmt.Errorf("strategy %d not found: %w", strategyID, err)
	}
	if st.Conditions == "" {
		return nil, fmt.Errorf("strategy %d has no conditions", strategyID)
	}
	var items []conditionItem
	if err := json.Unmarshal([]byte(st.Conditions), &items); err != nil {
		return nil, fmt.Errorf("parse strategy conditions: %w", err)
	}
	cfgs := make([]SignalConfig, 0, len(items))
	for _, item := range items {
		if item.SignalID == "" {
			continue
		}
		cfgs = append(cfgs, SignalConfig{
			SignalID: item.SignalID,
			Operator: item.Operator,
			Params:   item.Params,
		})
	}
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("strategy %d has no valid signals", strategyID)
	}
	return cfgs, nil
}

// loadDefaultStockPool 加载指定日期有日K数据的所有股票代码，作为默认股票池。
func loadDefaultStockPool(dateStr string) ([]string, error) {
	date, err := utilsParseDate(dateStr)
	if err != nil {
		return nil, fmt.Errorf("parse start_date for default stock pool: %w", err)
	}
	tradeDate, err := db.FindNearestDailyKlineDate(date)
	if err != nil {
		return nil, fmt.Errorf("no trading data found near %s: %w", dateStr, err)
	}
	stocks, err := db.LoadStockCodesByTradeDate(tradeDate)
	if err != nil {
		return nil, fmt.Errorf("load stock codes: %w", err)
	}
	codes := make([]string, len(stocks))
	for i, s := range stocks {
		codes[i] = s.Code
	}
	return codes, nil
}

// utilsParseDate 将 YYYYMMDD 整数转为 "2006-01-02" 格式的 time.Time，辅助函数。
func utilsParseDate(dateStr string) (int, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, err
	}
	return t.Year()*10000 + int(t.Month())*100 + t.Day(), nil
}

// validateRunRequest 校验回测请求参数。
func validateRunRequest(req RunRequest) error {
	if len(req.StockPool) == 0 {
		return fmt.Errorf("stock_pool is empty")
	}
	if req.InitialCapital <= 0 {
		return fmt.Errorf("initial_capital must be positive, got %.2f", req.InitialCapital)
	}

	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start_date %q: %w", req.StartDate, err)
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return fmt.Errorf("invalid end_date %q: %w", req.EndDate, err)
	}
	if end.Before(start) {
		return fmt.Errorf("end_date %s is before start_date %s", req.EndDate, req.StartDate)
	}
	return nil
}
