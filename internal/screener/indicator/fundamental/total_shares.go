package fundamental

import (
	"fmt"

	"stock-ai/internal/screener/indicator"
)

// ============================================================================
//  TotalShares — 总股本 (数值型)
//  ID: 030002 = CatCodeFundamental("03") + IndTotalSharesSeq("002")
//  数据源: StockDailySnapshot.TotalShares (int64, 单位: 股)
//  显示单位: 亿股
// ============================================================================

type TotalShares struct {
	indicator.BaseIndicator
}

// shareRangeDef 定义单个总股本内置信号的元数据
type shareRangeDef struct {
	seq          string // 2位信号序号
	name         string // 显示名称 ("小于2亿" 等)
	operator     indicator.CompareOperator
	minThreshold float64 // 阈值(亿),最小值
	maxThreshold float64 // 阈值(亿)，最大值
}

var totalShareDefs = []shareRangeDef{
	{"01", "总股本小于2亿", indicator.OpLT, 0, 2},
	{"02", "总股本2亿~5亿", indicator.OpBetween, 2, 5}, // between 用 min/max
	{"03", "总股本5亿~10亿", indicator.OpBetween, 5, 10},
	{"04", "总股本大于10亿", indicator.OpGT, 10, 0},
}

func NewTotalShares() *TotalShares {
	t := &TotalShares{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndTotalSharesSeq,
			NameStr:     "总股本",
			CategoryVal: indicator.CatFundamental,
			Desc:        "公司发行的总股本数量",
			UnitStr:     "亿股",
		},
	}
	unit := t.UnitStr

	// 数值型参数定义
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

	// 数据驱动生成4个内置信号
	builtInSigs := make([]indicator.Signal, 0, len(totalShareDefs))
	for _, def := range totalShareDefs {
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
		bs := indicator.NewBaseSignal(def.seq, def.name, fmt.Sprintf("总股本%s", def.name), indicator.ValNumber, numberOps, cfg)
		builtInSigs = append(builtInSigs, &totalShareSignal{bs})
	}
	t.SetBuiltInSignals(builtInSigs)

	// 自定义信号：用户自行设定阈值
	cs1 := indicator.NewBaseSignal("01", "总股本", "自定义总股本筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{"threshold": 10.0}})
	t.SetCustomSignals([]indicator.Signal{
		&totalShareSignal{cs1},
	})
	return t
}

func (t *TotalShares) Evaluate(stock *indicator.StockData, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: "未配置信号"}
	}
	value := t.getValue(stock)
	if value <= 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: "数据缺失: total_shares"}
	}

	for _, v := range configs {
		if s, ok := t.Signal[v.SignalID]; ok {
			if ss, ok := s.(*totalShareSignal); ok {
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

// getValue 从 StockData 中提取总股本，单位转换为 亿股
func (t *TotalShares) getValue(stock *indicator.StockData) float64 {
	if stock.DailySnapshot == nil || stock.DailySnapshot.TotalShares == 0 {
		return 0
	}
	return float64(stock.DailySnapshot.TotalShares) / 1e8 // 股 → 亿股
}

type totalShareSignal struct {
	indicator.BaseSignal
}

func (s *totalShareSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	switch config.Operator {
	case indicator.OpLT:
		threshold := config.GetFloat64("threshold", 0)
		if value < threshold {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 < %.0f亿 ✓", value, threshold)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 不小于 %.0f亿", value, threshold)}
	case indicator.OpLTE:
		threshold := config.GetFloat64("threshold", 0)
		if value <= threshold {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 ≤ %.0f亿 ✓", value, threshold)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 大于 %.0f亿", value, threshold)}
	case indicator.OpGT:
		threshold := config.GetFloat64("threshold", 0)
		if value > threshold {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 > %.0f亿 ✓", value, threshold)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 不大于 %.0f亿", value, threshold)}
	case indicator.OpGTE:
		threshold := config.GetFloat64("threshold", 0)
		if value >= threshold {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 ≥ %.0f亿 ✓", value, threshold)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 小于 %.0f亿", value, threshold)}
	case indicator.OpBetween:
		minV := config.GetFloat64("min", 0)
		maxV := config.GetFloat64("max", 100)
		if value >= minV && value <= maxV {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 在 [%.0f~%.0f]亿 ✓", value, minV, maxV)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 不在 [%.0f~%.0f]亿", value, minV, maxV)}
	case indicator.OpNotBetween:
		minV := config.GetFloat64("min", 0)
		maxV := config.GetFloat64("max", 100)
		if value < minV || value > maxV {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 不在 [%.0f~%.0f]亿 ✓", value, minV, maxV)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("总股本 %.2f亿 在 [%.0f~%.0f]亿内", value, minV, maxV)}
	default:
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: "不支持的操作符"}
	}
}
