package fundamental

import (
	"fmt"

	"stock-ai/internal/screener/indicator"
)

// ============================================================================
//  FloatShares — 流通股本 (数值型)
//  ID: 030003 = CatCodeFundamental("03") + IndFloatSharesSeq("003")
//  数据源: StockDailySnapshot.FloatShares (int64, 单位: 股)
//  显示单位: 亿股
// ============================================================================

type FloatShares struct {
	indicator.BaseIndicator
}

var floatShareDefs = []shareRangeDef{
	{"01", "流通股本小于2亿", indicator.OpLT, 0, 2},
	{"02", "流通股本2亿~5亿", indicator.OpBetween, 2, 5},
	{"03", "流通股本5亿~10亿", indicator.OpBetween, 5, 10},
	{"04", "流通股本大于10亿", indicator.OpGT, 10, 0},
}

func NewFloatShares() *FloatShares {
	f := &FloatShares{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndFloatSharesSeq,
			NameStr:     "流通股本",
			CategoryVal: indicator.CatFundamental,
			Desc:        "公司流通A股数量",
			UnitStr:     "亿股",
		},
	}
	unit := f.UnitStr

	threshParam := indicator.ParamDef{Key: "threshold", Label: "阈值", Type: "number", Unit: unit, Min: 0}
	rangeParamMin := indicator.ParamDef{Key: "min", Label: "下限", Type: "number", Unit: unit, Min: 0}
	rangeParamMax := indicator.ParamDef{Key: "max", Label: "上限", Type: "number", Unit: unit, Min: 0}

	numberOps := []indicator.OperatorOption{
		{Operator: indicator.OpLT, Label: "小于", Params: []indicator.ParamDef{threshParam}},
		{Operator: indicator.OpLTE, Label: "小于等于", Params: []indicator.ParamDef{threshParam}},
		{Operator: indicator.OpGT, Label: "大于", Params: []indicator.ParamDef{threshParam}},
		{Operator: indicator.OpGTE, Label: "大于等于", Params: []indicator.ParamDef{threshParam}},
		{Operator: indicator.OpBetween, Label: "区间内", Params: []indicator.ParamDef{rangeParamMin, rangeParamMax}},
		{Operator: indicator.OpNotBetween, Label: "区间外", Params: []indicator.ParamDef{rangeParamMin, rangeParamMax}},
	}

	builtInSigs := make([]indicator.Signal, 0, len(floatShareDefs))
	for _, def := range floatShareDefs {
		var cfg *indicator.SignalConfig
		switch def.operator {
		case indicator.OpLT, indicator.OpLTE:
			cfg = &indicator.SignalConfig{Operator: def.operator, Params: map[string]any{"threshold": def.maxThreshold}}
		case indicator.OpBetween, indicator.OpNotBetween:
			cfg = &indicator.SignalConfig{Operator: indicator.OpBetween,
				Params: map[string]any{"min": def.minThreshold, "max": def.maxThreshold}}
		case indicator.OpGT, indicator.OpGTE:
			cfg = &indicator.SignalConfig{Operator: def.operator, Params: map[string]any{"threshold": def.minThreshold}}
		default:
			continue
		}
		bs := indicator.NewBaseSignal(def.seq, def.name, fmt.Sprintf("流通股本%s", def.name), indicator.ValNumber, numberOps, cfg)
		builtInSigs = append(builtInSigs, &floatShareSignal{bs})
	}
	f.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", "流通股本", "自定义流通股本筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{"threshold": 10.0}})
	f.SetCustomSignals([]indicator.Signal{
		&floatShareSignal{cs1},
	})
	return f
}

func (f *FloatShares) Evaluate(stock *indicator.StockData, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: "未配置信号"}
	}
	value := f.getValue(stock)
	if value <= 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: "数据缺失: float_shares"}
	}

	for _, v := range configs {
		if s, ok := f.Signal[v.SignalID]; ok {
			if ss, ok := s.(*floatShareSignal); ok {
				if res := ss.Evaluate(value, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: "不支持的信号"}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

func (f *FloatShares) getValue(stock *indicator.StockData) float64 {
	if stock.DailySnapshot == nil || stock.DailySnapshot.FloatShares == 0 {
		return 0
	}
	return float64(stock.DailySnapshot.FloatShares) / 1e8 // 股 → 亿股
}

type floatShareSignal struct {
	indicator.BaseSignal
}

func (s *floatShareSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	label := "流通股本"
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	switch config.Operator {
	case indicator.OpLT:
		t := config.GetFloat64("threshold", 0)
		if value < t {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 < %.0f亿 ✓", label, value, t)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 不小于 %.0f亿", label, value, t)}
	case indicator.OpLTE:
		t := config.GetFloat64("threshold", 0)
		if value <= t {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 ≤ %.0f亿 ✓", label, value, t)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 大于 %.0f亿", label, value, t)}
	case indicator.OpGT:
		t := config.GetFloat64("threshold", 0)
		if value > t {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 > %.0f亿 ✓", label, value, t)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 不大于 %.0f亿", label, value, t)}
	case indicator.OpGTE:
		t := config.GetFloat64("threshold", 0)
		if value >= t {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 ≥ %.0f亿 ✓", label, value, t)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 小于 %.0f亿", label, value, t)}
	case indicator.OpBetween:
		minV := config.GetFloat64("min", 0)
		maxV := config.GetFloat64("max", 100)
		if value >= minV && value <= maxV {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 在 [%.0f~%.0f]亿 ✓", label, value, minV, maxV)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 不在 [%.0f~%.0f]亿", label, value, minV, maxV)}
	case indicator.OpNotBetween:
		minV := config.GetFloat64("min", 0)
		maxV := config.GetFloat64("max", 100)
		if value < minV || value > maxV {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 不在 [%.0f~%.0f]亿 ✓", label, value, minV, maxV)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f亿 在 [%.0f~%.0f]亿内", label, value, minV, maxV)}
	default:
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: "不支持的操作符"}
	}
}
