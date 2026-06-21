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
//    在 MACD.Evaluate 层统一计算 DIF/DEA/MACD 全量序列，
//    所有信号共享同一份 MACDResult，不重复计算。
//
//  公式:
//    EMA12 = EMA(CLOSE, 12)
//    EMA26 = EMA(CLOSE, 26)
//    DIF   = EMA12 - EMA26
//    DEA   = EMA(DIF, 9)
//    MACD柱= 2 * (DIF - DEA)
// ============================================================================

// MACDResult MACD 预计算结果，供该指标下所有信号复用。
//
// 数据顺序: 从旧到新 ([0]=最旧, [len-1]=最新)
// 与 NormalizeLookback 的索引映射一致 ("N天前" → dataLen-1-N)
type MACDResult struct {
	DIF        []float64 // DIF快线（从旧到新）
	DEA        []float64 // DEA慢线（从旧到新）
	MACD  []float64 // MACD柱 2*(DIF-DEA)（从旧到新）
	ClosePrice []float64 // 收盘价（元，从旧到新）
}

// macdMinKlines 计算 MACD 所需的最少 K 线根数
const macdMinKlines = 60

// MACD 默认参数
const (
	macdShortPeriod  = 12 // 快线 EMA 周期
	macdLongPeriod   = 26 // 慢线 EMA 周期
	macdSignalPeriod = 9  // DEA 信号线周期
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
		newSignalMacdCross("01", "水上金叉", "DIF在零轴上方上穿DEA（DIF>0且金叉）", true, true, indicator.OpCustom, "参数设置"),
		newSignalMacdCross("02", "水下金叉", "DIF在零轴下方上穿DEA（DIF≤0且金叉）", true, false, indicator.OpCustom, "参数设置"),
		newSignalMacdCross("03", "水上死叉", "DIF在零轴上方下穿DEA（DIF>0且死叉）", false, true, indicator.OpCrossBelow, "死叉"),
		newSignalMacdCross("04", "水下死叉", "DIF在零轴下方下穿DEA（DIF≤0且死叉）", false, false, indicator.OpCrossBelow, "死叉"),
		NewSignalMacdTopDivergence(),    // 05 顶背离
		NewSignalMacdBottomDivergence(), // 06 底背离
	})

	i.SetCustomSignals([]indicator.Signal{
		newSigMacdVal(macdValDIF, "01", "DIF值", "DIF快线数值比较", indicator.OpGTE),
		newSigMacdVal(macdValDEA, "02", "DEA值", "DEA慢线数值比较", indicator.OpGTE),
		newSigMacdVal(macdValHist, "03", "MACD柱", "MACD柱线数值比较", indicator.OpGT),
	})
	return i
}

// Evaluate MACD 指标评估入口
//
// 1. 计算 DIF/DEA/MACD 全量序列
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

	if len(klines) < macdMinKlines {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: configs[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", macdMinKlines, len(klines)),
		}
	}

	result := buildMACD(klines)

	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch v := s.(type) {
			case *SignalMacdCross:
				res = v.Evaluate(result, klines, cfg)
			case *SignalMacdTopDivergence:
				res = v.Evaluate(result, klines, cfg)
			case *SignalMacdBottomDivergence:
				res = v.Evaluate(result, klines, cfg)
			case *sigMacdVal:
				res = v.Evaluate(result, cfg)
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

// buildMACD 计算 DIF / DEA / MACD 序列。
//
//	严格按照递推公式（对应参考实现 Macd()）:
//	  EMA12 = 2/(12+1)*Close + (12-1)/(12+1)*PrevEMA12
//	  EMA26 = 2/(26+1)*Close + (26-1)/(26+1)*PrevEMA26
//	  DIF   = EMA12 - EMA26
//	  DEA   = 2/(9+1)*DIF   + (9-1)/(9+1)*PrevDEA
//	  MACD  = 2 * (DIF - DEA)
//
//	第一根K线: EMA12=EMA26=Close, DIF=0, DEA=0
//	klines 输入: [0]=最新, [len-1]=最旧
//	结果: oldest-first (从旧到新), [0]=最旧, [len-1]=最新
func buildMACD(klines []*model.DailyKline) MACDResult {
	n := len(klines)

	// klines[0] = 最新 → 拷贝并反转为 oldest-first，同时分→元
	closePrices := make([]float64, n)
	for i := range klines {
		closePrices[n-1-i] = float64(klines[i].Close) / 100.0
	}

	dif := make([]float64, n)
	dea := make([]float64, n)
	hist := make([]float64, n)

	shortA := 2.0 / (float64(macdShortPeriod) + 1.0) // α₁₂ = 2/(12+1)
	longA := 2.0 / (float64(macdLongPeriod) + 1.0)   // α₂₆ = 2/(26+1)
	sigA := 2.0 / (float64(macdSignalPeriod) + 1.0)  // α₉ = 2/(9+1)

	var ema12, ema26, deaPrev float64

	for i := 0; i < n; i++ { // 从旧到新遍历
		end := closePrices[i]

		if i == 0 {
			// 第一根K线：EMA 初始 = 收盘价
			ema12 = end
			ema26 = end
		} else {
			// EMA12 = α₁₂*Close + (1-α₁₂)*PrevEMA12
			ema12 = end*shortA + ema12*(1-shortA)
			// EMA26 = α₂₆*Close + (1-α₂₆)*PrevEMA26
			ema26 = end*longA + ema26*(1-longA)
		}

		diff := ema12 - ema26 // DIF

		if i == 0 {
			deaPrev = diff // 首个 DEA = 首个 DIF（即 0）
		} else {
			// DEA = α₉*DIF + (1-α₉)*PrevDEA
			deaPrev = diff*sigA + deaPrev*(1-sigA)
		}

		dif[i] = diff
		dea[i] = deaPrev
		hist[i] = 2.0 * (diff - deaPrev) // MACD柱 = 2*(DIF-DEA)
	}

	return MACDResult{
		DIF:        dif,
		DEA:        dea,
		MACD:  hist,
		ClosePrice: closePrices,
	}
}

// ============================================================================
//  SignalMacdCross — 金叉/死叉（水上/水下共用）
//
//  金叉: 前一日 DIF <= DEA, 当日 DIF > DEA
//  死叉: 前一日 DIF >= DEA, 当日 DIF < DEA
//  水上: 当日 DIF > 0
//  水下: 当日 DIF <= 0
// ============================================================================

// signalCrossName 返回信号的展示名称
func signalCrossName(isGolden, aboveWater bool) string {
	if isGolden {
		if aboveWater {
			return "水上金叉"
		}
		return "水下金叉"
	}
	if aboveWater {
		return "水上死叉"
	}
	return "水下死叉"
}

type SignalMacdCross struct {
	indicator.BaseSignal
	IsGolden   bool // true=金叉, false=死叉
	AboveWater bool // true=水上(DIF>0), false=水下(DIF<=0)
}

// newSignalMacdCross 创建金叉/死叉信号的工厂函数
func newSignalMacdCross(id, name, desc string, isGolden, aboveWater bool, op indicator.CompareOperator, label string) *SignalMacdCross {
	return &SignalMacdCross{
		BaseSignal: indicator.NewBaseSignal(
			id,
			name,
			desc,
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: op,
					Label:    label,
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(0, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: op,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(0),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
		IsGolden:   isGolden,
		AboveWater: aboveWater,
	}
}

func (s *SignalMacdCross) Evaluate(result MACDResult, _ []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.DIF))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	name := signalCrossName(s.IsGolden, s.AboveWater)

	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}

		crossed := false
		if s.IsGolden {
			crossed = result.DIF[i] > result.DEA[i] && result.DIF[i-1] <= result.DEA[i-1]
		} else {
			crossed = result.DIF[i] < result.DEA[i] && result.DIF[i-1] >= result.DEA[i-1]
		}

		if !crossed {
			continue
		}

		// 水上/水下校验
		if s.AboveWater && result.DIF[i] > 0 {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
		if !s.AboveWater && result.DIF[i] <= 0 {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到%s", start, end, name),
	}
}

// ============================================================================
//  sigMacdVal — MACD 数值比较自定义信号
//
//  对 MACD 的 DIF、DEA、MACD柱 进行数值比较（>、<、≥、≤、区间等）。
//  通过 series 字段区分取值来源，支持 "days" 参数指定往前第几天（0=最新）。
// ============================================================================

type macdValSeries int

const (
	macdValDIF  macdValSeries = iota // DIF 快线
	macdValDEA                       // DEA 慢线
	macdValHist                      // MACD 柱线
)

func (s macdValSeries) label() string {
	switch s {
	case macdValDIF:
		return "DIF"
	case macdValDEA:
		return "DEA"
	default:
		return "MACD柱"
	}
}

// macdValOps 返回 MACD 数值信号的可用操作符列表，每个操作符首参为"取值天数"。
func macdValOps() []indicator.OperatorOption {
	daysP := indicator.ParamDef{Key: indicator.ParamKeyDays, Label: "取值天数", Type: "number", Default: 0, Min: 0, Step: 1, Unit: "天前", Required: false}
	thresh := indicator.ParamDef{Key: indicator.ParamKeyThreshold, Label: "阈值", Type: "number"}
	minP := indicator.ParamDef{Key: indicator.ParamKeyMin, Label: "下限", Type: "number"}
	maxP := indicator.ParamDef{Key: indicator.ParamKeyMax, Label: "上限", Type: "number"}
	return []indicator.OperatorOption{
		{Operator: indicator.OpLT, Label: "小于", Params: []indicator.ParamDef{daysP, thresh}},
		{Operator: indicator.OpLTE, Label: "小于等于", Params: []indicator.ParamDef{daysP, thresh}},
		{Operator: indicator.OpGT, Label: "大于", Params: []indicator.ParamDef{daysP, thresh}},
		{Operator: indicator.OpGTE, Label: "大于等于", Params: []indicator.ParamDef{daysP, thresh}},
		{Operator: indicator.OpBetween, Label: "区间内", Params: []indicator.ParamDef{daysP, minP, maxP}},
		{Operator: indicator.OpNotBetween, Label: "区间外", Params: []indicator.ParamDef{daysP, minP, maxP}},
	}
}

// macdValIdx 根据 days 参数计算取值索引，返回索引和可能的错误。
func macdValIdx(dataLen, daysAgo int) (int, error) {
	idx := dataLen - 1 - daysAgo
	if idx < 0 {
		return 0, fmt.Errorf("往前第 %d 天超出数据范围（共 %d 天）", daysAgo, dataLen)
	}
	return idx, nil
}

type sigMacdVal struct {
	indicator.BaseSignal
	series macdValSeries
}

// newSigMacdVal 创建 MACD 数值比较自定义信号
func newSigMacdVal(series macdValSeries, id, name, desc string, defaultOp indicator.CompareOperator) *sigMacdVal {
	return &sigMacdVal{
		BaseSignal: indicator.NewBaseSignal(id, name, desc, indicator.ValNumber, macdValOps(),
			&indicator.SignalConfig{Operator: defaultOp, Params: map[string]any{
				indicator.ParamKeyThreshold: float64(0),
				indicator.ParamKeyDays:      float64(0),
			}},
		),
		series: series,
	}
}

func (s *sigMacdVal) Evaluate(result MACDResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	var data []float64
	switch s.series {
	case macdValDIF:
		data = result.DIF
	case macdValDEA:
		data = result.DEA
	default:
		data = result.MACD
	}

	idx, err := macdValIdx(len(data), config.GetInt(indicator.ParamKeyDays, 0))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: err.Error()}
	}
	return signalutil.EvalNumberOp(data[idx], s.series.label(), "%.4f", "%.4f", sId, config)
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
//
//	data[i] 比 [i-window, i+window) 范围内的所有其他值都大（或更小）。
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
//    在 lookback 窗口内分别找价格局部高点和 MACD 局部高点，
//    取最近两对进行比较。
// ============================================================================

type SignalMacdTopDivergence struct {
	indicator.BaseSignal
}

func NewSignalMacdTopDivergence() *SignalMacdTopDivergence {
	return &SignalMacdTopDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"05",
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

	if ok, msg := checkDivergence(result.ClosePrice, result.MACD, idxStart, idxEnd, peakWindow, true); ok {
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
			"06",
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

	if ok, msg := checkDivergence(result.ClosePrice, result.MACD, idxStart, idxEnd, peakWindow, false); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}
