package market

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  VolumeRatio — 量比 (数值型)
//  ID: 02003 = CatCodeMarket("02") + IndVolumeRatioSeq("003")
//  K 线顺序: 最新在前（DESC），klines[0] = 当日，klines[1:6] = 过去5日
//  量比 = 当日成交量 / 近5日平均成交量
// ============================================================================

type VolumeRatio struct {
	indicator.BaseIndicator
}

var volumeRatioDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于1", Alias: "缩量", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 1},
	{Seq: "02", Desc: "1~3", Alias: "正常量比", Operator: indicator.OpBetween, MinThreshold: 1, MaxThreshold: 3},
	{Seq: "03", Desc: "3~5", Alias: "放量", Operator: indicator.OpBetween, MinThreshold: 3, MaxThreshold: 5},
	{Seq: "04", Desc: "大于5", Alias: "巨量", Operator: indicator.OpGT, MinThreshold: 5, MaxThreshold: 0},
}

func NewVolumeRatio() *VolumeRatio {
	v := &VolumeRatio{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndVolumeRatioSeq,
			NameStr:     "量比",
			CategoryVal: indicator.CatMarket,
			Desc:        "当日成交量与近5日均量的比值",
			UnitStr:     "",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(v.UnitStr, 0)

	indicatorName := v.NameStr
	builtInSigs := signalutil.BuildRangeSignals(volumeRatioDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &volumeRatioSignal{BaseSignal: bs}
	})
	v.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "量比是股市中衡量相对成交量的重要指标，主要用于反映当前盘口的成交力度与过去5个交易日的平均成交力度相比是放大还是缩小。", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 1.5}})
	v.SetCustomSignals([]indicator.Signal{&volumeRatioSignal{BaseSignal: cs1}})
	return v
}

func (v *VolumeRatio) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil || len(klines) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("daily_kline").Error()}
	}

	// 量比 = 当日成交量 / 近5日平均成交量（不足5条则用已有条数）
	value := v.getValue(klines)

	for _, cfg := range configs {
		if s, ok := v.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*volumeRatioSignal); ok {
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

func (v *VolumeRatio) getValue(klines []*model.DailyKline) float64 {
	// 需要至少 6 条：klines[0] = 当日，klines[1]~klines[5] = 过去5日
	if len(klines) < 6 {
		return 0
	}

	latestVol := float64(klines[0].Volume)
	if latestVol <= 0 {
		return 0
	}

	// klines[1:6] 为过去5个交易日
	var sum int64
	for i := 1; i <= 5; i++ {
		sum += klines[i].Volume
	}
	avgVol := float64(sum) / 5
	if avgVol <= 0 {
		return 0
	}

	return latestVol / avgVol
}

type volumeRatioSignal struct {
	indicator.BaseSignal
}

func (s *volumeRatioSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "量比", "%.2f", "%.1f", sId, config)
}
