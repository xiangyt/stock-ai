package fundamental

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  CirculateMarketCap — 流通市值 (数值型)
//  ID: 03005 = CatCodeFundamental("03") + IndCirculateMarketCapSeq("005")
//  数据源: StockDailySnapshot.CirculateMarketCap (float64, 单位: 元)
//  显示单位: 亿元
// ============================================================================

type CirculateMarketCap struct {
	indicator.BaseIndicator
}

var circulateMarketCapDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于50亿", Operator: indicator.OpLT, MinThreshold: 50, MaxThreshold: 0},
	{Seq: "02", Desc: "50亿~200亿", Operator: indicator.OpBetween, MinThreshold: 50, MaxThreshold: 200},
	{Seq: "03", Desc: "200亿~1000亿", Operator: indicator.OpBetween, MinThreshold: 200, MaxThreshold: 1000},
	{Seq: "04", Desc: "大于1000亿", Operator: indicator.OpGT, MinThreshold: 1000, MaxThreshold: 0},
}

func NewCirculateMarketCap() *CirculateMarketCap {
	c := &CirculateMarketCap{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndCirculateMarketCapSeq,
			NameStr:     "流通市值",
			CategoryVal: indicator.CatFundamental,
			Desc:        "股票流通市值 = 流通A股 × 当前股价",
			UnitStr:     "亿元",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(c.UnitStr)

	builtInSigs := signalutil.BuildRangeSignals(circulateMarketCapDefs, c.NameStr, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &circulateMarketCapSignal{bs}
	})
	c.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", c.NameStr, "自定义流通市值筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 50.0, indicator.ParamKeyMax: 500.0}})
	c.SetCustomSignals([]indicator.Signal{&circulateMarketCapSignal{cs1}})
	return c
}

func (c *CirculateMarketCap) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("circulate_market_cap").Error()}
	}
	value := c.getValue(snap)

	for _, v := range configs {
		if s, ok := c.Signal[v.SignalID]; ok {
			if ss, ok := s.(*circulateMarketCapSignal); ok {
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

func (c *CirculateMarketCap) getValue(snap *model.StockDailySnapshot) float64 {
	if snap == nil || snap.CirculateMarketCap == 0 {
		return 0
	}
	return snap.CirculateMarketCap / 1e8 // 元 → 亿元
}

type circulateMarketCapSignal struct {
	indicator.BaseSignal
}

func (s *circulateMarketCapSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "流通市值", "%.2f亿", "%.0f亿", sId, config)
}
