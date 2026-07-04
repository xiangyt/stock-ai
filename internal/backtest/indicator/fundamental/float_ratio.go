package fundamental

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  FloatRatio — 流通比例 (数值型)
//  ID: 03006 = CatCodeFundamental("03") + IndFloatRatioSeq("006")
//  数据源: StockDailySnapshot.FloatShares / TotalShares
//  计算公式: 流通比例 = 流通A股 / 总股本 × 100%
// ============================================================================

type FloatRatio struct {
	indicator.BaseIndicator
}

var floatRatioDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于20%", Alias: "小于20%", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 20},
	{Seq: "02", Desc: "20%~50%", Alias: "20%~50%", Operator: indicator.OpBetween, MinThreshold: 20, MaxThreshold: 50},
	{Seq: "03", Desc: "大于50%", Alias: "大于50%", Operator: indicator.OpGT, MinThreshold: 50, MaxThreshold: 0},
}

func NewFloatRatio() *FloatRatio {
	f := &FloatRatio{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndFloatRatioSeq,
			NameStr:     "流通比例",
			CategoryVal: indicator.CatFundamental,
			Desc:        "流通A股占总股本的比例",
			UnitStr:     "%",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(f.UnitStr)

	indicatorName := f.NameStr
	builtInSigs := signalutil.BuildRangeSignals(floatRatioDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &floatRatioSignal{BaseSignal: bs}
	})

	// "全流通"信号：OpEQ 不被 BuildRangeSignals 支持，单独注册
	fullFloat := indicator.NewBaseSignal(
		"04", indicatorName, "100%",
		indicator.ValNumber,
		[]indicator.OperatorOption{{Operator: indicator.OpEQ, Label: "等于", Params: []indicator.ParamDef{
			signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", 100, "%"),
		}}},
		&indicator.SignalConfig{Operator: indicator.OpEQ, Params: map[string]any{indicator.ParamKeyThreshold: 100.0}},
	)
	fullFloat.SetAlias("全流通")
	builtInSigs = append(builtInSigs, &floatRatioSignal{BaseSignal: fullFloat})

	f.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义流通比例筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 0.0, indicator.ParamKeyMax: 100.0}})
	f.SetCustomSignals([]indicator.Signal{&floatRatioSignal{BaseSignal: cs1}})
	return f
}

func (f *FloatRatio) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("float_ratio").Error()}
	}
	value := f.getValue(snap)

	for _, v := range configs {
		if s, ok := f.Signal[v.SignalID]; ok {
			if ss, ok := s.(*floatRatioSignal); ok {
				if res := ss.Evaluate(value, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// getValue 计算流通比例（%），总股本为 0 时返回 0。
func (f *FloatRatio) getValue(snap *model.StockDailySnapshot) float64 {
	if snap == nil || snap.TotalShares == 0 {
		return 0
	}
	return float64(snap.FloatShares) / float64(snap.TotalShares) * 100
}

type floatRatioSignal struct {
	indicator.BaseSignal
}

func (s *floatRatioSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "流通比例", "%.2f%%", "%.0f%%", sId, config)
}
