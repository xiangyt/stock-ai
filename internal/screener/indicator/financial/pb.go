package financial

import (
	"fmt"

	"stock-ai/internal/screener/indicator"
)

// ============================================================================
//  PB — 市净率 (数值型)
//  ID: 040002 = CatCodeFinancial("04") + IndPBSeq("002")
//  数据源: StockDailySnapshot.PB (float64, 单位: 倍)
// ============================================================================

type PB struct {
	indicator.BaseIndicator
}

var pbDefs = []peRangeDef{
	{"01", "0~1", indicator.OpBetween, 0, 1},
	{"02", "1~2", indicator.OpBetween, 1, 2},
	{"03", "2~5", indicator.OpBetween, 2, 5},
	{"04", "大于5", indicator.OpGT, 5, 0},
}

func NewPB() *PB {
	pb := &PB{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndPBSeq,
			NameStr:     "市净率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "市净率(PB)",
			UnitStr:     "倍",
		},
	}
	unit := pb.UnitStr

	threshParam := indicator.ParamDef{Key: "threshold", Label: "阈值", Type: "number", Unit: unit}
	rangeParamMin := indicator.ParamDef{Key: "min", Label: "下限", Type: "number", Unit: unit}
	rangeParamMax := indicator.ParamDef{Key: "max", Label: "上限", Type: "number", Unit: unit}

	numberOps := []indicator.OperatorOption{
		{Operator: indicator.OpLT, Label: "小于", Params: []indicator.ParamDef{threshParam}},
		{Operator: indicator.OpLTE, Label: "小于等于", Params: []indicator.ParamDef{threshParam}},
		{Operator: indicator.OpGT, Label: "大于", Params: []indicator.ParamDef{threshParam}},
		{Operator: indicator.OpGTE, Label: "大于等于", Params: []indicator.ParamDef{threshParam}},
		{Operator: indicator.OpBetween, Label: "区间内", Params: []indicator.ParamDef{rangeParamMin, rangeParamMax}},
		{Operator: indicator.OpNotBetween, Label: "区间外", Params: []indicator.ParamDef{rangeParamMin, rangeParamMax}},
	}

	builtInSigs := make([]indicator.Signal, 0, len(pbDefs))
	for _, def := range pbDefs {
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
		bs := indicator.NewBaseSignal(def.seq, def.name, fmt.Sprintf("市净率%s", def.name), indicator.ValNumber, numberOps, cfg)
		builtInSigs = append(builtInSigs, &pbSignal{bs})
	}
	pb.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", "市净率", "自定义市净率筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{"threshold": 3.0}})
	pb.SetCustomSignals([]indicator.Signal{
		&pbSignal{cs1},
	})
	return pb
}

func (pb *PB) Evaluate(stock *indicator.StockData, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: "未配置信号"}
	}
	value := pb.getValue(stock)
	if value == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: "数据缺失: PB"}
	}

	for _, v := range configs {
		if s, ok := pb.Signal[v.SignalID]; ok {
			if ss, ok := s.(*pbSignal); ok {
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

func (pb *PB) getValue(stock *indicator.StockData) float64 {
	if stock.DailySnapshot != nil && stock.DailySnapshot.PB != 0 {
		return stock.DailySnapshot.PB
	}
	return 0
}

type pbSignal struct {
	indicator.BaseSignal
}

func (s *pbSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	label := "市净率"
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	switch config.Operator {
	case indicator.OpLT:
		t := config.GetFloat64("threshold", 0)
		if value < t {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f < %.0f ✓", label, value, t)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f 不小于 %.0f", label, value, t)}
	case indicator.OpLTE:
		t := config.GetFloat64("threshold", 0)
		if value <= t {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f ≤ %.0f ✓", label, value, t)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f 大于 %.0f", label, value, t)}
	case indicator.OpGT:
		t := config.GetFloat64("threshold", 0)
		if value > t {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f > %.0f ✓", label, value, t)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f 不大于 %.0f", label, value, t)}
	case indicator.OpGTE:
		t := config.GetFloat64("threshold", 0)
		if value >= t {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f ≥ %.0f ✓", label, value, t)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f 小于 %.0f", label, value, t)}
	case indicator.OpBetween:
		minV := config.GetFloat64("min", 0)
		maxV := config.GetFloat64("max", 100)
		if value >= minV && value <= maxV {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f 在 [%.0f~%.0f] ✓", label, value, minV, maxV)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f 不在 [%.0f~%.0f]", label, value, minV, maxV)}
	case indicator.OpNotBetween:
		minV := config.GetFloat64("min", 0)
		maxV := config.GetFloat64("max", 100)
		if value < minV || value > maxV {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s %.2f 不在 [%.0f~%.0f] ✓", label, value, minV, maxV)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s %.2f 在 [%.0f~%.0f] 内", label, value, minV, maxV)}
	default:
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: "不支持的操作符"}
	}
}
