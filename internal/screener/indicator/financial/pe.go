package financial

import (
	"fmt"

	"stock-ai/internal/screener/indicator"
)

// ============================================================================
//  PETTM — 市盈率(TTM) (数值型)
//  ID: 040001 = CatCodeFinancial("04") + IndPETTMSeq("001")
//  数据源: StockDailySnapshot.PETTM (float64, 单位: 倍)
// ============================================================================

type PETTM struct {
	indicator.BaseIndicator
}

// peRangeDef 定义市盈率内置信号元数据
type peRangeDef struct {
	seq          string
	name         string
	operator     indicator.CompareOperator
	minThreshold float64 // 阈值(亿),最小值
	maxThreshold float64 // 阈值(亿)，最大值
}

var peDefs = []peRangeDef{
	{"01", "小于0", indicator.OpLT, 0, 0},
	{"02", "0~25", indicator.OpBetween, 0, 25},
	{"03", "25~50", indicator.OpBetween, 25, 50},
	{"04", "大于50", indicator.OpGT, 50, 0},
}

func NewPETTM() *PETTM {
	p := &PETTM{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndPETTMSeq,
			NameStr:     "市盈率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "滚动市盈率(PE-TTM)",
			UnitStr:     "倍",
		},
	}
	unit := p.UnitStr

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

	builtInSigs := make([]indicator.Signal, 0, len(peDefs))
	for _, def := range peDefs {
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
		bs := indicator.NewBaseSignal(def.seq, def.name, fmt.Sprintf("市盈率%s", def.name), indicator.ValNumber, numberOps, cfg)
		builtInSigs = append(builtInSigs, &peTTMSignal{bs})
	}
	p.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", "市盈率", "自定义市盈率筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{"min": 0, "max": 30}})
	p.SetCustomSignals([]indicator.Signal{
		&peTTMSignal{cs1},
	})
	return p
}

func (p *PETTM) Evaluate(stock *indicator.StockData, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: "未配置信号"}
	}
	value := p.getValue(stock)
	if value == 0 && !hasPEData(stock) {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: "数据缺失: PE_TTM"}
	}

	for _, v := range configs {
		if s, ok := p.Signal[v.SignalID]; ok {
			if ss, ok := s.(*peTTMSignal); ok {
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

func (p *PETTM) getValue(stock *indicator.StockData) float64 {
	if stock.DailySnapshot != nil && stock.DailySnapshot.PETTM != 0 {
		return stock.DailySnapshot.PETTM
	}
	if stock.DailySnapshot != nil && stock.DailySnapshot.PEStatic != 0 {
		return stock.DailySnapshot.PEStatic
	}
	return 0
}

func hasPEData(stock *indicator.StockData) bool {
	return stock.DailySnapshot != nil &&
		(stock.DailySnapshot.PETTM != 0 || stock.DailySnapshot.PEStatic != 0 || stock.DailySnapshot.PEDynamic != 0)
}

type peTTMSignal struct {
	indicator.BaseSignal
}

func (s *peTTMSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	label := "市盈率"
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
