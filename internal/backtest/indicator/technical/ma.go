package technical

import (
	"fmt"
	"math"
	"strings"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/signalutil"
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
	MA30 []float64
	MA60 []float64
}

// maSeriesDays 预计算的均线序列长度（近 N 日）
const maSeriesDays = 30

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

	// 内置信号：非交叉信号 + 交叉信号配置表循环生成
	bareSigs := []indicator.Signal{
		NewSignalMaBullish("01"),   // 多头排列 (MA5/10/20/30/60)
		NewSignalMaBullish2("02"),  // 多头排列2 (MA10/20/30/60)
		NewSignalMaBullish3("03"),  // 多头排列3 (MA10/20/30)
		// 04~05 预留
		NewSignalMaSticky("06"),        // 均线粘合
		NewSignalPriceAboveMA5("10"),   // 站上5日线
		// 07~09, 11~14 预留
	}
	builtInSigs := append(bareSigs, buildMaCrossSignals(maCrossDefs)...)
	i.SetBuiltInSignals(builtInSigs)

	// 自定义信号：非交叉信号 + 交叉信号配置表循环生成
	customSigs := append(bareSigs, buildMaCrossSignals(customMaCrossDefs)...)
	i.SetCustomSignals(customSigs)
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
			case *SignalMaBullish2:
				if res := vv.Evaluate(lines, klines, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalMaBullish3:
				if res := vv.Evaluate(lines, v); res.Result == indicator.ResultPassed {
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
			case *SignalMaCross:
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
// MA 以高精度 float64 存储，比较时通过 floorMA() 向下取整。
func buildMALines(klines []*model.DailyKline) MALines {
	lines := MALines{
		MA5:  make([]float64, maSeriesDays),
		MA10: make([]float64, maSeriesDays),
		MA20: make([]float64, maSeriesDays),
		MA30: make([]float64, maSeriesDays),
		MA60: make([]float64, maSeriesDays),
	}
	for day := 0; day < maSeriesDays; day++ {
		lines.MA5[day] = calcSMA(klines[day:], 5)
		lines.MA10[day] = calcSMA(klines[day:], 10)
		lines.MA20[day] = calcSMA(klines[day:], 20)
		lines.MA30[day] = calcSMA(klines[day:], 30)
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

// floorMA 将 MA 值向下取整为整数分，用于比较时与人看到的元级精度一致。
// 注意：仅用于比较，MA 本身以高精度 float64 计算和存储。
func floorMA(v float64) float64 {
	return math.Floor(v)
}

// ============================================================================
//  SignalMaBullish — 多头排列
//
//  判定规则:
//    在 [lookback_start, lookback_end] 窗口内，每日均满足：
//      1. 均线顺序：MA5 >= MA10 >= MA20 >= MA30 >= MA60
//      2. 均线上升：MA5/MA10/MA20/MA30 逐日不降（需至少2天，单日跳过此项）
//    默认窗口 lookback_start=2, lookback_end=0，即最近 3 天每天都满足
//
//  语义：窗口内所有日均需满足（AND 逻辑），任一日不满足即拒绝
//  索引方向：newest-first，MA5[0]=最新一日，MA5[N]=N天前
// ============================================================================

type SignalMaBullish struct {
	indicator.BaseSignal
}

func NewSignalMaBullish(seq string) *SignalMaBullish {
	return &SignalMaBullish{
		BaseSignal: indicator.NewBaseSignal(
			seq,
			"多头排列",
			"窗口内每日MA5>=MA10>=MA20>=MA30>=MA60且均线逐日不降",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(2, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(2),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalMaBullish) Evaluate(lines MALines, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 2))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	if start < end {
		start, end = end, start // 容错：保证 start >= end（start=更早，end=更新）
	}
	if start >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("参数越界：回看起点 %d 超出 MA 计算范围（最大 %d）", start, maSeriesDays-1),
		}
	}

	// 1. 每一天检查均线顺序（比较时 floor 以对齐元级精度）
	for i := end; i <= start; i++ {
		v5, v10, v20, v30, v60 := lines.MA5[i], lines.MA10[i], lines.MA20[i], lines.MA30[i], lines.MA60[i]
		f5, f10, f20, f30, f60 := floorMA(v5), floorMA(v10), floorMA(v20), floorMA(v30), floorMA(v60)
		if !(f5 >= f10 && f10 >= f20 && f20 >= f30 && f30 >= f60) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前均线顺序不满足: MA5=%.0f MA10=%.0f MA20=%.0f MA30=%.0f MA60=%.0f", i, f5, f10, f20, f30, f60),
			}
		}
	}

	// 2. 窗口内均线逐日不降（比较时 floor 以对齐元级精度）
	for i := end; i < start; i++ {
		if floorMA(lines.MA5[i]) < floorMA(lines.MA5[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA5(%.2f) < %d天前MA5(%.2f)，下降", i, lines.MA5[i], i+1, lines.MA5[i+1]),
			}
		}
		if floorMA(lines.MA10[i]) < floorMA(lines.MA10[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA10(%.2f) < %d天前MA10(%.2f)，下降", i, lines.MA10[i], i+1, lines.MA10[i+1]),
			}
		}
		if floorMA(lines.MA20[i]) < floorMA(lines.MA20[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA20(%.2f) < %d天前MA20(%.2f)，下降", i, lines.MA20[i], i+1, lines.MA20[i+1]),
			}
		}
		if floorMA(lines.MA30[i]) < floorMA(lines.MA30[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA30(%.2f) < %d天前MA30(%.2f)，下降", i, lines.MA30[i], i+1, lines.MA30[i+1]),
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMaBullish2 — 多头排列2（不关注 MA5，只比较 MA10/20/30/60）
//
//  与多头排列的区别：
//    - 忽略 MA5，仅检查 MA10 >= MA20 >= MA30 >= MA60
//    - 默认窗口更宽：lookback_start=20, lookback_end=0
// ============================================================================

type SignalMaBullish2 struct {
	indicator.BaseSignal
}

func NewSignalMaBullish2(seq string) *SignalMaBullish2 {
	return &SignalMaBullish2{
		BaseSignal: indicator.NewBaseSignal(
			seq,
			"多头排列2",
			"窗口内每日MA10>=MA20>=MA30>=MA60且均线逐日不降",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(20, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(20),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalMaBullish2) Evaluate(lines MALines, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 20))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	if start < end {
		start, end = end, start
	}
	if start >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("参数越界：回看起点 %d 超出 MA 计算范围（最大 %d）", start, maSeriesDays-1),
		}
	}

	// 1. 每一天检查均线顺序：MA10 >= MA20 >= MA30 >= MA60（floor 以对齐元级精度）
	for i := end; i <= start; i++ {
		v10, v20, v30, v60 := lines.MA10[i], lines.MA20[i], lines.MA30[i], lines.MA60[i]
		f10, f20, f30, f60 := floorMA(v10), floorMA(v20), floorMA(v30), floorMA(v60)
		if !(f10 >= f20 && f20 >= f30 && f30 >= f60) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前均线顺序不满足: MA10=%.0f MA20=%.0f MA30=%.0f MA60=%.0f", i, f10, f20, f30, f60),
			}
		}
	}

	// 2. 窗口内均线逐日不降：MA10/MA20/MA30
	for i := end; i < start; i++ {
		if floorMA(lines.MA10[i]) < floorMA(lines.MA10[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA10(%.2f) < %d天前MA10(%.2f)，下降", i, lines.MA10[i], i+1, lines.MA10[i+1]),
			}
		}
		if floorMA(lines.MA20[i]) < floorMA(lines.MA20[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA20(%.2f) < %d天前MA20(%.2f)，下降", i, lines.MA20[i], i+1, lines.MA20[i+1]),
			}
		}
		if floorMA(lines.MA30[i]) < floorMA(lines.MA30[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA30(%.2f) < %d天前MA30(%.2f)，下降", i, lines.MA30[i], i+1, lines.MA30[i+1]),
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMaBullish3 — 多头排列3（仅比较 MA10/20/30）
//
//  与多头排列2的区别：
//    - 不关心 MA60，仅检查 MA10 >= MA20 >= MA30
//    - 适用于中短线趋势判断
// ============================================================================

type SignalMaBullish3 struct {
	indicator.BaseSignal
}

func NewSignalMaBullish3(seq string) *SignalMaBullish3 {
	return &SignalMaBullish3{
		BaseSignal: indicator.NewBaseSignal(
			seq,
			"多头排列3",
			"窗口内每日MA10>=MA20>=MA30且均线逐日不降",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(20, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(20),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalMaBullish3) Evaluate(lines MALines, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 20))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	if start < end {
		start, end = end, start
	}
	if start >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("参数越界：回看起点 %d 超出 MA 计算范围（最大 %d）", start, maSeriesDays-1),
		}
	}

	// 每一天检查均线顺序：MA10 >= MA20 >= MA30
	for i := end; i <= start; i++ {
		f10 := floorMA(lines.MA10[i])
		f20 := floorMA(lines.MA20[i])
		f30 := floorMA(lines.MA30[i])
		if !(f10 >= f20 && f20 >= f30) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前均线顺序不满足: MA10=%.0f MA20=%.0f MA30=%.0f", i, f10, f20, f30),
			}
		}
	}

	// 窗口内均线逐日不降：MA10/MA20/MA30
	for i := end; i < start; i++ {
		if floorMA(lines.MA10[i]) < floorMA(lines.MA10[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA10(%.2f) < %d天前MA10(%.2f)，下降", i, lines.MA10[i], i+1, lines.MA10[i+1]),
			}
		}
		if floorMA(lines.MA20[i]) < floorMA(lines.MA20[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA20(%.2f) < %d天前MA20(%.2f)，下降", i, lines.MA20[i], i+1, lines.MA20[i+1]),
			}
		}
		if floorMA(lines.MA30[i]) < floorMA(lines.MA30[i+1]) {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前MA30(%.2f) < %d天前MA30(%.2f)，下降", i, lines.MA30[i], i+1, lines.MA30[i+1]),
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMaSticky — 均线粘合
//
//  判定规则:
//    在 [lookback_start, lookback_end] 窗口内，每日 MA5/MA10/MA20 三条均线
//    两两之间的最大偏离度均 < threshold%（默认 2%）
//    默认 start=2, end=0，即检查今天~2天前共 3 天
//
//  语义：窗口内所有日均需满足（AND 逻辑），任一日不满足即拒绝
// ============================================================================

const (
	paramStickyThreshold = "threshold" // 最大允许偏离百分比，默认 2
)

type SignalMaSticky struct {
	indicator.BaseSignal
}

func NewSignalMaSticky(seq string) *SignalMaSticky {
	return &SignalMaSticky{
		BaseSignal: indicator.NewBaseSignal(
			seq,
			"均线粘合",
			"MA5/MA10/MA20三线最大偏离度<threshold%",
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
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 2))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	threshold := config.GetFloat64(paramStickyThreshold, 2.0)

	if start < end {
		start, end = end, start // 容错
	}
	if start >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("参数越界：回看起点 %d 超出 MA 计算范围（最大 %d）", start, maSeriesDays-1),
		}
	}

	for i := end; i <= start; i++ {
		dev := maxDeviation(floorMA(lines.MA5[i]), floorMA(lines.MA10[i]), floorMA(lines.MA20[i]))
		if dev >= threshold {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前粘合偏离 %.2f%% ≥ %.2f%%", i, dev, threshold),
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
//
//  语义：窗口内所有日均需满足（AND 逻辑），任一日不满足即拒绝
// ============================================================================

type SignalPriceAboveMA5 struct {
	indicator.BaseSignal
}

func NewSignalPriceAboveMA5(seq string) *SignalPriceAboveMA5 {
	return &SignalPriceAboveMA5{
		BaseSignal: indicator.NewBaseSignal(
			seq,
			"股价站上5日线",
			"窗口内每日收盘价>MA5",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(0, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(0),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalPriceAboveMA5) Evaluate(lines MALines, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	if start < end {
		start, end = end, start
	}
	if start >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("参数越界：回看起点 %d 超出 MA 计算范围（最大 %d）", start, maSeriesDays-1),
		}
	}

	for i := end; i <= start; i++ {
		price := float64(klines[i].Close)
		ma5 := floorMA(lines.MA5[i])
		if price <= ma5 {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("%d天前收盘价(%.2f)未站上MA5(%.2f)", i, price/100, ma5/100),
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}

// ============================================================================
//  SignalMaCross — 均线交叉（金叉/死叉共用）
//
//  金叉(上穿): 前一日 fast < slow, 当日 fast >= slow
//  死叉(下穿): 前一日 fast > slow, 当日 fast <= slow
//  可选：两线均向上（fast今日>fast昨日 且 slow今日>slow昨日）
//
//  语义：窗口内任意日满足即通过（OR 逻辑，滑动搜索）
//  索引方向：newest-first，MA5[0]=最新一日
// ============================================================================

const (
	paramCrossBothRising = "both_rising"
)

// maLineSelector 标识均线类型，用于交叉信号的快慢线选择
type maLineSelector int

const (
	maLine5  maLineSelector = iota // MA5
	maLine10                       // MA10
	maLine20                       // MA20
)

func (s maLineSelector) label() string {
	switch s {
	case maLine5:
		return "MA5"
	case maLine10:
		return "MA10"
	default:
		return "MA20"
	}
}

func (s maLineSelector) get(lines MALines, i int) float64 {
	switch s {
	case maLine5:
		return lines.MA5[i]
	case maLine10:
		return lines.MA10[i]
	default:
		return lines.MA20[i]
	}
}

type SignalMaCross struct {
	indicator.BaseSignal
	Fast     maLineSelector // 快线
	Slow     maLineSelector // 慢线
	IsGolden bool           // true=金叉(上穿), false=死叉(下穿)
}

// maCrossDef 均线交叉信号配置
type maCrossDef struct {
	Seq      string          // 信号序号
	Name     string          // 信号名称
	Desc     string          // 信号描述
	Fast     maLineSelector  // 快线
	Slow     maLineSelector  // 慢线
	IsGolden bool            // true=金叉, false=死叉
}

// maCrossDefs 内置均线交叉信号配置表
var maCrossDefs = []maCrossDef{
	{"15", "5日线金叉10日线", "前一日MA5 < MA10，当日MA5 >= MA10", maLine5, maLine10, true},
	{"16", "10日线金叉20日线", "前一日MA10 < MA20，当日MA10 >= MA20", maLine10, maLine20, true},
	{"17", "5日线死叉10日线", "前一日MA5 > MA10，当日MA5 <= MA10", maLine5, maLine10, false},
	{"18", "10日线死叉20日线", "前一日MA10 > MA20，当日MA10 <= MA20", maLine10, maLine20, false},
}

// customMaCrossDefs 自定义均线交叉信号配置表（通用金叉/死叉，快慢线可配置）
var customMaCrossDefs = []maCrossDef{
	{"15", "均线金叉", "前一日快线 < 慢线，当日快线 >= 慢线", maLine5, maLine10, true},
	{"16", "均线死叉", "前一日快线 > 慢线，当日快线 <= 慢线", maLine5, maLine10, false},
}

// buildMaCrossSignals 根据配置表循环生成均线交叉信号。
func buildMaCrossSignals(defs []maCrossDef) []indicator.Signal {
	sigs := make([]indicator.Signal, len(defs))
	for i, d := range defs {
		sigs[i] = newSignalMaCross(d.Seq, d.Name, d.Desc, d.Fast, d.Slow, d.IsGolden)
	}
	return sigs
}
// newSignalMaCross 创建均线交叉信号的工厂函数。
func newSignalMaCross(id, name, desc string, fast, slow maLineSelector, isGolden bool) *SignalMaCross {
	label := "金叉"
	if !isGolden {
		label = "死叉"
	}
	return &SignalMaCross{
		BaseSignal: indicator.NewBaseSignal(
			id, name, desc,
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    label,
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(0, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						ParamSelect(paramCrossBothRising, "两线均向上", false, 0, "否", "是"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(0),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramCrossBothRising:            float64(0),
				},
			},
		),
		Fast:     fast,
		Slow:     slow,
		IsGolden: isGolden,
	}
}

func NewSignalMA5CrossMA10() *SignalMaCross {
	return newSignalMaCross("04", "5日线金叉10日线", "前一日MA5 < MA10，当日MA5 >= MA10", maLine5, maLine10, true)
}

func NewSignalMA10CrossMA20() *SignalMaCross {
	return newSignalMaCross("05", "10日线金叉20日线", "前一日MA10 < MA20，当日MA10 >= MA20", maLine10, maLine20, true)
}

func NewSignalMA5DeathCrossMA10() *SignalMaCross {
	return newSignalMaCross("06", "5日线死叉10日线", "前一日MA5 > MA10，当日MA5 <= MA10", maLine5, maLine10, false)
}

func NewSignalMA10DeathCrossMA20() *SignalMaCross {
	return newSignalMaCross("07", "10日线死叉20日线", "前一日MA10 > MA20，当日MA10 <= MA20", maLine10, maLine20, false)
}

// NewSignalMaCrossGold 创建通用均线金叉信号（自定义用：不限定具体快慢线，用户可配置参数）。
func NewSignalMaCrossGold() *SignalMaCross {
	return newSignalMaCross("08", "均线金叉", "前一日快线 < 慢线，当日快线 >= 慢线", maLine5, maLine10, true)
}

// NewSignalMaCrossDeath 创建通用均线死叉信号（自定义用：不限定具体快慢线，用户可配置参数）。
func NewSignalMaCrossDeath() *SignalMaCross {
	return newSignalMaCross("09", "均线死叉", "前一日快线 > 慢线，当日快线 <= 慢线", maLine5, maLine10, false)
}

func (s *SignalMaCross) Evaluate(lines MALines, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	if start < end {
		start, end = end, start // 容错
	}
	// 需要前一日数据，检查边界
	if end+1 >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("参数越界：回看终点 %d 太靠近今天，无前一日数据", end),
		}
	}
	if start >= maSeriesDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("参数越界：回看起点 %d 超出数据范围", start),
		}
	}

	bothRising := config.GetFloat64(paramCrossBothRising, 0) != 0
	name := fmt.Sprintf("%s%s%s", s.Fast.label(), signalCrossOp(s.IsGolden), s.Slow.label())

	for i := end; i <= start; i++ {
		fastToday, slowToday := s.Fast.get(lines, i), s.Slow.get(lines, i)
		fastPrev, slowPrev := s.Fast.get(lines, i+1), s.Slow.get(lines, i+1)

		ft, st := floorMA(fastToday), floorMA(slowToday)
		fp, sp := floorMA(fastPrev), floorMA(slowPrev)

		var crossed bool
		if s.IsGolden {
			// 金叉：前一日 fast < slow，当日 fast >= slow
			crossed = fp < sp && ft >= st
		} else {
			// 死叉：前一日 fast > slow，当日 fast <= slow
			crossed = fp > sp && ft <= st
		}

		if !crossed {
			continue
		}

		if bothRising {
			if ft <= fp || st <= sp {
				continue
			}
		}

		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到%s", start, end, name),
	}
}

func signalCrossOp(isGolden bool) string {
	if isGolden {
		return "金叉"
	}
	return "死叉"
}
