package market

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  Amount — 成交额 (数值型)
//  ID: 02002 = CatCodeMarket("02") + IndAmountSeq("002")
//  数据源: GetDailyKline()[0].Amount (int64, 单位: 分)
//  显示单位: 亿元
// ============================================================================

type Amount struct {
	indicator.BaseIndicator
}

var amountDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于1亿", Alias: "低成交", Operator: indicator.OpLT, MinThreshold: 1, MaxThreshold: 0},
	{Seq: "02", Desc: "1亿~10亿", Alias: "正常成交", Operator: indicator.OpBetween, MinThreshold: 1, MaxThreshold: 10},
	{Seq: "03", Desc: "10亿~100亿", Alias: "活跃成交", Operator: indicator.OpBetween, MinThreshold: 10, MaxThreshold: 100},
	{Seq: "04", Desc: "大于100亿", Alias: "爆量", Operator: indicator.OpGT, MinThreshold: 100, MaxThreshold: 0},
}

func NewAmount() *Amount {
	a := &Amount{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndAmountSeq,
			NameStr:     "成交额",
			CategoryVal: indicator.CatMarket,
			Desc:        "当日成交额（最新一个交易日）",
			UnitStr:     "亿元",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(a.UnitStr, 0)

	indicatorName := a.NameStr
	builtInSigs := signalutil.BuildRangeSignals(amountDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &amountSignal{BaseSignal: bs}
	})
	a.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义成交额筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 5.0}})
	a.SetCustomSignals([]indicator.Signal{&amountSignal{BaseSignal: cs1}})
	return a
}

func (a *Amount) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil || len(klines) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("daily_kline").Error()}
	}
	value := a.getValue(klines[0])

	for _, cfg := range configs {
		if s, ok := a.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*amountSignal); ok {
				if res := ss.Evaluate(value, cfg); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

func (a *Amount) getValue(kline *model.DailyKline) float64 {
	if kline == nil || kline.Amount == 0 {
		return 0
	}
	// 分 → 亿元 (1亿 = 1e8元 = 1e10分)
	return float64(kline.Amount) / 1e10
}

type amountSignal struct {
	indicator.BaseSignal
}

func (s *amountSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "成交额", "%.2f亿", "%.0f亿", sId, config)
}
