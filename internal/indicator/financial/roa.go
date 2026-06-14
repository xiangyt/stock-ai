package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  ROA — 总资产收益率(%) (数值型)
//  ID: 04005 = CatCodeFinancial("04") + IndROASeq("005")
//  数据源: StockDailySnapshot.ROA (float64, 单位: %)
//  公式: 净利润 / 总资产 × 100%
// ============================================================================

type ROA struct {
	indicator.BaseIndicator
}

var roaDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于0", Alias: "亏损", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
	{Seq: "02", Desc: "0~5", Alias: "偏低", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 5},
	{Seq: "03", Desc: "5~15", Alias: "良好", Operator: indicator.OpBetween, MinThreshold: 5, MaxThreshold: 15},
	{Seq: "04", Desc: "大于15", Alias: "优秀", Operator: indicator.OpGT, MinThreshold: 15, MaxThreshold: 0},
}

func NewROA() *ROA {
	roa := &ROA{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndROASeq,
			NameStr:     "总资产收益率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "总资产收益率(ROA) = 净利润 / 总资产 × 100%",
			UnitStr:     "%",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(roa.UnitStr)

	indicatorName := "总资产收益率"
	builtInSigs := signalutil.BuildRangeSignals(roaDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &roaSignal{BaseSignal: bs}
	})
	roa.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义总资产收益率筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 5.0}})
	roa.SetCustomSignals([]indicator.Signal{&roaSignal{BaseSignal: cs1}})
	return roa
}

func (r *ROA) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("ROA").Error()}
	}
	value := r.getValue(snap)
	for _, v := range configs {
		if s, ok := r.Signal[v.SignalID]; ok {
			if ss, ok := s.(*roaSignal); ok {
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

func (r *ROA) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.ROA != 0 {
		return snap.ROA
	}
	return 0
}

type roaSignal struct {
	indicator.BaseSignal
}

func (s *roaSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "总资产收益率", "%.2f%%", "%.0f", sId, config)
}
