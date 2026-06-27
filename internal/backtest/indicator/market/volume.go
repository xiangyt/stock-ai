package market

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  Volume — 成交量 (数值型)
//  ID: 02001 = CatCodeMarket("02") + IndVolumeSeq("001")
//  数据源: GetDailyKline()[0].Volume (int64, 单位: 股)
//  显示单位: 万手 (1万手 = 100万股)
// ============================================================================

type Volume struct {
	indicator.BaseIndicator
}

var volumeDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于1万手", Alias: "缩量", Operator: indicator.OpLT, MinThreshold: 1, MaxThreshold: 0},
	{Seq: "02", Desc: "1万手~5万手", Alias: "正常量", Operator: indicator.OpBetween, MinThreshold: 1, MaxThreshold: 5},
	{Seq: "03", Desc: "5万手~20万手", Alias: "放量", Operator: indicator.OpBetween, MinThreshold: 5, MaxThreshold: 20},
	{Seq: "04", Desc: "大于20万手", Alias: "巨量", Operator: indicator.OpGT, MinThreshold: 20, MaxThreshold: 0},
}

func NewVolume() *Volume {
	v := &Volume{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndVolumeSeq,
			NameStr:     "成交量",
			CategoryVal: indicator.CatMarket,
			Desc:        "当日成交量（最新一个交易日）",
			UnitStr:     "万手",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(v.UnitStr, 0)

	indicatorName := v.NameStr
	builtInSigs := signalutil.BuildRangeSignals(volumeDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &volumeSignal{BaseSignal: bs}
	})
	v.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义成交量筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 5.0}})
	v.SetCustomSignals([]indicator.Signal{&volumeSignal{BaseSignal: cs1}})
	return v
}

func (v *Volume) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil || len(klines) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("daily_kline").Error()}
	}
	// 取第一条数据（按用户要求）
	value := v.getValue(klines[0])

	for _, cfg := range configs {
		if s, ok := v.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*volumeSignal); ok {
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

func (v *Volume) getValue(kline *model.DailyKline) float64 {
	if kline == nil || kline.Volume == 0 {
		return 0
	}
	// 股 → 万手 (1万手 = 100万股 = 1e6股)
	return float64(kline.Volume) / 1e6
}

type volumeSignal struct {
	indicator.BaseSignal
}

func (s *volumeSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "成交量", "%.2f万手", "%.0f万手", sId, config)
}
