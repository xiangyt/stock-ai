package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"stock-ai/internal/service"

	"github.com/xiangyt/mcpkit"
)

// StockMCPServer MCP 股票服务
type StockMCPServer struct {
	server  *mcpkit.Server
	service *service.StockService
}

// NewStockMCPServer 创建 MCP 服务器
func NewStockMCPServer(name, version string) *StockMCPServer {
	s := &StockMCPServer{
		server:  mcpkit.NewServer(name, version),
		service: service.NewStockService(),
	}
	s.registerTools()
	return s
}

// registerTools 注册 MCP Tools
func (s *StockMCPServer) registerTools() {
	// 1. filter_stocks - 条件选股
	s.server.RegisterTool(mcpkit.Tool{
		Name:        "filter_stocks",
		Description: "根据技术指标筛选股票，支持均线、MACD、KDJ、RSI等条件",
		InputSchema: mcpkit.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"conditions": map[string]interface{}{
					"type":        "array",
					"description": "筛选条件列表",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"field": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"均线", "资金流入", "MACD", "KDJ", "RSI", "RSL"},
								"description": "技术指标字段",
							},
							"operator": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"大于", "小于", "等于", "大于等于", "小于等于"},
								"description": "比较操作符",
							},
							"value": map[string]interface{}{
								"type":        "number",
								"description": "阈值数值",
							},
						},
						"required": []string{"field", "operator", "value"},
					},
				},
				"market": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"A股", "港股", "美股"},
					"description": "市场类型",
					"default":     "A股",
				},
			},
			Required: []string{"conditions"},
		},
	}, s.handleFilterStocks)

	// 2. ai_stock_query - AI选股
	s.server.RegisterTool(mcpkit.Tool{
		Name:        "ai_stock_query",
		Description: "使用自然语言查询股票，例如：找出MACD金叉的股票、筛选低价股等",
		InputSchema: mcpkit.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "自然语言查询语句",
				},
			},
			Required: []string{"query"},
		},
	}, s.handleAIQuery)

	// 3. get_hot_topics - 热门题材
	s.server.RegisterTool(mcpkit.Tool{
		Name:        "get_hot_topics",
		Description: "获取当前市场热门题材和概念股",
		InputSchema: mcpkit.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleGetHotTopics)

	// 4. get_stock_detail - 股票详情
	s.server.RegisterTool(mcpkit.Tool{
		Name:        "get_stock_detail",
		Description: "获取指定股票的详细信息",
		InputSchema: mcpkit.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"code": map[string]interface{}{
					"type":        "string",
					"description": "股票代码，如：000001、600519",
				},
			},
			Required: []string{"code"},
		},
	}, s.handleGetStockDetail)

	// 5. get_stock_prices - 价格历史
	s.server.RegisterTool(mcpkit.Tool{
		Name:        "get_stock_prices",
		Description: "获取股票的历史价格数据",
		InputSchema: mcpkit.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"code": map[string]interface{}{
					"type":        "string",
					"description": "股票代码",
				},
				"days": map[string]interface{}{
					"type":        "number",
					"description": "获取天数，默认30天，最大365天",
					"default":     30,
				},
			},
			Required: []string{"code"},
		},
	}, s.handleGetStockPrices)
}

// handleFilterStocks 处理条件选股
func (s *StockMCPServer) handleFilterStocks(ctx context.Context, args map[string]interface{}) (*mcpkit.ToolResult, error) {
	// 解析参数
	conditionsData, ok := args["conditions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid conditions parameter")
	}

	var conditions []service.FilterCondition
	for _, c := range conditionsData {
		condMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		
		field, _ := condMap["field"].(string)
		operator, _ := condMap["operator"].(string)
		value := condMap["value"]
		
		conditions = append(conditions, service.FilterCondition{
			Field:    field,
			Operator: operator,
			Value:    value,
		})
	}

	market := "A股"
	if m, ok := args["market"].(string); ok {
		market = m
	}

	// 调用服务
	req := service.FilterRequest{
		Conditions: conditions,
		Market:     market,
	}

	resp, err := s.service.FilterStocks(req)
	if err != nil {
		return nil, err
	}

	// 格式化结果
	result := formatFilterResult(resp)
	return &mcpkit.ToolResult{
		Content: []mcpkit.Content{
			{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// handleAIQuery 处理AI选股
func (s *StockMCPServer) handleAIQuery(ctx context.Context, args map[string]interface{}) (*mcpkit.ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required")
	}

	resp, err := s.service.AIQuery(query)
	if err != nil {
		return nil, err
	}

	result := formatFilterResult(resp)
	return &mcpkit.ToolResult{
		Content: []mcpkit.Content{
			{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// handleGetHotTopics 处理热门题材
func (s *StockMCPServer) handleGetHotTopics(ctx context.Context, args map[string]interface{}) (*mcpkit.ToolResult, error) {
	resp, err := s.service.GetHotTopics()
	if err != nil {
		return nil, err
	}

	var result string
	result = "## 热门题材\n\n"
	for _, topic := range resp.Topics {
		result += fmt.Sprintf("### %s [%s]\n", topic.Name, topic.Tag)
		result += fmt.Sprintf("- 描述: %s\n", topic.Description)
		result += fmt.Sprintf("- 相关股票: %s\n\n", joinStrings(topic.Stocks, ", "))
	}

	return &mcpkit.ToolResult{
		Content: []mcpkit.Content{
			{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// handleGetStockDetail 处理股票详情
func (s *StockMCPServer) handleGetStockDetail(ctx context.Context, args map[string]interface{}) (*mcpkit.ToolResult, error) {
	code, ok := args["code"].(string)
	if !ok || code == "" {
		return nil, fmt.Errorf("code is required")
	}

	resp, err := s.service.GetStockDetail(code)
	if err != nil {
		return nil, err
	}

	stock := resp.Stock
	result := fmt.Sprintf("## %s (%s)\n\n", stock.Name, stock.Code)
	result += fmt.Sprintf("- 当前价格: ¥%.2f\n", stock.Price)
	result += fmt.Sprintf("- 涨跌幅: %.2f%%\n", stock.ChangePercent)
	result += fmt.Sprintf("- 成交量: %d\n", stock.Volume)
	result += fmt.Sprintf("- 市值: %.2f亿\n", stock.MarketCap)
	if stock.PERatio != nil {
		result += fmt.Sprintf("- PE: %.2f\n", *stock.PERatio)
	}
	if stock.PBRatio != nil {
		result += fmt.Sprintf("- PB: %.2f\n", *stock.PBRatio)
	}

	return &mcpkit.ToolResult{
		Content: []mcpkit.Content{
			{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// handleGetStockPrices 处理价格历史
func (s *StockMCPServer) handleGetStockPrices(ctx context.Context, args map[string]interface{}) (*mcpkit.ToolResult, error) {
	code, ok := args["code"].(string)
	if !ok || code == "" {
		return nil, fmt.Errorf("code is required")
	}

	days := 30
	if d, ok := args["days"].(float64); ok {
		days = int(d)
	}

	resp, err := s.service.GetStockPrices(code, days)
	if err != nil {
		return nil, err
	}

	var result string
	result = fmt.Sprintf("## %s 价格历史 (最近%d天)\n\n", code, days)
	result += "| 日期 | 开盘 | 收盘 | 最高 | 最低 | 涨跌幅 |\n"
	result += "|------|------|------|------|------|--------|\n"

	for _, p := range resp.Prices {
		result += fmt.Sprintf("| %s | %.2f | %.2f | %.2f | %.2f | %.2f%% |\n",
			p.Date, p.Open, p.Close, p.High, p.Low, p.ChangePct)
	}

	return &mcpkit.ToolResult{
		Content: []mcpkit.Content{
			{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// Start 启动 MCP 服务器
func (s *StockMCPServer) Start() error {
	return s.server.Start()
}

// Stop 停止 MCP 服务器
func (s *StockMCPServer) Stop() error {
	return s.server.Stop()
}

// 辅助函数
func formatFilterResult(resp *service.FilterResponse) string {
	if !resp.Success {
		return "选股失败: " + resp.Message
	}

	result := fmt.Sprintf("## 选股结果\n\n")
	result += fmt.Sprintf("- 符合条件: %d 只股票\n", resp.Total)
	result += fmt.Sprintf("- 耗时: %d ms\n\n", resp.ExecutionTimeMs)

	if len(resp.Stocks) == 0 {
		result += "未找到符合条件的股票\n"
		return result
	}

	result += "| 代码 | 名称 | 价格 | 涨跌幅 | 市值(亿) |\n"
	result += "|------|------|------|--------|----------|\n"

	for _, s := range resp.Stocks {
		changeEmoji := ""
		if s.ChangePercent > 0 {
			changeEmoji = "📈"
		} else if s.ChangePercent < 0 {
			changeEmoji = "📉"
		}
		result += fmt.Sprintf("| %s | %s | ¥%.2f | %s %.2f%% | %.2f |\n",
			s.Code, s.Name, s.Price, changeEmoji, s.ChangePercent, s.MarketCap)
	}

	return result
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
