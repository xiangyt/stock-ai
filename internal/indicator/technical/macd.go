package technical

import (
	"fmt"

	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  MACD — 指数平滑异同移动平均线 (序列型)
//  ID: 01003 = CatCodeTechnical("01") + IndMacdSeq("003")
//  数据源: GetDailyKline()
//
//  设计要点:
//    在 MACD.Evaluate 层统一计算 DIF/DEA/Histogram 近 macdSeriesDays 日序列，
//    所有信号共享同一份 MACDResult，不重复计算。
//
//  公式:
//    EMA12 = EMA(CLOSE, 12)
//    EMA26 = EMA(CLOSE, 26)
//    DIF   = EMA12 - EMA26
//    DEA   = EMA(DIF, 9)
//    MACD柱= 2 * (DIF - DEA)
// ============================================================================

// MACDResult MACD 预计算结果，供该指标下所有信号复用
type MACDResult struct {
	DIF        []float64 // DIF快线序列 [0]=最新
	DEA        []float64 // DEA慢线序列 [0]=最新
	Histogram []float64 // MACD柱 (2*(DIF-DEA)) [0]=最新
	ClosePrice []float64 // 收盘价序列 [0]=最新（元，用于背离检测）
}

// macdSeriesDays 预计算的 MACD 序列长度（近 N 日）
const macdSeriesDays = 120

// macdMinKlines 计算 MACD 所需的最少 K 线根数
const macdMinKlines = 60

// MACD 默认参数
const (
	macdShortPeriod = 12 // 快线 EMA 周期
	macdLongPeriod  = 26 // 慢线 EMA 周期
	macdSignalPeriod = 9 // DEA 信号线周期
)

type Macd struct {
	indicator.BaseIndicator
}

func NewMacd() *Macd {
	i := &Macd{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndMacdSeq,
			NameStr:     "MACD",
			CategoryVal: indicator.CatTechnical,
			Desc:        "指数平滑异同移动平均线（金叉/死叉/背离等）",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalMacdGoldenAboveWater(), // 01 水上金叉
		NewSignalMacdGoldenBelowWater(), // 02 水下金叉
		NewSignalMacdTopDivergence(),    // 03 顶背离
		NewSignalMacdBottomDivergence(), // 04 底背离
		NewSignalMacdDeathCross(),       // 05 死叉
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate MACD 指标评估入口
//
// 1. 固定计算 DIF/DEA/Histogram 近 macdSeriesDays 日序列
// 2. 将结果和原始 klines 传给各信号分发处理
func (i *Macd) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < macdMinKlines+macdSeriesDays-1 {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: configs[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", macdMinKlines+macdSeriesDays-1, len(klines)),
		}
	}

	result := buildMACD(klines)

	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch v := s.(type) {
			case *SignalMacdGoldenAboveWater:
				res = v.Evaluate(result, klines, cfg)
			case *SignalMacdGoldenBelowWater:
				res = v.Evaluate(result, klines, cfg)
			case *SignalMacdTopDivergence:
				res = v.Evaluate(result, klines, cfg)
			case *SignalMacdBottomDivergence:
				res = v.Evaluate(result, klines, cfg)
			case *SignalMacdDeathCross:
				res = v.Evaluate(result, klines, cfg)
			default:
				return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID,
					Message: indicator.ErrUnsupportedSignal.Error()}
			}
			if res.Result == indicator.ResultPassed {
				continue
			} else {
				return res
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// buildMACD 计算 DIF / DEA / Histogram 近 macdSeriesDays 日序列。
//
// klines[0] 为最新K线（价格单位：分），返回的切片 [0] 对应最新一日。
//
// 计算流程：
//  1. 将 klines 反转为 oldest-first（满足 ema 函数的数据假定 data[0]=最旧）
//  2. 计算 EMA(CLOSE,12), EMA(CLOSE,26) → DIF → EMA(DIF,9)=DEA → Histogram
//  3. 取最近 macdSeriesDays 日结果，再反转回 newest-first（[0]=最新）
func buildMACD(klines []*model.DailyKline) MACDResult {
	n := len(klines)

	// 提取收盘价（转为 float64 元），反转成 oldest-first
	closePrices := make([]float64, n)
	for i := range klines {
		closePrices[n-1-i] = float64(klines[i].Close) / 100.0 // 最旧在 [0]
	}

	ema12 := ema(closePrices, macdShortPeriod)  // EMA(CLOSE, 12)
	ema26 := ema(closePrices, macdLongPeriod)   // EMA(CLOSE, 26)

	difAll := make([]float64, n)
	for i := range difAll {
		difAll[i] = ema12[i] - ema26[i]
	}

	deaAll := ema(difAll, macdSignalPeriod) // DEA = EMA(DIF, 9)

	histAll := make([]float64, n)
	for i := range histAll {
		histAll[i] = 2.0 * (difAll[i] -deaAll[i])
	}

	// 取最近 macdSeriesDays 日数据，反转回 newest-first ([0]=最新)
	seriesLen := min(macdSeriesDays, n)
	result := MACDResult{
		DIF:        make([]float64, seriesLen),
		DEA:        make([]float64, seriesLen),
		Histogram: make([]float64, seriesLen),
		ClosePrice: make([]float64, seriesLen),
	}
	for i := 0; i < seriesLen; i++ {
		srcIdx := n - 1 - i // 从末尾取最新的 seriesLen 个元素
		result.DIF[i] = difAll[srcIdx]
		result.DEA[i] =deaAll[srcIdx]
		result.Histogram[i] = histAll[srcIdx]
		result.ClosePrice[i] = closePrices[srcIdx]
	}
	return result
}

// ============================================================================
//  SignalMacdGoldenAboveWater — 01 水上金叉
//
//  判定规则:
//    DIF 在零轴上方 (DIF > 0) 且 DIF 上穿 DEA
//    即: 前一日 DIF <= DEA, 当日 DIF > DEA, 且当日 DIF > 0
// ============================================================================

type SignalMacdGoldenAboveWater struct {
	indicator.BaseSignal
}

func NewSignalMacdGoldenAboveWater() *SignalMacdGoldenAboveWater {
	return &SignalMacdGoldenAboveWater{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"水上金叉",
			"DIF在零轴上方上穿DEA（DIF>0且金叉）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(5, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(5),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalMacdGoldenAboveWater) Evaluate(result MACDResult, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.DIF))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口 [idxStart, idxEnd) 内查找水上金叉
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue // 无前一日数据
		}
		// 金叉条件: 前一日 DIF <= DEA, 当日 DIF > DEA
		if result.DIF[i] > result.DEA[i] && result.DIF[i-1] <= result.DEA[i-1] {
			// 水上条件: 当日 DIF > 0
			if result.DIF[i] > 0 {
				return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
			}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到水上金叉", start, end),
	}
}

// ============================================================================
//  SignalMacdGoldenBelowWater — 02 水下金叉
//
//  判定规则:
//    DIF 在零轴下方或触及零轴 (DIF <= 0) 且 DIF 上穿 DEA
//    即: 前一日 DIF <= DEA, 当日 DIF > DEA, 且当日 DIF <= 0
// ============================================================================

type SignalMacdGoldenBelowWater struct {
	indicator.BaseSignal
}

func NewSignalMacdGoldenBelowWater() *SignalMacdGoldenBelowWater {
	return &SignalMacdGoldenBelowWater{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"水下金叉",
			"DIF在零轴下方上穿DEA（DIF≤0且金叉）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(5, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(5),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalMacdGoldenBelowWater) Evaluate(result MACDResult, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.DIF))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口内查找水下金叉
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}
		// 金叉条件: 前一日 DIF <= DEA, 当日 DIF > DEA
		if result.DIF[i] > result.DEA[i] && result.DIF[i-1] <= result.DEA[i-1] {
			// 水下条件: 当日 DIF <= 0
			if result.DIF[i] <= 0 {
				return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
			}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到水下金叉", start, end),
	}
}

// ============================================================================
//  SignalMacdDeathCross — 05 死叉
//
//  判定规则:
//    DIF 从上方下穿 DEA
//    即: 前一日 DIF >= DEA, 当日 DIF < DEA
// ============================================================================

type SignalMacdDeathCross struct {
	indicator.BaseSignal
}

func NewSignalMacdDeathCross() *SignalMacdDeathCross {
	return &SignalMacdDeathCross{
		BaseSignal: indicator.NewBaseSignal(
			"05",
			"死叉",
			"DIF从上方下穿DEA",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCrossBelow,
					Label:    "死叉",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(5, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCrossBelow,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(5),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalMacdDeathCross) Evaluate(result MACDResult, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.DIF))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口内查找死叉
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}
		// 死叉条件: 前一日 DIF >= DEA, 当日 DIF < DEA
		if result.DIF[i] < result.DEA[i] && result.DIF[i-1] >= result.DEA[i-1] {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到MACD死叉", start, end),
	}
}

// ============================================================================
//  背离检测公共辅助函数
// ============================================================================

// peakWindow 默认局部峰值检测窗口（前后各比较的天数）
const defaultPeakWindow = 5

// paramPeakWindow 背离信号的峰值检测窗口参数 key
const paramPeakWindow = "peak_window"

// findPeaks 在 data 序列中找局部峰值点索引。
// 窗口 [idxStart, idxEnd) 内，data[i] 是峰值的条件：
//   data[i] 比 [i-window, i+window) 范围内的所有其他值都大（或更小）。
//
// 参数:
//   - data: 数据序列，[0]=最新
//   - idxStart / idxEnd: 检查范围（半开区间）
//   - window: 局部峰值检测窗口半径
//   - findMax: true=找极大值(高点), false=找极小值(低点)
//
// 返回: 峰值索引列表（从新到旧排序）
func findPeaks(data []float64, idxStart, idxEnd, window int, findMax bool) []int {
	var peaks []int
	for i := idxStart; i < idxEnd; i++ {
		isPeak := true
		for j := max(0, i-window); j <= min(len(data)-1, i+window); j++ {
			if j == i {
				continue
			}
			if findMax {
				if data[j] > data[i] {
					isPeak = false
					break
				}
			} else {
				if data[j] < data[i] {
					isPeak = false
					break
				}
			}
		}
		if isPeak {
			peaks = append(peaks, i)
		}
	}
	return peaks
}

// checkDivergence 通用背离检测。
//
// 顶背离 (findMax=true): 价格后高 > 前高 且 指标后高 < 前高
// 底背离 (findMax=false): 价格后低 < 前低 且 指标后低 > 前低
//
// 返回: 是否发生背离, 详细消息
func checkDivergence(priceData, indicatorData []float64, idxStart, idxEnd, peakWindow int, isTop bool) (bool, string) {
	pricePeaks := findPeaks(priceData, idxStart, idxEnd, peakWindow, isTop)
	indPeaks := findPeaks(indicatorData, idxStart, idxEnd, peakWindow, isTop)

	// 至少需要两对峰值才能判断背离
	if len(pricePeaks) < 2 || len(indPeaks) < 2 {
		return false, fmt.Sprintf("峰值数量不足（价格%d个, 指标%d个），至少需各2个", len(pricePeaks), len(indPeaks))
	}

	// 取最近两个价格峰值（peaks 是从新到旧排序，pricePeaks[0] 最近）
	p1Idx := pricePeaks[0] // 最近的价格峰值
	p2Idx := pricePeaks[1] // 次近的价格峰值

	// 在指标峰值中找与价格峰值位置最近的匹配
	// 找距离 p1Idx 最近的指标峰值作为 h1，次近的作为 h2
	h1Idx := findNearestPeak(indPeaks, p1Idx)
	h2Idx := findNearestPeak(removePeakAt(indPeaks, h1Idx), p2Idx)

	if h1Idx < 0 || h2Idx < 0 {
		return false, "无法匹配合适的指标峰值"
	}

	if isTop {
		// 顶背离: 价格后高 > 前高 且 指标后高 < 前高
		priceRising := priceData[p1Idx] > priceData[p2Idx]
		indFalling := indicatorData[h1Idx] < indicatorData[h2Idx]

		msg := fmt.Sprintf("价格: %.2f→%.2f(%s), MACD柱: %.4f→%.4f(%s)",
			priceData[p2Idx], priceData[p1Idx], mapBoolStr(priceRising, "创新高", "未新高"),
			indicatorData[h2Idx], indicatorData[h1Idx], mapBoolStr(indFalling, "未新高", "创新高"))

		if priceRising && indFalling {
			return true, msg
		}
		return false, msg + " — 不构成顶背离"
	}

	// 底背离: 价格后低 < 前低 且 指标后低 > 前低
	priceFalling := priceData[p1Idx] < priceData[p2Idx]
	indRising := indicatorData[h1Idx] > indicatorData[h2Idx]

	msg := fmt.Sprintf("价格: %.2f→%.2f(%s), MACD柱: %.4f→%.4f(%s)",
		priceData[p2Idx], priceData[p1Idx], mapBoolStr(priceFalling, "创新低", "未新低"),
		indicatorData[h2Idx], indicatorData[h1Idx], mapBoolStr(indRising, "创新低", "未新低"))

	if priceFalling && indRising {
		return true, msg
	}
	return false, msg + " — 不构成底背离"
}

// findNearestPeak 在 peaks 列表中找离 targetIdx 最近的峰值索引
func findNearestPeak(peaks []int, targetIdx int) int {
	bestIdx := -1
	minDist := 1 << 30 // 大数
	for _, p := range peaks {
		dist := absInt(p - targetIdx)
		if dist < minDist {
			minDist = dist
			bestIdx = p
		}
	}
	return bestIdx
}

// removePeakAt 从峰值列表中移除指定索引，返回新切片
func removePeakAt(peaks []int, removeIdx int) []int {
	result := make([]int, 0, len(peaks)-1)
	for _, p := range peaks {
		if p != removeIdx {
			result = append(result, p)
		}
	}
	return result
}

func mapBoolStr(cond bool, trueStr, falseStr string) string {
	if cond {
		return trueStr
	}
	return falseStr
}

// absInt 返回 int 的绝对值
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ============================================================================
//  SignalMacdTopDivergence — 03 顶背离
//
//  判定规则:
//    股价一峰比一峰高（创新高），但 MACD 红柱图形一峰比一峰低（未创新高）。
//    一般是股价高位即将反转下跌的卖出信号。
//
//  实现方式:
//    在 lookback 窗口内分别找价格局部高点和 Histogram 局部高点，
//    取最近两对进行比较。
// ============================================================================

type SignalMacdTopDivergence struct {
	indicator.BaseSignal
}

func NewSignalMacdTopDivergence() *SignalMacdTopDivergence {
	return &SignalMacdTopDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"顶背离",
			"价格创新高但MACD红柱未创新高（股价高位反转信号）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpDivergencePos,
					Label:    "顶背离",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(60, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						{Key: paramPeakWindow, Label: "峰值检测窗口", Type: "number", Required: false, Default: 5, Min: 2, Max: 20, Step: 1, Unit: "天"},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpDivergencePos,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(60),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramPeakWindow:                 float64(defaultPeakWindow),
				},
			},
		),
	}
}

func (s *SignalMacdTopDivergence) Evaluate(result MACDResult, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.Histogram, idxStart, idxEnd, peakWindow, true); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}

// ============================================================================
//  SignalMacdBottomDivergence — 04 底背离
//
//  判定规则:
//    股价一底比一底低（创新低），但 MACD 绿柱图形一底比一底高（未创新低）。
//    一般是股价低位可能反弹向上的买入信号。
//
//  实现方式:
//    与顶背离对称，使用 findMax=false 找极小值（低点）
// ============================================================================

type SignalMacdBottomDivergence struct {
	indicator.BaseSignal
}

func NewSignalMacdBottomDivergence() *SignalMacdBottomDivergence {
	return &SignalMacdBottomDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"04",
			"底背离",
			"价格创新低但MACD绿柱未创新低（股价低位反弹信号）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpDivergenceNeg,
					Label:    "底背离",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(60, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						{Key: paramPeakWindow, Label: "峰值检测窗口", Type: "number", Required: false, Default: 5, Min: 2, Max: 20, Step: 1, Unit: "天"},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpDivergenceNeg,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(60),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramPeakWindow:                 float64(defaultPeakWindow),
				},
			},
		),
	}
}

func (s *SignalMacdBottomDivergence) Evaluate(result MACDResult, klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.Histogram, idxStart, idxEnd, peakWindow, false); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}
