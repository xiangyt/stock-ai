package market

import (
	"fmt"

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

	// 自定义信号1：单日成交量比较（原有功能）
	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义成交量筛选条件",
		indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 5.0}})

	// 自定义信号2/3：连续放量/缩量（共用 volumeTrendSignal）
	cs2 := newVolumeTrendSignal("02", "连续放量",
		"时间窗口内每天成交量相对前一日持续放大。支持时间窗口配置。",
		trendUp)
	cs3 := newVolumeTrendSignal("03", "连续缩量",
		"时间窗口内每天成交量相对前一日持续缩小。支持时间窗口配置。",
		trendDown)

	v.SetCustomSignals([]indicator.Signal{
		&volumeSignal{BaseSignal: cs1},
		cs2,
		cs3,
	})
	return v
}

// Evaluate 评估股票是否满足成交量条件。
//
// 根据信号类型分发到对应的评估逻辑：
// - volumeSignal → 单日成交量比较
// - volumeTrendSignal → 时间窗口内的成交趋势检查
func (v *Volume) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil || len(klines) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("daily_kline").Error()}
	}

	for _, cfg := range configs {
		s, ok := v.Signal[cfg.SignalID]
		if !ok {
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
		}

		switch ss := s.(type) {
		case *volumeSignal:
			value := v.getValue(klines[0])
			res := ss.Evaluate(value, cfg)
			if res.Result == indicator.ResultPassed {
				continue
			}
			return res
		case *volumeTrendSignal:
			res := ss.EvaluateWithWindow(klines, cfg)
			if res.Result == indicator.ResultPassed {
				continue
			}
			return res
		default:
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
		}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// getValue 计算成交量（单位：万手）。
//
// kline 为单条日线数据，股 → 万手 (1万手 = 100万股)。
func (v *Volume) getValue(kline *model.DailyKline) float64 {
	if kline == nil || kline.Volume == 0 {
		return 0
	}
	// 股 → 万手 (1万手 = 100万股 = 1e6股)
	return float64(kline.Volume) / 1e6
}

// ============================================================================
//  volumeSignal — 单日成交量信号
// ============================================================================

type volumeSignal struct {
	indicator.BaseSignal
}

func (s *volumeSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	cfg := config
	if !config.IsCustom() {
		cfg = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "成交量", "%.2f万手", "%.0f万手", sId, cfg)
}

// ============================================================================
//  volumeTrendSignal — 连续放量/缩量信号（共用 struct，支持时间窗口）
// ============================================================================

// trend 类型：标记是连续放量还是连续缩量。
type trendType string

const (
	trendUp   trendType = "up"   // 连续放量：当日/前日 > threshold
	trendDown trendType = "down" // 连续缩量：当日/前日 < threshold
)

// volumeTrendSignal 连续放量/缩量信号，支持时间窗口。
//
// 计算当日成交量 / 前一日成交量的比值，与倍数阈值比较。
// trend 字段决定比较方向。
type volumeTrendSignal struct {
	indicator.BaseSignal
	trend trendType // 放量(up) 或 缩量(down)
}

// newVolumeTrendSignal 创建成交量趋势信号工厂。
func newVolumeTrendSignal(seq, name, desc string, trend trendType) *volumeTrendSignal {
	baseOps := []indicator.OperatorOption{{
		Operator: indicator.OpCustom,
		Label:    trend.Label(),
		Params: []indicator.ParamDef{
			signalutil.ParamLookbackStart(2, "天前"),
			signalutil.ParamLookbackEnd(0, "天前"),
			signalutil.ParamNumber(indicator.ParamKeyThreshold, "倍数", 1.0, ""),
		},
	}}

	baseSig := indicator.NewBaseSignal(seq, name, desc,
		indicator.ValNumber, baseOps,
		&indicator.SignalConfig{
			Operator: indicator.OpCustom,
			Params: map[string]any{
				indicator.ParamKeyThreshold:     1.0,
				indicator.ParamKeyLookbackStart: float64(2),
				indicator.ParamKeyLookbackEnd:   float64(0),
			},
		})

	return &volumeTrendSignal{BaseSignal: baseSig, trend: trend}
}

// trend.Label 返回趋势中文描述。
func (t trendType) Label() string {
	switch t {
	case trendUp:
		return "连续放量"
	case trendDown:
		return "连续缩量"
	default:
		return "趋势"
	}
}

// Trend 返回趋势类型（供测试和外部使用）。
func (s *volumeTrendSignal) Trend() trendType {
	return s.trend
}

// EvaluateWithWindow 基于K线数据评估时间窗口内的成交趋势。
//
// 计算每日成交量 / 前一日成交量的比值：
//   - 放量：比值 > threshold（倍数）
//   - 缩量：比值 < threshold（倍数）
//
// K 线为 latest-first，窗口内每天都要满足条件才算通过。
func (s *volumeTrendSignal) EvaluateWithWindow(klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	if start < end {
		start, end = end, start
	}

	threshold := config.GetFloat64(indicator.ParamKeyThreshold, 1.0)
	windowSize := start - end + 1

	// 最远一天需要和前一日比较，需要 start+2 条
	if start+1 >= len(klines) {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("K线数据不足：需要至少 %d 条，实际 %d 条", start+2, len(klines)),
		}
	}

	trendDesc := s.trend.Label()

	for dayOffset := end; dayOffset <= start; dayOffset++ {
		todayVol := float64(klines[dayOffset].Volume)
		prevVol := float64(klines[dayOffset+1].Volume)
		if prevVol <= 0 {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("第%d天前成交量%.2f万手，无法计算比值（前一日为0）", dayOffset, todayVol/1e6),
			}
		}

		ratio := todayVol / prevVol

		var ok bool
		switch s.trend {
		case trendUp:
			ok = ratio > threshold
		case trendDown:
			ok = ratio < threshold
		}

		if !ok {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("第%d天前成交倍比%.2fx不满足%s条件（窗口[%d-%d]天前）", dayOffset, ratio, trendDesc, end, start),
			}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultPassed,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("近%d天（%d-%d天前）均满足%s条件", windowSize, end, start, trendDesc),
	}
}
