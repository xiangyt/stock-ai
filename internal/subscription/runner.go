package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/indicator"
	stocksource "stock-ai/internal/indicator/stocksource"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ============================================================================
//  SubscriptionRunner 订阅执行器
// ============================================================================

// RunResult 单次执行结果
type RunResult struct {
	LogID        uint               `json:"log_id"`
	Status       model.LogStatus    `json:"status"`
	TotalScanned int                `json:"total_scanned"`
	MatchCount   int                `json:"match_count"`
	DurationMs   int                `json:"duration_ms"`
	MatchStocks  []model.MatchStock `json:"match_stocks,omitempty"`
	PushStatus   map[uint]string    `json:"push_status,omitempty"`
	ErrorMsg     string             `json:"error_msg,omitempty"`
}

// SubscriptionRunner 订阅执行器
type SubscriptionRunner struct {
	cache    QuoteCache
	engine   *indicator.Engine
	notifier Notifier
}

// NewSubscriptionRunner 创建订阅执行器
func NewSubscriptionRunner(cache QuoteCache, engine *indicator.Engine, notifier Notifier) *SubscriptionRunner {
	return &SubscriptionRunner{
		cache:    cache,
		engine:   engine,
		notifier: notifier,
	}
}

// Run 执行单次订阅
func (r *SubscriptionRunner) Run(ctx context.Context, sub *model.Subscription) (*RunResult, error) {
	startTime := time.Now()
	result := &RunResult{
		PushStatus: make(map[uint]string),
	}

	// 1. 加载策略（解析 Conditions JSON → []indicator.SignalConfig）
	strategy, err := db.GetStrategyByID(sub.StrategyID)
	if err != nil {
		return nil, fmt.Errorf("加载策略失败: %w", err)
	}

	configs, err := parseSignalConfigs(strategy.Conditions)
	if err != nil {
		return nil, fmt.Errorf("解析策略条件失败: %w", err)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("策略条件为空")
	}

	// 2. 按 scope 获取股票代码列表
	codes := r.resolveStockCodes(sub)
	if len(codes) == 0 {
		result.Status = model.LogStatusFailed
		result.ErrorMsg = "没有可扫描的股票"
		return result, nil
	}

	result.TotalScanned = len(codes)

	// 3. 构建指标注册表（需要解析 indicator ID → Indicator）
	// 获取交易日期
	tradeDate := time.Now().Format("20060102")
	tradeDateInt := 0
	fmt.Sscanf(tradeDate, "%d", &tradeDateInt)

	// 4. 构建 []indicator.StockSource
	stocks := r.buildStockSources(codes, tradeDateInt)

	// 5. 执行选股引擎
	var allPassed []*indicator.EvaluatedStock

	if sub.Scope == model.ScopeAll {
		// 全量扫描：每 500 只为一批
		const batchSize = 500
		for i := 0; i < len(stocks); i += batchSize {
			end := i + batchSize
			if end > len(stocks) {
				end = len(stocks)
			}
			batch := stocks[i:end]

			// 单批超时保护由 runner 外层的 ctx 控制
			evaluated := r.engine.Execute(batch, configs, 50)

			for _, ev := range evaluated {
				if ev.Result == indicator.ResultPassed {
					allPassed = append(allPassed, ev)
				}
			}
		}
	} else {
		// 持仓/自选：直接执行
		evaluated := r.engine.Execute(stocks, configs, 10)
		for _, ev := range evaluated {
			if ev.Result == indicator.ResultPassed {
				allPassed = append(allPassed, ev)
			}
		}
	}

	// 6. 过滤结果，组装 MatchStock
	result.MatchStocks = make([]model.MatchStock, 0, len(allPassed))
	for _, ev := range allPassed {
		result.MatchStocks = append(result.MatchStocks, model.MatchStock{
			Code:  ev.Code,
			Name:  ev.Name,
			Price: ev.Price,
		})
	}
	result.MatchCount = len(result.MatchStocks)

	// 7. 加载关联机器人
	bots, err := db.GetSubscriptionBots(sub.ID)
	if err != nil {
		log.Printf("[SubscriptionRunner] 获取订阅 %d 关联机器人失败: %v", sub.ID, err)
	}

	// 8. 渲染通知消息并发送
	message := r.renderMessage(sub, strategy.Name, result)

	for _, bot := range bots {
		sendErr := r.notifier.Send(ctx, &bot, message)
		if sendErr != nil {
			result.PushStatus[bot.ID] = "失败: " + sendErr.Error()
		} else {
			result.PushStatus[bot.ID] = "成功"
		}
	}

	// 9. 计算耗时
	result.DurationMs = int(time.Since(startTime).Milliseconds())

	// 10. 确定状态
	if result.MatchCount > 0 && len(result.PushStatus) > 0 {
		allSuccess := true
		for _, status := range result.PushStatus {
			if status != "成功" {
				allSuccess = false
				break
			}
		}
		if allSuccess {
			result.Status = model.LogStatusSuccess
		} else {
			result.Status = model.LogStatusPartial
		}
	} else if result.MatchCount == 0 {
		result.Status = model.LogStatusSuccess
	} else {
		result.Status = model.LogStatusFailed
	}

	// 11. 写入 SubscriptionLog
	result.LogID = r.writeLog(sub, result)

	// 12. 更新 LastRunAt
	if err := db.UpdateLastRunTime(sub.ID); err != nil {
		log.Printf("[SubscriptionRunner] 更新订阅 %d 最后运行时间失败: %v", sub.ID, err)
	}

	return result, nil
}

// resolveStockCodes 按 scope 获取股票代码列表
func (r *SubscriptionRunner) resolveStockCodes(sub *model.Subscription) []string {
	switch sub.Scope {
	case model.ScopeAll:
		stocks := db.LoadAllStockCodes()
		codes := make([]string, 0, len(stocks))
		for _, s := range stocks {
			codes = append(codes, s.Code)
		}
		return codes
	case model.ScopeHeld:
		positions, _, err := db.ListPositions(sub.UID, "holding", 1, 1000)
		if err != nil {
			log.Printf("[SubscriptionRunner] 获取用户 %d 持仓失败: %v", sub.UID, err)
			return nil
		}
		codes := make([]string, 0, len(positions))
		for _, p := range positions {
			codes = append(codes, p.StockCode)
		}
		return codes
	case model.ScopeCustom:
		var codes []string
		if sub.CustomStocks != "" {
			if err := json.Unmarshal([]byte(sub.CustomStocks), &codes); err != nil {
				log.Printf("[SubscriptionRunner] 解析 custom_stocks JSON 失败: %v", err)
				return nil
			}
		}
		return codes
	default:
		return nil
	}
}

// buildStockSources 构建 []indicator.StockSource 列表
func (r *SubscriptionRunner) buildStockSources(codes []string, tradeDate int) []indicator.StockSource {
	stocks := make([]indicator.StockSource, 0, len(codes))

	utils.ConcurrentExec(codes, 20, func(i int, code string) error {
		stockDetail, err := db.FindStockByCode(code)
		if err != nil {
			// 股票不存在则跳过
			return nil
		}

		base := stocksource.NewDBStock(&stockDetail, tradeDate)
		cached := NewCachedStock(base, r.cache)
		cached.SetTradeDate(tradeDate)
		stocks = append(stocks, cached)
		return nil
	})

	return stocks
}

// renderMessage 渲染通知消息
func (r *SubscriptionRunner) renderMessage(sub *model.Subscription, strategyName string, result *RunResult) string {
	// 选择模板
	tpl := sub.Template
	if tpl == "" {
		tpl = GetDefaultTemplate(result.MatchCount > 0, false)
	}

	// 构建匹配股票列表文本
	matchList := ""
	for i, ms := range result.MatchStocks {
		matchList += fmt.Sprintf("%d. %s(%s) %.2f\n", i+1, ms.Name, ms.Code, ms.Price)
	}
	if matchList == "" {
		matchList = "无"
	}

	// 获取 scope 标签
	scopeLabel := "全部A股"
	switch sub.Scope {
	case model.ScopeHeld:
		scopeLabel = "用户持仓"
	case model.ScopeCustom:
		scopeLabel = "自选股票"
	}

	vars := map[string]string{
		"strategy_name": strategyName,
		"scope_label":   scopeLabel,
		"total_scanned": fmt.Sprintf("%d", result.TotalScanned),
		"match_count":   fmt.Sprintf("%d", result.MatchCount),
		"duration_ms":   fmt.Sprintf("%d", result.DurationMs),
		"run_time":      time.Now().Format("2006-01-02 15:04:05"),
		"match_list":    matchList,
		"error_msg":     "",
	}

	return r.notifier.Render(tpl, vars)
}

// writeLog 写入执行日志
func (r *SubscriptionRunner) writeLog(sub *model.Subscription, result *RunResult) uint {
	matchStocksJSON, _ := json.Marshal(result.MatchStocks)
	pushStatusJSON, _ := json.Marshal(result.PushStatus)

	now := time.Now()
	logEntry := &model.SubscriptionLog{
		SubscriptionID: sub.ID,
		RunTime:        now,
		FinishedAt:     &now,
		Scope:          sub.Scope,
		TotalScanned:   result.TotalScanned,
		MatchCount:     result.MatchCount,
		MatchStocks:    string(matchStocksJSON),
		DurationMs:     result.DurationMs,
		Status:         result.Status,
		PushStatus:     string(pushStatusJSON),
	}

	if err := db.CreateSubscriptionLog(logEntry); err != nil {
		log.Printf("[SubscriptionRunner] 写入执行日志失败: %v", err)
		return 0
	}
	return logEntry.ID
}

// parseSignalConfigs 解析策略 Conditions JSON 为 []indicator.SignalConfig
func parseSignalConfigs(conditionsJSON string) ([]*indicator.SignalConfig, error) {
	if conditionsJSON == "" {
		return nil, fmt.Errorf("策略条件为空")
	}

	var configs []*indicator.SignalConfig
	if err := json.Unmarshal([]byte(conditionsJSON), &configs); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	return configs, nil
}
