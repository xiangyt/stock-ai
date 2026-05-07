package fundamental

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
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

var floatShareDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于2亿", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 2},
	{Seq: "02", Desc: "2亿~5亿", Operator: indicator.OpBetween, MinThreshold: 2, MaxThreshold: 5},
	{Seq: "03", Desc: "5亿~10亿", Operator: indicator.OpBetween, MinThreshold: 5, MaxThreshold: 10},
	{Seq: "04", Desc: "大于10亿", Operator: indicator.OpGT, MinThreshold: 10, MaxThreshold: 0},
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

	numberOps := signalutil.NumberOpsByUnit(f.UnitStr)

	builtInSigs := signalutil.BuildRangeSignals(floatShareDefs, f.NameStr, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &floatShareSignal{bs}
	})
	f.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", f.NameStr, "自定义流通股本筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{indicator.ParamKeyThreshold: 10.0}})
	f.SetCustomSignals([]indicator.Signal{&floatShareSignal{cs1}})
	return f
}

func (f *FloatShares) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("float_shares").Error()}
	}
	value := f.getValue(snap)

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
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

func (f *FloatShares) getValue(snap *model.StockDailySnapshot) float64 {
	if snap == nil || snap.FloatShares == 0 {
		return 0
	}
	return float64(snap.FloatShares) / 1e8 // 股 → 亿股
}

type floatShareSignal struct {
	indicator.BaseSignal
}

func (s *floatShareSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "流通股本", "%.2f亿", "%.0f亿", sId, config)
}
