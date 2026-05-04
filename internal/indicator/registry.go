package indicator

import (
	"fmt"
	"sync"

	"stock-ai/utils"
)

// ============================================================================
//  Engine — 选股引擎
// ============================================================================

type Engine struct {
	indicators map[string]Indicator // 6位指标ID → Indicator
}

func NewEngine(indicators []Indicator) *Engine {
	e := &Engine{
		indicators: make(map[string]Indicator, len(indicators)),
	}
	for _, ind := range indicators {
		e.indicators[ind.ID()] = ind
	}
	return e
}

// EvaluatedStock 单只股票的评估结果
type EvaluatedStock struct {
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Price    float64         `json:"price"`
	Result   EvaluatedResult `json:"result"`
	SignalID string          `json:"signal_id"` // 完整8位
	Message  string          `json:"message"`
}

type ScreenResult struct {
	Passed   []EvaluatedStock `json:"passed"`
	Rejected []EvaluatedStock `json:"rejected"`
	Pending  []EvaluatedStock `json:"pending"`
}

// Execute 并发执行选股逻辑。
//
// 处理流程:
//  1. 按 SignalID[:6] (指标ID) 将配置分组 — 同一指标的信号只计算一次
//  2. 对每只股票启动 goroutine，受 semaphore 控制并发度
//  3. 每只股票内部按指标分组逐组评估 (evalStockGrouped)
//  4. 采用短路策略：任一指标不通过即返回 Pending/Rejected
//
// 参数:
//   - stocks: 待评估股票列表
//   - configs: 用户配置的所有信号条件
//   - maxConcurrency: 最大并发数 (<=0 表示不限制)
func (e *Engine) Execute(stocks []*StockData, configs []*SignalConfig, maxConcurrency int) []*EvaluatedStock {
	if len(stocks) == 0 || len(configs) == 0 {
		return nil
	}

	groups := e.groupByIndicator(configs)
	if len(groups) == 0 {
		return nil
	}

	var (
		result = make([]*EvaluatedStock, 0, len(stocks))
		mu     sync.Mutex
	)

	utils.ConcurrentExec(stocks, maxConcurrency, func(i int, stock *StockData) error {
		evaluated := e.evalStockGrouped(stocks[i], groups)
		mu.Lock()
		result = append(result, evaluated)
		mu.Unlock()
		return nil
	})
	return result
}

// groupByIndicator 按 SignalID 前6位 (指标ID) 对信号配置分组。
// 目的：同一指标下的多个信号只触发一次 Indicator.Evaluate，
// 避免重复提取/计算指标数据。
func (e *Engine) groupByIndicator(configs []*SignalConfig) map[string][]*SignalConfig {
	groups := make(map[string][]*SignalConfig)
	for _, cfg := range configs {
		indKey := cfg.GetIndicatorID()
		groups[indKey] = append(groups[indKey], cfg)
	}
	return groups
}

// evalStockGrouped 对单只股票按指标分组评估。
//
// 短路评估策略:
//   - 每个指标组调用 Indicator.Evaluate，若通过则 continue
//   - 任一指标组返回 Rejected/Pending 则立即终止并返回
//   - 所有指标组均通过才返回 Passed
//
// 返回值: 该股票的最终评估结果 (含代码、名称、价格、不通过原因)
func (e *Engine) evalStockGrouped(stock *StockData, groups map[string][]*SignalConfig) *EvaluatedStock {
	var code, name string
	var price float64
	if stock.Detail != nil {
		code = stock.Detail.Code
		name = stock.Detail.Name
	}
	if len(stock.DailyKline) > 0 {
		latest := stock.DailyKline[0]
		price = float64(latest.Close) / 100.00
	}

	ev := EvaluatedStock{
		Code:   code,
		Name:   name,
		Price:  price,
		Result: ResultRejected,
	}

	for indKey, groupConfigs := range groups {
		ind, ok := e.indicators[indKey]
		if !ok {
			ev.Message = fmt.Sprintf("指标 %s 不存在", indKey)
			return &ev
		}

		if result := ind.Evaluate(stock, groupConfigs); result.Result == ResultPassed {
			continue
		} else {
			ev.Result = ResultPending
			ev.SignalID = result.SignalID
			ev.Message = result.Message
			return &ev
		}
	}
	ev.Result = ResultPassed
	return &ev
}

type EvaluatedResult = int

const (
	ResultPassed EvaluatedResult = iota
	ResultRejected
	ResultPending
)

// ============================================================================
//  compareValue — 核心数值比较工具
//
//  支持的操作符及参数约定:
//    gt/gte/lt/lte/eq/neq → Threshold()   (单阈值)
//    between/not_between  → RangeMin() + RangeMax() (区间上下限)
//
//  用途: NumberFieldIndicator 等数值型指标的通用评估逻辑
// ============================================================================

func compareValue(actual float64, config SignalConfig) bool {
	switch config.Operator {
	case OpGT:
		return actual > config.Threshold()
	case OpGTE:
		return actual >= config.Threshold()
	case OpLT:
		return actual < config.Threshold()
	case OpLTE:
		return actual <= config.Threshold()
	case OpEQ:
		return actual == config.Threshold()
	case OpNEQ:
		return actual != config.Threshold()
	case OpBetween:
		return actual >= config.RangeMin() && actual <= config.RangeMax()
	case OpNotBetween:
		return actual < config.RangeMin() || actual > config.RangeMax()
	default:
		return false
	}
}

// ============================================================================
//  Registry — 注册中心（聚合元数据供 API 使用）
// ============================================================================

type Registry struct {
	engine      *Engine
	categories  []CategoryMeta
	enumOptions map[string][]EnumOption
}

func NewRegistry(indicators []Indicator) *Registry {
	r := &Registry{
		engine:     NewEngine(indicators),
		categories: defaultCategories(),
	}
	return r
}

func (r *Registry) Engine() *Engine { return r.engine }

func (r *Registry) ToAPIMeta() APIMeta {
	inds := r.engine.indicators
	meta := APIMeta{
		Categories:  r.categories,
		Indicators:  make([]IndicatorMeta, 0, len(inds)),
		EnumOptions: r.enumOptions,
	}
	for _, ind := range inds {
		meta.Indicators = append(meta.Indicators, ToAPIMeta(ind))
	}
	return meta
}

func (r *Registry) GetIndicatorByID(id string) (Indicator, bool) {
	ind, ok := r.engine.indicators[id]
	return ind, ok
}

func (r *Registry) ListByCategory(cat Category) []Indicator {
	var result []Indicator
	for _, ind := range r.engine.indicators {
		if ind.Category() == cat {
			result = append(result, ind)
		}
	}
	return result
}

func defaultCategories() []CategoryMeta {
	return []CategoryMeta{
		{CatTechnical, "技术面", "MACD、均线等技术指标"},
		{CatMarket, "行情面", "价格、涨跌幅、市值等实时行情"},
		{CatFundamental, "基本面", "上市板块、行业分类"},
		{CatFinancial, "财务面", "PE、ROE、EPS 等财务指标"},
	}
}
