package fundamental

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
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

var totalShareDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于2亿", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 2},
	{Seq: "02", Desc: "2亿~5亿", Operator: indicator.OpBetween, MinThreshold: 2, MaxThreshold: 5},
	{Seq: "03", Desc: "5亿~10亿", Operator: indicator.OpBetween, MinThreshold: 5, MaxThreshold: 10},
	{Seq: "04", Desc: "大于10亿", Operator: indicator.OpGT, MinThreshold: 10, MaxThreshold: 0},
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

	numberOps := signalutil.NumberOpsByUnit(t.UnitStr)

	builtInSigs := signalutil.BuildRangeSignals(totalShareDefs, "总股本", numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &totalShareSignal{bs}
	})
	t.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", "总股本", "自定义总股本筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{indicator.ParamKeyThreshold: 10.0}})
	t.SetCustomSignals([]indicator.Signal{&totalShareSignal{cs1}})
	return t
}

func (t *TotalShares) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("total_shares").Error()}
	}
	value := t.getValue(snap)

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
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// getValue 从 StockDailySnapshot 中提取总股本，单位转换为 亿股
func (t *TotalShares) getValue(snap *model.StockDailySnapshot) float64 {
	if snap == nil || snap.TotalShares == 0 {
		return 0
	}
	return float64(snap.TotalShares) / 1e8
}

type totalShareSignal struct {
	indicator.BaseSignal
}

func (s *totalShareSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "总股本", "%.2f亿", "%.0f亿", sId, config)
}
