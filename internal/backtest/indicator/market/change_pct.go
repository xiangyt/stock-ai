package market

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  ChangePct — 涨跌幅 (数值型)
//  ID: 02004 = CatCodeMarket("02") + IndChangePctSeq("004")
//  数据源: GetDailyKline() 近2条 Close (int, 单位: 分)
//  K 线顺序: 最新在前（DESC）
//  涨跌幅 = (当日收盘价 - 前日收盘价) / 前日收盘价 × 100
// ============================================================================

type ChangePct struct {
	indicator.BaseIndicator
}

var changePctDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于-5%", Alias: "大跌", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: -5},
	{Seq: "02", Desc: "-5%~0", Alias: "小跌", Operator: indicator.OpBetween, MinThreshold: -5, MaxThreshold: 0},
	{Seq: "03", Desc: "0~5%", Alias: "小涨", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 5},
	{Seq: "04", Desc: "大于5%", Alias: "大涨", Operator: indicator.OpGT, MinThreshold: 5, MaxThreshold: 0},
}

func NewChangePct() *ChangePct {
	v := &ChangePct{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndChangePctSeq,
			NameStr:     "涨跌幅",
			CategoryVal: indicator.CatMarket,
			Desc:        "当日收盘价相对前日收盘价的涨跌幅",
			UnitStr:     "%",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(v.UnitStr, 0)

	indicatorName := v.NameStr
	builtInSigs := signalutil.BuildRangeSignals(changePctDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &changePctSignal{BaseSignal: bs}
	})
	v.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义涨跌幅筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 3.0}})
	v.SetCustomSignals([]indicator.Signal{&changePctSignal{BaseSignal: cs1}})
	return v
}

func (v *ChangePct) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil || len(klines) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("daily_kline").Error()}
	}

	value := v.getValue(klines)

	for _, cfg := range configs {
		if s, ok := v.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*changePctSignal); ok {
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

func (v *ChangePct) getValue(klines []*model.DailyKline) float64 {
	// 需要至少 2 条：klines[0] = 当日，klines[1] = 前日
	if len(klines) < 2 {
		return 0
	}

	todayClose := float64(klines[0].Close)
	prevClose := float64(klines[1].Close)
	if prevClose <= 0 {
		return 0
	}

	// Close 单位是分，转换为元后计算涨跌幅
	return (todayClose - prevClose) / prevClose * 100
}

type changePctSignal struct {
	indicator.BaseSignal
}

func (s *changePctSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "涨跌幅", "%.2f%%", "%.1f%%", sId, config)
}
