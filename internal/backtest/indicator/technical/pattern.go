package technical

import (
	"fmt"

	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

const (
	paramMVIRatioMin = "ratio_min" // 量比下限，默认 1.2
	paramMVIRatioMax = "ratio_max" // 量比上限，默认 2.5
)

// ============================================================================
//  Pattern — 形态 (序列型)
//  ID: 01006 = CatCodeMarket("01") + IndVolumeSeq("006")
//  数据源: GetDailyKline()
// ============================================================================

type Pattern struct {
	indicator.BaseIndicator
}

func NewPattern() *Pattern {
	i := &Pattern{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndPatternSeq,
			NameStr:     "形态",
			CategoryVal: indicator.CatTechnical,
			Desc:        "各种自定义技术指标",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignal513(),
	})

	i.SetCustomSignals([]indicator.Signal{
		NewSignalMildVolumeIncrease(),
	})
	return i
}

func (i *Pattern) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}

	for _, v := range config {
		if s, ok := i.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *Signal513:
				if res := vv.Evaluate(klines, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalMildVolumeIncrease:
				if res := vv.Evaluate(klines, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// 513战法
type Signal513 struct {
	indicator.BaseSignal
}

func NewSignal513() *Signal513 {
	return &Signal513{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"513战法",
			"5连阳后放量异动阳线，随后缩量横盘不破异动阳线开盘价",
			indicator.ValNumber,
			[]indicator.OperatorOption{},
			nil,
		),
	}
}

// Evaluate ...
func (s *Signal513) Evaluate(klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	var key1, key2 = 5, 2
	if len(klines) < (key1 + key2 + 1) {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
			Message: indicator.DataEmptyError("513 kline").Error()}
	}

	if klines[key2].Volume > klines[key2+1].Volume*2 { // key2天前那天成交量大于2倍4天前成交量
		maxRate := float64((klines[key2].High - klines[key2+1].Close)) / float64(klines[key2+1].Close) * 100
		rate1 := float64((klines[key2].High - klines[key2].Close)) / float64(klines[key2+1].Close) * 100
		rate2 := float64((klines[key2].Close - klines[key2].Open)) / float64(klines[key2+1].Close) * 100
		if !(maxRate > 5 && rate1 > 1 && rate2 > 3) { //异动当天最高至少涨8%，回落超过1%，阳线高度至少3%
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
				Message: "未出现异动阳线"}
		}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
			Message: "未出现异动阳线"}
	}
	for i := key2; i < key2+key1; i++ {
		if !(klines[i].Close >= klines[i].Open && klines[i].Close >= klines[i+1].Close) {
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: "至少5连阳"}
		}
	}
	for i := 0; i < key2; i++ {
		if klines[i].Open < klines[key2].Open {
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
				Message: "股价（收盘价）不能跌破那根异动阳线的开盘价"}
		} else if klines[i].Volume > klines[key2].Volume {
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
				Message: "成交量必须明显萎缩，不能超过异动阳线当天的量"}
		} else {
			rate := float64((klines[i].High - klines[i].Low)) / float64(klines[i+1].Close) * 100
			if rate >= 5 {
				return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
					Message: "盘面不能剧烈波动"}
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMildVolumeIncrease — 温和放量（自定义信号）
//
//  klines[0]=最新，klines[start]=窗口最早，klines[end]=窗口最晚（最新）。
//
//  判定规则：
//    1. 窗口内每根 K 线量比位于 [ratio_min, ratio_max]
//    2. 每一天成交量 >= 前一天成交量的 90%（窗口最早一日跳过）
//    3. 最新一天成交量 >= 窗口最早一日成交量
//    4. 窗口内每一天都是阳线（Close >= Open）
// ============================================================================

type SignalMildVolumeIncrease struct {
	indicator.BaseSignal
}

func NewSignalMildVolumeIncrease() *SignalMildVolumeIncrease {
	return &SignalMildVolumeIncrease{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"温和放量",
			"时间窗口内量比在区间内且温和递增，均为阳线且最后一天量不缩",
			indicator.ValSeries,
			[]indicator.OperatorOption{{
				Operator: indicator.OpCustom,
				Label:    "参数设置",
				Params: []indicator.ParamDef{
					signalutil.ParamLookbackStart(2, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
					signalutil.ParamNumber(paramMVIRatioMin, "量比下限", 1.2, ""),
					signalutil.ParamNumber(paramMVIRatioMax, "量比上限", 2.5, ""),
				},
			}},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(2),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramMVIRatioMin:                float64(1.2),
					paramMVIRatioMax:                float64(2.5),
				},
			},
		),
	}
}

// Evaluate 温和放量判定。
//
// klines[0]=最新，参数 lookback_start/end 表示"距今日N天前"。
// 窗口范围：klines[end]（最新） 到 klines[start]（最早），end <= start。
func (s *SignalMildVolumeIncrease) Evaluate(klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 2))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	ratioMin := config.GetFloat64(paramMVIRatioMin, 1.2)
	ratioMax := config.GetFloat64(paramMVIRatioMax, 2.5)

	// 确保 start >= end（窗口始于更早的日期）
	if start < end {
		start, end = end, start
	}

	// 需要 start+1 天的 K 线 + 量比分母 5 根
	if len(klines) < start+6 {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", start+6, len(klines)),
		}
	}

	// 遍历窗口内每一天：从 end（最新）到 start（最早）
	for i := end; i <= start; i++ {
		k := klines[i]

		// 4. 必须是阳线
		if k.Close < k.Open {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("距今日%d天 (%d) 非阳线 Open=%d Close=%d", i, k.TradeDate, k.Open, k.Close),
			}
		}

		// 1. 量比检查：当日量 / 近5日均量
		vr := calcVolumeRatio(klines, i)
		if vr < ratioMin || vr > ratioMax {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("距今日%d天 (%d) 量比=%.2f 不在 [%.1f, %.1f]", i, k.TradeDate, vr, ratioMin, ratioMax),
			}
		}

		// 2. 每天成交量 >= 前一日（自然日顺序）的 90%（窗口最早一日 klines[start] 跳过）
		if i < start {
			if klines[i].Volume < klines[i+1].Volume*9/10 {
				return &indicator.EvaluatedStock{
					Result:   indicator.ResultRejected,
					SignalID: config.SignalID,
					Message:  fmt.Sprintf("距今日%d天 (%d) 成交量 %d < 前一日 (%d) %d 的 90%%",
						i, k.TradeDate, k.Volume, klines[i+1].TradeDate, klines[i+1].Volume),
				}
			}
		}
	}

	// 3. 最新一天 >= 窗口最早一日
	if klines[end].Volume < klines[start].Volume {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("最新 (%d，%d天前) 成交量 %d < 窗口最早 (%d，%d天前) 成交量 %d",
				klines[end].TradeDate, end, klines[end].Volume, klines[start].TradeDate, start, klines[start].Volume),
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// calcVolumeRatio 计算 klines[offset] 的量比: 当日量 / 近5日均量。
func calcVolumeRatio(klines []*model.DailyKline, offset int) float64 {
	vol := float64(klines[offset].Volume)
	if vol <= 0 {
		return 0
	}
	var sum int64
	cnt := 0
	for i := offset + 1; i < offset+6 && i < len(klines); i++ {
		if klines[i].Volume > 0 {
			sum += klines[i].Volume
			cnt++
		}
	}
	if cnt == 0 {
		return 0
	}
	return vol / (float64(sum) / float64(cnt))
}
