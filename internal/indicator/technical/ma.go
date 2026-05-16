package technical

import (
	"fmt"
	"strings"

	"stock-ai/internal/indicator"
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
//    近 days 日（默认 4 日）均满足 MA5 > MA10 > MA20 > MA60
//    忽略最近 offset_days 天（0=从今天开始）
// ============================================================================

const (
	paramBullishDays       = "days"        // 连续满足天数，默认 4
	paramBullishOffsetDays = "offset_days" // 偏移天数，默认 0（从今天开始）
)

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
					Operator: indicator.OpRising,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						{Key: paramBullishOffsetDays, Label: "忽略近X天", Type: "number", Required: false, Default: 0, Min: 0, Max: 9, Unit: ""},
						{Key: paramBullishDays, Label: "连续N天", Type: "number", Required: false, Default: 4, Min: 1, Max: 10, Unit: ""},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpRising,
				Params: map[string]any{
					paramBullishOffsetDays: float64(0),
					paramBullishDays:       float64(4),
				},
			},
		),
	}
}

// checkBullish 判断从 offset 天起，近 days 日是否同时满足：
//  1. 均线顺序正确: MA5 > MA10 > MA20 > MA60（每日都检查）
//  2. 均线自身上升: MA5[0]>MA5[1]>MA5[2]>MA5[3]（MA10/MA20 同理，MA60 不检查）
func checkBullish(lines MALines, offset, days int) (bool, int, string) {
	// 先检查均线自身是否上升（MA5/MA10/MA20，从 offset 起近 days 日）
	for i := offset; i < offset+days-1; i++ {
		if lines.MA5[i] <= lines.MA5[i+1] {
			return false, i - offset, fmt.Sprintf("MA5第%d日(%.2f) ≤ 第%d日(%.2f)", i-offset+1, lines.MA5[i], i-offset+2, lines.MA5[i+1])
		}
		if lines.MA10[i] <= lines.MA10[i+1] {
			return false, i - offset, fmt.Sprintf("MA10第%d日(%.2f) ≤ 第%d日(%.2f)", i-offset+1, lines.MA10[i], i-offset+2, lines.MA10[i+1])
		}
		if lines.MA20[i] <= lines.MA20[i+1] {
			return false, i - offset, fmt.Sprintf("MA20第%d日(%.2f) ≤ 第%d日(%.2f)", i-offset+1, lines.MA20[i], i-offset+2, lines.MA20[i+1])
		}
	}
	// 再检查每日均线顺序是否正确
	for i := offset; i < offset+days; i++ {
		v5, v10, v20, v60 := lines.MA5[i], lines.MA10[i], lines.MA20[i], lines.MA60[i]
		if !(v5 > v10 && v10 > v20 && v20 > v60) {
			return false, i - offset, fmt.Sprintf("MA5(%.2f) MA10(%.2f) MA20(%.2f) MA60(%.2f)", v5, v10, v20, v60)
		}
	}
	return true, 0, ""
}

func (s *SignalMaBullish) Evaluate(lines MALines, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	offset := int(config.GetFloat64(paramBullishOffsetDays, 0))
	days := int(config.GetFloat64(paramBullishDays, 4))
	if offset+days > maSeriesDays {
		days = maSeriesDays - offset
	}

	if ok, idx, msg := checkBullish(lines, offset, days); !ok {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("第 %d 日: %s", idx+1, msg),
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMaSticky — 均线粘合
//
//  判定规则:
//    MA5/MA10/MA20 三条均线两两之间的最大偏离度 < threshold%（默认 2%）
//    要求近 N 日（默认 3 日）均满足粘合条件，避免单日噪点。
//    忽略最近 offset_days 天（0=从今天开始）
// ============================================================================

const (
	paramStickyThreshold  = "threshold"   // 最大允许偏离百分比，默认 2
	paramStickyDays       = "days"        // 连续粘合天数，默认 3
	paramStickyOffsetDays = "offset_days" // 偏移天数，默认 0（从今天开始）
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
					Operator: indicator.OpRising,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						{Key: paramStickyOffsetDays, Label: "忽略近X天", Type: "number", Required: false, Default: 0, Min: 0, Max: 9, Unit: ""},
						{Key: paramStickyThreshold, Label: "最大偏离", Type: "number", Required: false, Default: 2, Min: 0.1, Max: 10, Step: 0.1, Unit: "%"},
						{Key: paramStickyDays, Label: "连续X天", Type: "number", Required: false, Default: 3, Min: 1, Max: 10, Unit: ""},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpRising,
				Params: map[string]any{
					paramStickyOffsetDays: float64(0),
					paramStickyThreshold:  2.0,
					paramStickyDays:       float64(3),
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
	offset := int(config.GetFloat64(paramStickyOffsetDays, 0))
	threshold := config.GetFloat64(paramStickyThreshold, 2.0)
	days := int(config.GetFloat64(paramStickyDays, 3))

	if offset+days > maSeriesDays {
		days = maSeriesDays - offset
	}

	for i := offset; i < offset+days; i++ {
		dev := maxDeviation(lines.MA5[i], lines.MA10[i], lines.MA20[i])
		if dev >= threshold {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("第 %d 日粘合偏离 %.2f%% ≥ %.2f%%", i-offset+1, dev, threshold),
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalPriceAboveMA5 — 股价站上5日线
//
//  判定规则:
//    连续 days 日（默认 1 日）收盘价 > MA5
//    忽略最近 offset_days 天（0=从今天开始）
// ============================================================================

const (
	paramPriceAboveDays       = "days"        // 连续站上MA5天数，默认 1
	paramPriceAboveOffsetDays = "offset_days" // 忽略天数，默认 0（从今天开始）
)

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
						{Key: paramPriceAboveOffsetDays, Label: "忽略近X天", Type: "number", Required: false, Default: 0, Min: 0, Max: 9, Unit: ""},
						{Key: paramPriceAboveDays, Label: "连续N天", Type: "number", Required: false, Default: 1, Min: 1, Max: 10, Unit: ""},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCrossAbove,
				Params: map[string]any{
					paramPriceAboveOffsetDays: float64(0),
					paramPriceAboveDays:       float64(1),
				},
			},
		),
	}
}

func (s *SignalPriceAboveMA5) Evaluate(lines MALines, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	offset := int(config.GetFloat64(paramPriceAboveOffsetDays, 0))
	days := int(config.GetFloat64(paramPriceAboveDays, 1))

	if offset+days > maSeriesDays {
		days = maSeriesDays - offset
	}

	for i := offset; i < offset+days; i++ {
		price := float64(klines[i].Close)
		ma5 := lines.MA5[i]
		if price <= ma5 {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("第%d日收盘价(%.2f)未站上MA5(%.2f)", i+1, price/100, ma5/100),
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMA5CrossMA10 — 5日线金叉10日线
//
//  判定规则:
//    前一日：MA5 <= MA10
//    当日：    MA5 >  MA10
//    可选：两线均向上（MA5今日 > MA5昨日 且 MA10今日 > MA10昨日）
//    忽略最近 offset_days 天（0=从今天开始）
// ============================================================================

const (
	paramCrossBothRising = "both_rising"
	paramCrossOffsetDays = "offset_days" // 偏移天数，默认 0（从今天开始）
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
					{Key: paramCrossOffsetDays, Label: "忽略近X天", Type: "number", Required: false, Default: 0, Min: 0, Max: 8, Unit: ""},
					ParamSelect(paramCrossBothRising, "两线均向上", false, 0, "否", "是"),
				},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCrossAbove,
				Params: map[string]any{
					paramCrossOffsetDays: float64(0),
					paramCrossBothRising: float64(0),
				},
			},
		),
	}
}

func (s *SignalMA5CrossMA10) Evaluate(lines MALines, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	offset := int(config.GetFloat64(paramCrossOffsetDays, 0))

	ma5Today, ma10Today := lines.MA5[offset], lines.MA10[offset]
	ma5Prev, ma10Prev := lines.MA5[offset+1], lines.MA10[offset+1]

	// 金叉：前一日 MA5 <= MA10，当日 MA5 > MA10
	if ma5Prev > ma10Prev {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("第%d日MA5(%.2f)已大于MA10(%.2f)，非金叉", offset+2, ma5Prev, ma10Prev),
		}
	}
	if ma5Today <= ma10Today {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("第%d日MA5(%.2f)未大于MA10(%.2f)", offset+1, ma5Today, ma10Today),
		}
	}

	bothRising := config.GetFloat64(paramCrossBothRising, 0) != 0
	if bothRising {
		if ma5Today <= ma5Prev {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  "MA5今日未高于昨日，均线未向上",
			}
		}
		if ma10Today <= ma10Prev {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  "MA10今日未高于昨日，均线未向上",
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}
