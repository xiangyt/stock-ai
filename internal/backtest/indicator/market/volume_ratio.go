package market

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  VolumeRatio — 量比 (数值型)
//  ID: 02003 = CatCodeMarket("02") + IndVolumeRatioSeq("003")
//  K 线顺序: 最新在前（DESC），klines[0] = 当日
//  量比(offset=0) = 当日成交量 / 近5日平均成交量（klines[1:6]）
//  量比(offset=1) = 前一日成交量 / klines[2:6] 均值（前1天 vs 前6天~前2天均值）
// ============================================================================

const paramVolumeRatioOffset = "offset_days" // 偏移天数，默认 0

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
			Desc:        "指定偏移日的成交量与过去5日均量的比值",
			UnitStr:     "",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(v.UnitStr, 0)

	indicatorName := v.NameStr
	builtInSigs := signalutil.BuildRangeSignals(volumeRatioDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &volumeRatioSignal{BaseSignal: bs}
	})
	v.SetBuiltInSignals(builtInSigs)

	// 自定义信号：阈值比较操作符 + 偏移天数参数
	customOps := append(numberOps, indicator.OperatorOption{
		Operator: indicator.OpCustom,
		Label:    "参数设置",
		Params:   []indicator.ParamDef{signalutil.ParamNumber(paramVolumeRatioOffset, "偏移天数", 0, "天")},
	})
	cs1 := indicator.NewBaseSignal("01", indicatorName,
		"指定偏移日的成交量与近5日均量的比值。偏移天数可调（默认0=当日）。",
		indicator.ValNumber, customOps,
		&indicator.SignalConfig{
			Operator: indicator.OpGT,
			Params: map[string]any{
				indicator.ParamKeyThreshold: 1.5,
				paramVolumeRatioOffset:      float64(0),
			},
		})
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

	for _, cfg := range configs {
		offset := 0
		if cfg.IsCustom() {
			offset = int(cfg.GetFloat64(paramVolumeRatioOffset, 0))
		}
		value := v.getValue(klines, offset)

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

// getValue 计算量比：偏移 N 天的成交量 / 过去 5 日均量。
// offset=0 时 klines[0] 为基准日，klines[1:N+6] 为分母；offset=1 则 klines[1] 为基准日。
func (v *VolumeRatio) getValue(klines []*model.DailyKline, offset int) float64 {
	need := 6 + offset
	if len(klines) < need {
		return 0
	}

	latestVol := float64(klines[offset].Volume)
	if latestVol <= 0 {
		return 0
	}

	// klines[offset+1 : offset+6] 为过去 5 个交易日
	var sum int64
	for i := offset + 1; i < offset+6; i++ {
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
