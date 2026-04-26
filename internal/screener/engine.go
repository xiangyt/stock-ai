package screener

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"stock-ai/internal/model"
)

// ============================================================================
//  ScreenerEngine — 选股引擎
//
//  核心思想: 双重循环遍历 — 外层股票，内层信号列表
//
//    for stock in stocks:           ← 第一层：遍历所有股票
//      for signal in signals:       ← 第二层：依次检查每个条件
//        eval(stock, signal)        ← 通过继续，不通过标记失败
//    全部通过 → Passed / 任一不通过 → Rejected / 数据错误 → Pending
//
//    MaxConcurrency 控制同时执行的股票数量（0 = 不限制）
// ============================================================================

// Evaluator 信号评估函数类型
type Evaluator func(stock *model.StockData, signal *model.Signal) *model.EvaluateResult

// Engine 选股引擎
type Engine struct {
	evaluatorRegistry map[string]Evaluator // indicator_id → 评估函数
	mu                sync.RWMutex
}

// NewEngine 创建新的选股引擎
func NewEngine() *Engine {
	e := &Engine{
		evaluatorRegistry: make(map[string]Evaluator),
	}
	return e
}

// RegisterEvaluator 注册指标对应的评估函数
func (e *Engine) RegisterEvaluator(indicatorID string, evaluator Evaluator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evaluatorRegistry[indicatorID] = evaluator
	log.Printf("[screener] 注册指标评估器: %s", indicatorID)
}

// getEvaluator 安全获取评估函数
func (e *Engine) getEvaluator(indicatorID string) (Evaluator, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	fn, ok := e.evaluatorRegistry[indicatorID]
	return fn, ok
}

// RegisteredIndicators 返回所有已注册的指标 ID 列表
func (e *Engine) RegisteredIndicators() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ids := make([]string, 0, len(e.evaluatorRegistry))
	for id := range e.evaluatorRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ---------- 核心：滤网执行 ----------

// ExecuteContext 筛选执行上下文
type ExecuteContext struct {
	Ctx            context.Context // 标准上下文（超时、取消、trace）
	Limit          int             // 最大返回数量, 0=不限制
	Debug          bool            // 是否输出调试信息
	MaxConcurrency int             // 最大并发股票数, 0或1=不限制(全量并发), >1=semaphore限流
}

// Execute 执行选股筛选
//
// 执行流程:
//  1. 按 Order 排序信号
//  2. 双重 for 循环：外层股票，内层信号列表
//  3. 每只股票跑完全部信号，全通过 → Passed，任一不通过 → Rejected，数据错误 → Pending
func (e *Engine) Execute(ctx *ExecuteContext, signals []model.Signal, stocks []model.StockData) *model.ScreenResultV2 {
	startTime := time.Now()

	result := &model.ScreenResultV2{
		TotalScanned: len(stocks),
		Passed:       make([]model.EvaluatedStock, 0),
		Rejected:     make([]model.EvaluatedStock, 0),
		Pending:      make([]model.EvaluatedStock, 0),
		SignalStats:  make(map[string]int),
	}

	// 按 Order 排序 + 过滤 disabled
	sortedSignals := make([]model.Signal, 0, len(signals))
	for i := range signals {
		if signals[i].Enabled {
			sortedSignals = append(sortedSignals, signals[i])
			result.SignalStats[signals[i].ID] = 0
		}
	}
	sort.Slice(sortedSignals, func(i, j int) bool {
		return sortedSignals[i].Order < sortedSignals[j].Order
	})

	if len(sortedSignals) == 0 {
		result.Pending = makeEvaluatedStocks(stocks, "无信号条件")
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	maxConc := ctx.MaxConcurrency

	// ===== 双重循环：外层股票 × 内层信号 =====

	if maxConc <= 1 {
		// 全量并发：每只股票一个 goroutine，不限制数量
		var wg sync.WaitGroup
		var mu sync.Mutex

		wg.Add(len(stocks))
		for i := range stocks {
			go func(idx int) {
				defer wg.Done()
				evaluated := e.evalStock(&stocks[idx], sortedSignals)

				mu.Lock()
				defer mu.Unlock()
				for sigID, count := range evaluated.signalStats {
					result.SignalStats[sigID] += count
				}
				e.classifyResult(result, evaluated)
			}(i)
		}
		wg.Wait()
	} else {
		// 限流并发：semaphore 控制同时执行的股票数
		var mu sync.Mutex
		sem := make(chan struct{}, maxConc)

		for i := range stocks {
			sem <- struct{}{}
			go func(idx int) {
				defer func() { <-sem }()

				evaluated := e.evalStock(&stocks[idx], sortedSignals)

				mu.Lock()
				defer mu.Unlock()
				for sigID, count := range evaluated.signalStats {
					result.SignalStats[sigID] += count
				}
				e.classifyResult(result, evaluated)
			}(i)
		}

		// 等待所有 goroutine 完成
		for j := 0; j < cap(sem); j++ {
			sem <- struct{}{}
		}
	}

	// 截断
	if ctx.Limit > 0 {
		if len(result.Passed) > ctx.Limit {
			result.Passed = result.Passed[:ctx.Limit]
		}
		if len(result.Rejected) > ctx.Limit {
			result.Rejected = result.Rejected[:ctx.Limit]
		}
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	concurrencyLabel := "全量并发"
	if maxConc > 1 {
		concurrencyLabel = fmt.Sprintf("限流%d", maxConc)
	}
	log.Printf("[screener] 筛选完成: 扫描=%d 通过=%d 驳回=%d 待定=%d 耗时=%dms 条件数=%d 模式=%s",
		len(stocks), len(result.Passed), len(result.Rejected), len(result.Pending),
		result.DurationMs, len(sortedSignals), concurrencyLabel)

	return result
}

// ---------- stockEvalResult 单只股票的评估结果（内部中间态） ----------

type stockEvalResult struct {
	code           string
	name           string
	price          float64
	changePct      float64
	matchedSignals []string
	failedSignals  []string
	signalDetails  map[string]string
	dataErrors     []string
	reason         string
	signalStats    map[string]int // 各信号通过计数（并发时用于合并）
}

// evalStock 对单只股票执行全部信号的评估（内层循环）
func (e *Engine) evalStock(stock *model.StockData, signals []model.Signal) stockEvalResult {
	r := stockEvalResult{
		code:           stock.Code,
		name:           stock.Name,
		price:          stock.Current["close"],
		changePct:      stock.Current["change_pct"],
		matchedSignals: make([]string, 0, len(signals)),
		failedSignals:  make([]string, 0),
		signalDetails:  make(map[string]string, len(signals)),
		dataErrors:     make([]string, 0),
		signalStats:    make(map[string]int),
	}

	for _, sig := range signals {
		evalFn, ok := e.getEvaluator(sig.IndicatorID)
		if !ok {
			log.Printf("[screener] ⚠ 未注册的指标: %s (signal=%s)", sig.IndicatorID, sig.ID)
			r.failedSignals = append(r.failedSignals, sig.ID)
			continue
		}

		result := evalFn(stock, &sig)

		if result.Error != nil {
			r.dataErrors = append(r.dataErrors, fmt.Sprintf("%s:%v", sig.ID, result.Error))
			r.signalDetails[sig.ID] = fmt.Sprintf("错误: %v", result.Error)
			continue
		}

		if result.Pass {
			r.matchedSignals = append(r.matchedSignals, sig.ID)
			r.signalStats[sig.ID]++
		} else {
			r.failedSignals = append(r.failedSignals, sig.ID)
		}

		r.signalDetails[sig.ID] = result.Detail
	}

	return r
}

// classifyResult 将单只股票的评估结果分类到 Passed / Rejected / Pending
func (e *Engine) classifyResult(result *model.ScreenResultV2, r stockEvalResult) {
	if len(r.dataErrors) > 0 {
		result.Pending = append(result.Pending, model.EvaluatedStock{
			Code:           r.code,
			Name:           r.name,
			Price:          r.price,
			ChangePct:      r.changePct,
			MatchedSignals: r.matchedSignals,
			FailedSignals:  r.failedSignals,
			SignalDetails:  r.signalDetails,
			Reason:         strings.Join(r.dataErrors, "; "),
		})
	} else if len(r.failedSignals) == 0 {
		result.Passed = append(result.Passed, model.EvaluatedStock{
			Code:           r.code,
			Name:           r.name,
			Price:          r.price,
			ChangePct:      r.changePct,
			MatchedSignals: r.matchedSignals,
			FailedSignals:  r.failedSignals,
			SignalDetails:  r.signalDetails,
		})
	} else {
		result.Rejected = append(result.Rejected, model.EvaluatedStock{
			Code:           r.code,
			Name:           r.name,
			Price:          r.price,
			ChangePct:      r.changePct,
			MatchedSignals: r.matchedSignals,
			FailedSignals:  r.failedSignals,
			SignalDetails:  r.signalDetails,
			Reason:         r.failedSignals[0],
		})
	}
}

// ============================================================================
//  内置通用比较评估器工厂函数
// ============================================================================

// NumberEvaluatorFactory 创建数值型比较的通用评估器
func NumberEvaluatorFactory(fieldName, unit string) Evaluator {
	return func(stock *model.StockData, signal *model.Signal) *model.EvaluateResult {
		start := time.Now()
		result := &model.EvaluateResult{
			SignalID:  signal.ID,
			Pass:      false,
			CostNanos: 0,
		}

		value, exists := stock.Current[fieldName]
		if !exists {
			result.Error = fmt.Errorf("字段 %s 不存在", fieldName)
			result.CostNanos = time.Since(start).Nanoseconds()
			return result
		}

		result.RawValue = value
		pass, detail := compareValue(value, signal.Operator,
			signal.ValueNumber,
			signal.GetEffectiveMin(),
			signal.GetEffectiveMax(),
			unit)
		result.Pass = pass
		result.Detail = detail
		result.CostNanos = time.Since(start).Nanoseconds()

		return result
	}
}

// FinancialEvaluatorFactory 创建财报字段的通用评估器
func FinancialEvaluatorFactory(fieldName, unit string) Evaluator {
	return func(stock *model.StockData, signal *model.Signal) *model.EvaluateResult {
		start := time.Now()
		result := &model.EvaluateResult{
			SignalID:  signal.ID,
			Pass:      false,
			CostNanos: 0,
		}

		value, exists := stock.Financial[fieldName]
		if !exists {
			result.Error = fmt.Errorf("财报字段 %s 不存在", fieldName)
			result.CostNanos = time.Since(start).Nanoseconds()
			return result
		}

		result.RawValue = value
		pass, detail := compareValue(value, signal.Operator,
			signal.ValueNumber,
			signal.GetEffectiveMin(),
			signal.GetEffectiveMax(),
			unit)
		result.Pass = pass
		result.Detail = detail
		result.CostNanos = time.Since(start).Nanoseconds()

		return result
	}
}

// EnumEvaluatorFactory 创建枚举型评估器（行业、板块等）
func EnumEvaluatorFactory(fieldName string) Evaluator {
	return func(stock *model.StockData, signal *model.Signal) *model.EvaluateResult {
		start := time.Now()
		result := &model.EvaluateResult{
			SignalID:  signal.ID,
			Pass:      false,
			CostNanos: 0,
		}

		strVal, exists := stock.Info[fieldName]
		if !exists {
			result.Error = fmt.Errorf("枚举字段 %s 不存在", fieldName)
			result.CostNanos = time.Since(start).Nanoseconds()
			return result
		}

		pass := false
		switch signal.Operator {
		case model.OpIn:
			for _, v := range signal.ValueList {
				if strings.EqualFold(strVal, v) {
					pass = true
					break
				}
			}
		case model.OpNotIn:
			pass = true
			for _, v := range signal.ValueList {
				if strings.EqualFold(strVal, v) {
					pass = false
					break
				}
			}
		case model.OpContains:
			for _, v := range signal.ValueList {
				if strings.Contains(strVal, v) {
					pass = true
					break
				}
			}
		case model.OpEQ:
			pass = strings.EqualFold(strVal, signal.ValueList[0])
		}

		result.Pass = pass
		opSymbol := model.OperatorMeta[signal.Operator].Symbol
		result.Detail = fmt.Sprintf("%s=%s %s %v → %v",
			fieldName, strVal, opSymbol, signal.ValueList, passMark(pass))

		result.CostNanos = time.Since(start).Nanoseconds()
		return result
	}
}

// ---------- 通用比较逻辑 ----------

// compareValue 执行数值比较操作
func compareValue(value float64, op model.CompareOperator,
	valNum, valLow, valHigh float64, unit string) (bool, string) {

	var pass bool

	switch op {
	case model.OpGT:
		pass = value > valNum
	case model.OpGTE:
		pass = value >= valNum
	case model.OpLT:
		pass = value < valNum
	case model.OpLTE:
		pass = value <= valNum
	case model.OpEQ:
		pass = math.Abs(value-valNum) < 1e-9
	case model.OpNEQ:
		pass = math.Abs(value-valNum) >= 1e-9
	case model.OpBetween:
		pass = value >= valLow && value <= valHigh
	case model.OpNotBetween:
		pass = value < valLow || value > valHigh
	default:
		pass = false
	}

	detail := formatDetail(value, op, valNum, valLow, valHigh, unit, pass)
	return pass, detail
}

// formatDetail 格式化详情字符串
func formatDetail(value float64, op model.CompareOperator,
	valNum, valLow, valHigh float64, unit string, pass bool) string {

	opSym := model.OperatorMeta[op].Symbol
	formattedVal := formatNumber(value, unit)

	switch op {
	case model.OpGT, model.OpGTE, model.OpLT, model.OpLTE, model.OpEQ, model.OpNEQ:
		return fmt.Sprintf("%s%s %s %s%s → %v",
			formattedVal, unit, opSym, formatNumber(valNum, unit), unit, passMark(pass))
	case model.OpBetween, model.OpNotBetween:
		return fmt.Sprintf("%s%s %s [%s%s, %s%s] → %v",
			formattedVal, unit, opSym,
			formatNumber(valLow, unit), unit, formatNumber(valHigh, unit), unit, passMark(pass))
	default:
		return fmt.Sprintf("%s → ? (未知操作符 %s)", formattedVal, op)
	}
}

// formatNumber 格式化数字显示
func formatNumber(v float64, unit string) string {
	switch unit {
	case "%":
		return fmt.Sprintf("%.2f", v)
	case "倍", "":
		return fmt.Sprintf("%.2f", v)
	case "亿":
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

// passMark 通过标记
func passMark(pass bool) string {
	if pass {
		return "✓"
	}
	return "✗"
}

// ---------- 辅助函数 ----------

// makeEvaluatedStocks 从 StockData 列表快速构造 EvaluatedStock 列片（用于 Pending 等批量场景）
func makeEvaluatedStocks(stocks []model.StockData, reason string) []model.EvaluatedStock {
	result := make([]model.EvaluatedStock, len(stocks))
	for i := range stocks {
		result[i] = model.EvaluatedStock{
			Code:   stocks[i].Code,
			Name:   stocks[i].Name,
			Price:  stocks[i].Current["close"],
			Reason: reason,
		}
	}
	return result
}
