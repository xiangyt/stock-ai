package technical

import (
	"fmt"
	"strings"

	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ParamSelect 创建单选参数（前端渲染为单选框）
// options 格式: "否", "是"（value 自动为 "0","1"）或 "0:否","1:是"（显式指定 value）
func ParamSelect(key, label string, required bool, defaultVal float64, options ...string) indicator.ParamDef {
	opts := make([]indicator.EnumOption, len(options))
	for i, opt := range options {
		val := fmt.Sprintf("%d", i)
		lbl := opt
		if idx := strings.Index(opt, ":"); idx >= 0 {
			val = opt[:idx]
			lbl = strings.TrimSpace(opt[idx+1:])
		}
		opts[i] = indicator.EnumOption{Value: val, Label: lbl}
	}
	return indicator.ParamDef{
		Key:      key,
		Label:    label,
		Type:     "select",
		Required: required,
		Default:  defaultVal,
		Options:  opts,
	}
}

// ============================================================================
//  Ma — 均线 (序列型)
//  ID: 01001 = CatCodeTechnical("01") + IndMaSeq("001")
//  数据源: GetDailyKline()
//
//  设计要点:
//    在 Ma.Evaluate 层统一计算固定4条均线（MA5/10/20/60）近10日序列，
//    所有信号共享同一份 MALines，不重复计算。
// ============================================================================

// MALines 均线预计算结果，供该指标下所有信号复用
type MALines struct {
	MA5  []float64 // 近 maSeriesDays 日的 MA5 序列，[0]=最新
	MA10 []float64
	MA20 []float64
	MA60 []float64
}

// maSeriesDays 预计算的均线序列长度（近 N 日）
const maSeriesDays = 10

// minKlineLen 计算 MA60 所需的最少 K 线根数
const minKlineLen = 60

type Ma struct {
	indicator.BaseIndicator
}

func NewMa() *Ma {
	i := &Ma{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndMaSeq,
			NameStr:     "均线",
			CategoryVal: indicator.CatTechnical,
			Desc:        "均线多头/空头排列、粘合、金叉等",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalMaBullish(),
		NewSignalMaSticky(),
		NewSignalPriceAboveMA5(),
		NewSignalMA5CrossMA10(),
	})

	i.SetCustomSignals([]indicator.Signal{
		NewSignalMaBullish(),
		NewSignalMaSticky(),
		NewSignalPriceAboveMA5(),
		NewSignalMA5CrossMA10(),
	})
	return i
}

// Evaluate 均线指标评估入口
//
//  1. 固定计算 MA5/10/20/60 近 maSeriesDays 日序列
//  2. 将结果和原始 klines 传给各信号
func (i *Ma) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < minKlineLen+maSeriesDays-1 {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", minKlineLen+maSeriesDays-1, len(klines)),
		}
	}

	lines := buildMALines(klines)

	for _, v := range config {
		if s, ok := i.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *SignalMaBullish:
				if res := vv.Evaluate(lines, klines, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalMaSticky:
				if res := vv.Evaluate(lines, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalPriceAboveMA5:
				if res := vv.Evaluate(lines, klines, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalMA5CrossMA10:
				if res := vv.Evaluate(lines, v); res.Result == indicator.ResultPassed {
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

// buildMALines 计算 MA5/10/20/60 近 maSeriesDays 日序列。
// klines[0] 为最新K线，Close 以"分"为单位。
// 返回的切片 [0] 对应最新一日，[maSeriesDays-1] 对应最早一日。
func buildMALines(klines []*model.DailyKline) MALines {
	lines := MALines{
		MA5:  make([]float64, maSeriesDays),
		MA10: make([]float64, maSeriesDays),
		MA20: make([]float64, maSeriesDays),
		MA60: make([]float64, maSeriesDays),
	}
	for day := 0; day < maSeriesDays; day++ {
		lines.MA5[day] = calcSMA(klines[day:], 5)
		lines.MA10[day] = calcSMA(klines[day:], 10)
		lines.MA20[day] = calcSMA(klines[day:], 20)
		lines.MA60[day] = calcSMA(klines[day:], 60)
	}
	return lines
}

// calcSMA 计算 klines 前 n 根收盘价的简单移动均线。
// klines[0] 为最新，数据不足时返回 0。
func calcSMA(klines []*model.DailyKline, n int) float64 {
	if n <= 0 || len(klines) < n {
		return 0
	}
	var sum int64
	for i := 0; i < n; i++ {
		sum += int64(klines[i].Close)
	}
	return float64(sum) / float64(n)
}

// ============================================================================
//  SignalMaBullish — 多头排列
//
//  判定规则:
//    在 [lookback_start, lookback_end] 窗口内，最近连续 3 日均满足：
//      MA5 > MA10 > MA20 > MA60，且 MA5/MA10/MA20 自身逐日上升
//    默认窗口 lookback_start=3, lookback_end=0，即检查最近 3 天（今天~2天前）
// ============================================================================

type SignalMaBullish struct {
	indicator.BaseSignal
}

func NewSignalMaBullish() *SignalMaBullish {
	return &SignalMaBullish{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"多头排列",
			"多头排列",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(3, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(3),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

// checkBullish 判断在 [start, end] 窗口内，最近连续 3 天（即 [end, end+3)）是否同时满足：
//  1. 均线顺序正确: MA5 > MA10 > MA20 > MA60（每日都检查）
//  2. 均线自身上升: MA5[0]>MA5[1]>MA5[2]>MA5[3]（MA10/MA20 同理，MA60 不检查）
// 索引说明：lines.MA5[0]=最新一日，[maSeriesDays-1]=最早一日
// start/end 均为"N天前"，即 end < start，检查区间为 [end, end+4)
func checkBullish(lines MALines, start, end int) (bool, int, string) {
	const requiredDays = 3
	checkStart := end
	checkEnd := end + requiredDays
	if checkEnd > start+1 {
		return false, 0, "参数越界：需要连续3天，超出信号窗口范围"
	}
	// 先检查均线自身是否上升（MA5/MA10/MA20，在 [checkStart, checkEnd) 内）
	for i := checkStart; i < checkEnd-1; i++ {
		if lines.MA5[i] <= lines.MA5[i+1] {
			return false, i - checkStart, fmt.Sprintf("MA5第%d日(%.2f) ≤ 第%d日(%.2f)", i-checkStart+1, lines.MA5[i], i-checkStart+2, lines.MA5[i+1])
		}
		if lines.MA10[i] <= lines.MA10[i+1] {
			return false, i - checkStart, fmt.Sprintf("MA10第%d日(%.2f) ≤ 第%d日(%.2f)", i-checkStart+1, lines.MA10[i], i-checkStart+2, lines.MA10[i+1])
		}
		if lines.MA20[i] <= lines.MA20[i+1] {
			return false, i - checkStart, fmt.Sprintf("MA20第%d日(%.2f) ≤ 第%d日(%.2f)", i-checkStart+1, lines.MA20[i], i-checkStart+2, lines.MA20[i+1])
		}
	}
	// 再检查每日均线顺序是否正确
	for i := checkStart; i < checkEnd; i++ {
		v5, v10, v20, v60 := lines.MA5[i], lines.MA10[i], lines.MA20[i], lines.MA60[i]
		if !(v5 > v10 && v10 > v20 && v20 > v60) {
			return false, i - checkStart, fmt.Sprintf("MA5(%.2f) MA10(%.2f) MA20(%.2f) MA60(%.2f)", v5, v10, v20, v60)
		}
	}
	return true, 0, ""
}

func (s *SignalMaBullish) Evaluate(lines MALines, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 3))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	if start < end {
		start, end = end, start // 容错：保证 start >= end
	}
	// 默认窗口 [3, 0]，检查最近 [0, 3) 共 3 天；若用户自定义窗口，检查 [end, end+3)
	// 校验窗口大小至少为 3
	if start-end+1 < 3 {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("信号窗口需至少覆盖 3 天，当前仅 %d 天", start-end+1),
		}
	}

	if ok, idx, msg := checkBullish(lines, start, end); !ok {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("第 %d 天: %s", idx+1, msg),
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMaSticky — 均线粘合
//
//  判定规则:
//    MA5/MA10/MA20 三条均线两两之间的最大偏离度 < threshold%（默认 2%）
//    在 [lookback_start, lookback_end] 窗口内，每日均满足粘合条件
//    默认 start=2, end=0，即检查今天~2天前共 3 天
// ============================================================================

const (
	paramStickyThreshold = "threshold" // 最大允许偏离百分比，默认 2
)

type SignalMaSticky struct {
	indicator.BaseSignal
}

func NewSignalMaSticky() *SignalMaSticky {
	return &SignalMaSticky{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"均线粘合",
			"均线粘合",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(2, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						{Key: paramStickyThreshold, Label: "最大偏离", Type: "number", Required: false, Default: 2, Min: 0.1, Max: 10, Step: 0.1, Unit: "%"},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(2),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramStickyThreshold:            2.0,
				},
			},
		),
	}
}

// maxDeviation 计算 MA5/MA10/MA20 两两之间的最大相对偏离百分比（以三者均值为基准）
func maxDeviation(ma5, ma10, ma20 float64) float64 {
	avg := (ma5 + ma10 + ma20) / 3
	if avg == 0 {
		return 100 // 数据不足时视为偏离极大
	}
	d1 := absDiff(ma5, ma10) / avg * 100
	d2 := absDiff(ma10, ma20) / avg * 100
	d3 := absDiff(ma5, ma20) / avg * 100
	return maxFloat64(d1, d2, d3)
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func maxFloat64(vals ...float64) float64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func (s *SignalMaSticky) Evaluate(lines MALines, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 2))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	threshold := config.GetFloat64(paramStickyThreshold, 2.0)

	if start < end {
		start, end = end, start // 容错
	}

	for i := end; i <= start; i++ {
		dev := maxDeviation(lines.MA5[i], lines.MA10[i], lines.MA20[i])
		if dev >= threshold {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("第 %d 日粘合偏离 %.2f%% ≥ %.2f%%", i-end+1, dev, threshold),
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalPriceAboveMA5 — 股价站上5日线
//
//  判定规则:
//    在 [lookback_start, lookback_end] 窗口内，每日收盘价 > MA5
//    默认 start=0, end=0，即检查今天
// ============================================================================

type SignalPriceAboveMA5 struct {
	indicator.BaseSignal
}

func NewSignalPriceAboveMA5() *SignalPriceAboveMA5 {
	return &SignalPriceAboveMA5{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"股价站上5日线",
			"连续N日收盘价站上MA5",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCrossAbove,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(0, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCrossAbove,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(0),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalPriceAboveMA5) Evaluate(lines MALines, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	if start < end {
		start, end = end, start
	}

	for i := end; i <= start; i++ {
		price := float64(klines[i].Close)
		ma5 := lines.MA5[i]
		if price <= ma5 {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("第%d日收盘价(%.2f)未站上MA5(%.2f)", i-end+1, price/100, ma5/100),
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMA5CrossMA10 — 5日线金叉10日线
//
//  判定规则:
//    在 [lookback_start, lookback_end] 窗口内（按 oldest-first 索引），
//    存在某一天 i 满足：
//      前一日：MA5[i+1] <= MA10[i+1]
//      当日：  MA5[i]   >  MA10[i]
//    可选：两线均向上（MA5今日 > MA5昨日 且 MA10今日 > MA10昨日）
// ============================================================================

const (
	paramCrossBothRising = "both_rising"
)

type SignalMA5CrossMA10 struct {
	indicator.BaseSignal
}

func NewSignalMA5CrossMA10() *SignalMA5CrossMA10 {
	return &SignalMA5CrossMA10{
		BaseSignal: indicator.NewBaseSignal(
			"04",
			"5日线金叉10日线",
			"5日线金叉10日线",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCrossAbove,
					Label:    "金叉",
					Params: []indicator.ParamDef{
						{Key: indicator.ParamKeyLookbackStart, Label: "信号起点(天前)", Type: "number", Required: false, Default: 1, Min: 1, Max: 9, Unit: ""},
						{Key: indicator.ParamKeyLookbackEnd, Label: "信号终点(天前)", Type: "number", Required: false, Default: 1, Min: 1, Max: 9, Unit: ""},
						ParamSelect(paramCrossBothRising, "两线均向上", false, 0, "否", "是"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCrossAbove,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(1),
					indicator.ParamKeyLookbackEnd:   float64(1),
					paramCrossBothRising:   float64(0),
				},
			},
		),
	}
}

func (s *SignalMA5CrossMA10) Evaluate(lines MALines, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 1))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 1))

	if start < end {
		start, end = end, start // 容错
	}
	// 检查区间：[end, start+1)，i 为当日索引，i+1 需合法
	if end+1 >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  "参数越界：回看终点太靠近今天，无前一日数据",
		}
	}
	if start >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  "参数越界：回看起点超出数据范围",
		}
	}

	bothRising := config.GetFloat64(paramCrossBothRising, 0) != 0

	for i := end; i <= start; i++ {
		ma5Today, ma10Today := lines.MA5[i], lines.MA10[i]
		ma5Prev, ma10Prev := lines.MA5[i+1], lines.MA10[i+1]

		// 金叉：前一日 MA5 <= MA10，当日 MA5 > MA10
		if ma5Prev > ma10Prev {
			continue
		}
		if ma5Today <= ma10Today {
			continue
		}

		if bothRising {
			if ma5Today <= ma5Prev {
				continue
			}
			if ma10Today <= ma10Prev {
				continue
			}
		}

		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[第%d天前, 第%d天前]窗口内未检测到MA5金叉MA10", start, end),
	}
}
