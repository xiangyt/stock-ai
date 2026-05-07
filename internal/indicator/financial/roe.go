package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  ROE — 净资产收益率(%) (数值型)
//  ID: 04004 = CatCodeFinancial("04") + IndROESeq("004")
//  数据源: StockDailySnapshot.ROE (float64, 单位: %)
//  公式: 归属母公司净利润 / 归属母公司股东权益 × 100%
//  注: 快照中使用加权平均 ROE (roe_w)
// ============================================================================

type ROE struct {
	indicator.BaseIndicator
}

var roeDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于0", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
	{Seq: "02", Desc: "0~10", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 10},
	{Seq: "03", Desc: "10~20", Operator: indicator.OpBetween, MinThreshold: 10, MaxThreshold: 20},
	{Seq: "04", Desc: "大于20", Operator: indicator.OpGT, MinThreshold: 20, MaxThreshold: 0},
}

func NewROE() *ROE {
	roe := &ROE{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndROESeq,
			NameStr:     "净资产收益率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "净资产收益率(ROE) = 归属母公司净利润 / 归属母公司股东权益 × 100%",
			UnitStr:     "%",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(roe.UnitStr)

	builtInSigs := signalutil.BuildRangeSignals(roeDefs, "净资产收益率", numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &roeSignal{bs}
	})
	roe.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", "净资产收益率", "自定义净资产收益率筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 15.0}})
	roe.SetCustomSignals([]indicator.Signal{&roeSignal{cs1}})
	return roe
}

func (r *ROE) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("ROE").Error()}
	}
	value := r.getValue(snap)
	for _, v := range configs {
		if s, ok := r.Signal[v.SignalID]; ok {
			if ss, ok := s.(*roeSignal); ok {
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

func (r *ROE) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.ROE != 0 {
		return snap.ROE
	}
	return 0
}

type roeSignal struct {
	indicator.BaseSignal
}

func (s *roeSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "净资产收益率", "%.2f%%", "%.0f", sId, config)
}
