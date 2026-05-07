package fundamental

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  TotalMarketCap — 总市值 (数值型)
//  ID: 03004 = CatCodeFundamental("03") + IndTotalMarketCapSeq("004")
//  数据源: StockDailySnapshot.TotalMarketCap (float64, 单位: 元)
//  显示单位: 亿元
// ============================================================================

type TotalMarketCap struct {
	indicator.BaseIndicator
}

var totalMarketCapDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于50亿", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 50},
	{Seq: "02", Desc: "50亿~200亿", Operator: indicator.OpBetween, MinThreshold: 50, MaxThreshold: 200},
	{Seq: "03", Desc: "200亿~1000亿", Operator: indicator.OpBetween, MinThreshold: 200, MaxThreshold: 1000},
	{Seq: "04", Desc: "大于1000亿", Operator: indicator.OpGT, MinThreshold: 1000, MaxThreshold: 0},
}

func NewTotalMarketCap() *TotalMarketCap {
	t := &TotalMarketCap{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndTotalMarketCapSeq,
			NameStr:     "总市值",
			CategoryVal: indicator.CatFundamental,
			Desc:        "股票总市值 = 总股本 × 当前股价",
			UnitStr:     "亿元",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(t.UnitStr)

	builtInSigs := signalutil.BuildRangeSignals(totalMarketCapDefs, t.NameStr, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &totalMarketCapSignal{bs}
	})
	t.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", t.NameStr, "自定义总市值筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 50.0, indicator.ParamKeyMax: 500.0}})
	t.SetCustomSignals([]indicator.Signal{&totalMarketCapSignal{cs1}})
	return t
}

func (t *TotalMarketCap) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("total_market_cap").Error()}
	}
	value := t.getValue(snap)

	for _, v := range configs {
		if s, ok := t.Signal[v.SignalID]; ok {
			if ss, ok := s.(*totalMarketCapSignal); ok {
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

func (t *TotalMarketCap) getValue(snap *model.StockDailySnapshot) float64 {
	if snap == nil || snap.TotalMarketCap == 0 {
		return 0
	}
	return snap.TotalMarketCap / 1e8 // 元 → 亿元
}

type totalMarketCapSignal struct {
	indicator.BaseSignal
}

func (s *totalMarketCapSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "总市值", "%.2f亿", "%.0f亿", sId, config)
}
