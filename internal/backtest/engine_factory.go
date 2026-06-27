package backtest

import "encoding/json"

// ============================================================================
//  EngineFactory — 回测引擎工厂
//
//  每次 Initiate 调用通过工厂创建独立的 Engine 实例，
//  避免 positionManager / tradeRecorder 等有状态组件在并发回测时互相干扰。
// ============================================================================

// EngineFactory 回测引擎工厂，持有可复用的无状态组件。
type EngineFactory struct {
	screener      Screener
	feeCalculator FeeCalculator
	dataProvider  DataProvider
}

// NewEngineFactory 创建引擎工厂。
func NewEngineFactory(screener Screener, feeCalculator FeeCalculator) *EngineFactory {
	return &EngineFactory{
		screener:      screener,
		feeCalculator: feeCalculator,
	}
}

// SetDataProvider 设置数据提供者（可选，在 Wire 注入后补充设置）。
func (f *EngineFactory) SetDataProvider(p DataProvider) {
	f.dataProvider = p
}

// NewEngine 为一次回测运行创建全新的引擎实例。
//
// 每次调用返回独立的 defaultEngine，其 positionManager 和 tradeRecorder
// 互不共享，保证并发安全。
// exitRules 用于构建复合退出执行器（止损/止盈/到期/移动止盈/分段止盈）。
func (f *EngineFactory) NewEngine(exitRules []ExitRule) (Engine, error) {
	opts := []EngineOption{
		WithScreener(f.screener),
		WithFeeCalculator(f.feeCalculator),
		WithTradeRecorder(NewDBTradeRecorder()),
	}
	// 根据前端传入的卖出规则构建 ExitExecutor
	if ex := buildExitExecutor(exitRules); ex != nil {
		opts = append(opts, WithExitExecutor(ex))
	}
	if f.dataProvider != nil {
		opts = append(opts, WithDataProvider(f.dataProvider))
	}
	return NewEngine(opts...)
}

// buildExitExecutor 根据退出规则列表构建 CompositeExitExecutor。
func buildExitExecutor(rules []ExitRule) *CompositeExitExecutor {
	var execs []ExitExecutor
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Type {
		case "stop_loss":
			pct := getFloatParam(r.Params, "threshold_pct", -8)
			if pct < 0 { pct = -pct } // 前端传负数，统一取绝对值
			execs = append(execs, NewStopLossExecutor(pct, 0))
		case "take_profit":
			pct := getFloatParam(r.Params, "threshold_pct", 20)
			execs = append(execs, NewTakeProfitExecutor(pct, 0))
		case "time_exit":
			days := getIntParam(r.Params, "hold_days", 10)
			execs = append(execs, NewNextDayExitExecutorWithDays(days))
		case "trailing_stop":
			trail := getFloatParam(r.Params, "trail_pct", 5)
			activation := getFloatParam(r.Params, "activation_pct", 10)
			execs = append(execs, NewTrailingStopExecutor(trail, activation))
		case "segment_profit":
			if seg := buildSegmentExecutor(r.Params); seg != nil {
				execs = append(execs, seg)
			}
		}
	}
	if len(execs) == 0 {
		return nil
	}
	return NewCompositeExitExecutor(execs...)
}

// buildSegmentExecutor 从 params 构建分段止盈执行器。
func buildSegmentExecutor(params map[string]any) *SegmentProfitExecutor {
	raw, ok := params["levels"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	levels := make([]struct{ SellRatio, ThresholdPct float64 }, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		lv := struct{ SellRatio, ThresholdPct float64 }{}
		if v, ok := m["sell_ratio"]; ok {
			lv.SellRatio = toFloat64(v)
		}
		if v, ok := m["threshold_pct"]; ok {
			lv.ThresholdPct = toFloat64(v)
		}
		levels = append(levels, lv)
	}
	if len(levels) == 0 {
		return nil
	}
	return NewSegmentProfitExecutor(levels)
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case json.Number:
		f, _ := val.Float64()
		return f
	}
	return 0
}

func getFloatParam(params map[string]any, key string, defaultVal float64) float64 {
	if v, ok := params[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case json.Number:
			f, _ := val.Float64()
			return f
		}
	}
	return defaultVal
}

func getIntParam(params map[string]any, key string, defaultVal int) int {
	if v, ok := params[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case json.Number:
			i, _ := val.Int64()
			return int(i)
		}
	}
	return defaultVal
}
