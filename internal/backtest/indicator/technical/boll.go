package technical

import (
	"fmt"
	"math"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  BOLL — 布林带指标 (序列型)
//  ID: 01007 = CatCodeTechnical("01") + IndBollSeq("007")
//  数据源: GetDailyKline()
//
//  设计要点:
//    在 Boll.Evaluate 层统一计算 MB/UP/DN 全量序列，
//    并基于该序列预计算 %B 与 BBW 派生序列，
//    所有信号共享同一份 BOLLResult，不重复计算。
//
//  公式 (默认参数 N=20, K=2):
//    MB = MA(Close, N)                     — 中轨
//    UP = MB + K × STD(Close, N)           — 上轨
//    DN = MB - K × STD(Close, N)           — 下轨
//
//  信号:
//    01 突破上轨 — 收盘价从下方上穿上轨
//    02 跌破下轨 — 收盘价从上方下穿下轨
//    03 顶背离   — 价格创新高但上轨未创新高
//    04 底背离   — 价格创新低但下轨未创新低
//  自定义信号:
//    01 布林带位置(%B) — N天前 (Close-DN)/(UP-DN) 与阈值比较
//    02 布林带宽(BBW)  — N天前 (UP-DN)/MB 与阈值比较（衡量市场波动状态）
//    03 带宽比值(BBW)  — BBW(N天前) ÷ BBW(N+1天前) 与阈值比较（>1=开口扩大）
// ============================================================================

// BOLLResult BOLL 预计算结果，供该指标下所有信号复用。
//
// 数据顺序: 从旧到新 ([0]=最旧, [len-1]=最新)
// 与 NormalizeLookback 的索引映射一致 ("N天前" → dataLen-1-N)
type BOLLResult struct {
	MB         []float64 // 中轨 MB（从旧到新）
	UP         []float64 // 上轨 UP（从旧到新）
	DN         []float64 // 下轨 DN（从旧到新）
	ClosePrice []float64 // 收盘价（元，从旧到新）
	PercentB   []float64 // 布林带位置 %B=(Close-DN)/(UP-DN)，带宽≤0 处为 NaN
	BandWidth  []float64 // 布林带宽 BBW=(UP-DN)/MB，中轨≤0 处为 NaN
}

// bollMinKlines 计算 BOLL 所需的最少 K 线根数
const bollMinKlines = 33

// BOLL 默认参数
const (
	bollDefaultN = 20 // MA 周期
	bollDefaultK = 2  // 标准差倍数
)

// paramK — 标准差倍数参数 key
const paramK = "k"

type Boll struct {
	indicator.BaseIndicator
}

func NewBoll() *Boll {
	i := &Boll{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndBollSeq,
			NameStr:     "BOLL",
			CategoryVal: indicator.CatTechnical,
			Desc:        "布林带指标（突破上轨/跌破下轨/背离等）",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalBollBreakAbove(),       // 01 突破上轨
		NewSignalBollBreakBelow(),       // 02 跌破下轨
		NewSignalBollTopDivergence(),    // 03 顶背离
		NewSignalBollBottomDivergence(), // 04 底背离
	})

	i.SetCustomSignals([]indicator.Signal{
		NewSignalBollPosition(),       // 自定义 01 布林带位置（%B）
		NewSignalBollBandWidth(),      // 自定义 02 布林带宽（BBW）
		NewSignalBollBandWidthRatio(), // 自定义 03 布林带宽比值（BBW）
	})
	return i
}

// Evaluate BOLL 指标评估入口
//
//  1. 固定计算 MB/UP/DN 全量序列
//  2. 将结果传给各信号分发处理
func (i *Boll) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < bollMinKlines {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: configs[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", bollMinKlines, len(klines)),
		}
	}

	result := buildBOLL(klines)

	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch v := s.(type) {
			case *SignalBollBreakAbove:
				res = v.Evaluate(result, cfg)
			case *SignalBollBreakBelow:
				res = v.Evaluate(result, cfg)
			case *SignalBollTopDivergence:
				res = v.Evaluate(result, cfg)
			case *SignalBollBottomDivergence:
				res = v.Evaluate(result, cfg)
			case *sigBollPos:
				res = v.Evaluate(result, cfg)
			case *sigBollBw:
				res = v.Evaluate(result, cfg)
			case *sigBollBwRatio:
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

// buildBOLL 计算 MB / UP / DN 序列及派生序列 %B、BBW。
//
//	公式:
//	  MB = MA(Close, N)
//	  UP = MB + K × STD(Close, N)
//	  DN = MB - K × STD(Close, N)
//	  STD = sqrt(Σ(Close[i] - MA)² / N)  总体标准差
//	  %B = (Close - DN) / (UP - DN)
//	  BBW = (UP - DN) / MB
//
//	klines 输入: [0]=最新, [len-1]=最旧
//	结果: oldest-first (从旧到新), [0]=最旧, [len-1]=最新
func buildBOLL(klines []*model.DailyKline) BOLLResult {
	n := len(klines)

	// klines[0] = 最新 → 拷贝并反转为 oldest-first，同时分→元
	closePrices := make([]float64, n)
	for i := range klines {
		closePrices[n-1-i] = float64(klines[i].Close) / 100.0
	}

	// MB = MA(Close, N)
	mb := ma(closePrices, bollDefaultN)

	// 计算标准差和上下轨
	up := make([]float64, n)
	dn := make([]float64, n)

	for i := 0; i < n; i++ {
		std := calcStd(closePrices, i, bollDefaultN)
		up[i] = mb[i] + float64(bollDefaultK)*std
		dn[i] = mb[i] - float64(bollDefaultK)*std
	}

	// 派生序列 %B 与 BBW，供信号直接按索引取值
	percentB, bandWidth := calcBollDerived(closePrices, mb, up, dn)

	return BOLLResult{
		MB:         mb,
		UP:         up,
		DN:         dn,
		ClosePrice: closePrices,
		PercentB:   percentB,
		BandWidth:  bandWidth,
	}
}

// calcBollDerived 基于收盘价与 MB/UP/DN 序列预计算派生序列:
//
//	PercentB[i] = (Close[i] - DN[i]) / (UP[i] - DN[i])  布林带位置 %B
//	BandWidth[i] = (UP[i] - DN[i]) / MB[i]              布林带宽 BBW
//
// 分母非法处以 NaN 填充（%B: 带宽≤0；BBW: 中轨≤0），调用方按"无法计算"拒绝。
func calcBollDerived(closePx, mb, up, dn []float64) ([]float64, []float64) {
	n := len(closePx)
	percentB := make([]float64, n)
	bandWidth := make([]float64, n)
	for i := 0; i < n; i++ {
		band := up[i] - dn[i]
		if band > 0 {
			percentB[i] = (closePx[i] - dn[i]) / band
		} else {
			percentB[i] = math.NaN()
		}
		if mb[i] > 0 {
			bandWidth[i] = band / mb[i]
		} else {
			bandWidth[i] = math.NaN()
		}
	}
	return percentB, bandWidth
}

// calcStd 计算以 i 为终点、长度为 period 的窗口内总体标准差。
// 数据不足 period 根时，用已有数据计算。
func calcStd(data []float64, i, period int) float64 {
	start := i - period + 1
	if start < 0 {
		start = 0
	}
	m := float64(0)
	count := float64(i - start + 1)
	for j := start; j <= i; j++ {
		m += data[j]
	}
	m /= count

	variance := float64(0)
	for j := start; j <= i; j++ {
		diff := data[j] - m
		variance += diff * diff
	}
	variance /= count

	return math.Sqrt(variance)
}

// ============================================================================
//  SignalBollBreakAbove — 01 突破上轨
//
//  判定规则:
//    收盘价从下方上穿上轨
//    即: 前一日 Close <= UP, 当日 Close > UP
// ============================================================================

type SignalBollBreakAbove struct {
	indicator.BaseSignal
}

func NewSignalBollBreakAbove() *SignalBollBreakAbove {
	return &SignalBollBreakAbove{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"突破上轨",
			"收盘价从下方上穿布林带上轨（强势突破信号）",
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

func (s *SignalBollBreakAbove) Evaluate(result BOLLResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口 [idxStart, idxEnd) 内查找突破上轨信号
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue // 无前一日数据
		}
		// 突破上轨条件: 前一日 Close <= UP, 当日 Close > UP
		if result.ClosePrice[i] > result.UP[i] && result.ClosePrice[i-1] <= result.UP[i-1] {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到收盘价突破布林带上轨", start, end),
	}
}

// ============================================================================
//  SignalBollBreakBelow — 02 跌破下轨
//
//  判定规则:
//    收盘价从上方下穿下轨
//    即: 前一日 Close >= DN, 当日 Close < DN
// ============================================================================

type SignalBollBreakBelow struct {
	indicator.BaseSignal
}

func NewSignalBollBreakBelow() *SignalBollBreakBelow {
	return &SignalBollBreakBelow{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"跌破下轨",
			"收盘价从上方下穿布林带下轨（超卖信号）",
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

func (s *SignalBollBreakBelow) Evaluate(result BOLLResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口内查找跌破下轨信号
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}
		// 跌破下轨条件: 前一日 Close >= DN, 当日 Close < DN
		if result.ClosePrice[i] < result.DN[i] && result.ClosePrice[i-1] >= result.DN[i-1] {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到收盘价跌破布林带下轨", start, end),
	}
}

// ============================================================================
//  SignalBollTopDivergence — 03 顶背离
//
//  判定规则:
//    股价一峰比一峰高（创新高），但布林带上轨一峰比一峰低（未创新高）。
//    一般是股价高位即将反转下跌的卖出信号。
//
//  实现方式:
//    复用 findPeaks / checkDivergence（macd.go 同包公共函数），
//    使用上轨 UP 替代 MACD 柱。
// ============================================================================

type SignalBollTopDivergence struct {
	indicator.BaseSignal
}

func NewSignalBollTopDivergence() *SignalBollTopDivergence {
	return &SignalBollTopDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"顶背离",
			"价格创新高但布林带上轨未创新高（股价高位反转信号）",
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

func (s *SignalBollTopDivergence) Evaluate(result BOLLResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.UP, idxStart, idxEnd, peakWindow, true); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}

// ============================================================================
//  SignalBollBottomDivergence — 04 底背离
//
//  判定规则:
//    股价一底比一底低（创新低），但布林带下轨一底比一底高（未创新低）。
//    一般是股价低位可能反弹向上的买入信号。
//
//  实现方式:
//    与顶背离对称，使用 isTop=false
// ============================================================================

type SignalBollBottomDivergence struct {
	indicator.BaseSignal
}

func NewSignalBollBottomDivergence() *SignalBollBottomDivergence {
	return &SignalBollBottomDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"04",
			"底背离",
			"价格创新低但布林带下轨未创新低（股价低位反弹信号）",
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

func (s *SignalBollBottomDivergence) Evaluate(result BOLLResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.DN, idxStart, idxEnd, peakWindow, false); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}

// ============================================================================
//  sigBollPos — 布林带位置（%B）数值比较自定义信号
//
//  判定规则:
//    取距今日 days 天前那根 K 线，计算其收盘价在当日布林带内的相对位置:
//      %B = (Close - DN) / (UP - DN)
//    将 %B 与阈值做数值比较（>、<、≥、≤、区间内/外等）。
//    %B 取值范围: 0=贴近下轨, 1=贴上轨；突破带外时可能 <0 或 >1。
// ============================================================================

// bollNumberOps 返回 BOLL 数值比较信号（如 %B、BBW）共用的操作符列表，
// 每个操作符首参为"取值天数"。
func bollNumberOps(defaultThreshold float64) []indicator.OperatorOption {
	daysP := indicator.ParamDef{Key: indicator.ParamKeyDays, Label: "取值天数", Type: "number", Default: 0, Min: 0, Step: 1, Unit: "天前", Required: false}
	thresh := indicator.ParamDef{Key: indicator.ParamKeyThreshold, Label: "阈值", Type: "number", Default: defaultThreshold}
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

// bollPosOps 返回布林带位置（%B）信号的操作符列表（阈值默认 0.8）。
func bollPosOps() []indicator.OperatorOption { return bollNumberOps(0.8) }

// bollBwOps 返回布林带宽（BBW）信号的操作符列表（阈值默认 0.15，张开过度线）。
func bollBwOps() []indicator.OperatorOption { return bollNumberOps(0.15) }

// bollBwRatioOps 返回布林带宽比值（BBW 两点变化）信号的操作符列表（阈值默认 1）。
func bollBwRatioOps() []indicator.OperatorOption { return bollNumberOps(1) }

type sigBollPos struct {
	indicator.BaseSignal
}

// NewSignalBollPosition 创建布林带位置（%B）数值比较自定义信号。
func NewSignalBollPosition() *sigBollPos {
	return &sigBollPos{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"布林带位置(%B)",
			"取N天前收盘价在布林带中的位置 (Close-DN)/(UP-DN)，与阈值比较",
			indicator.ValNumber,
			bollPosOps(),
			&indicator.SignalConfig{
				Operator: indicator.OpGTE,
				Params: map[string]any{
					indicator.ParamKeyThreshold: float64(0.8),
					indicator.ParamKeyDays:      float64(0),
				},
			},
		),
	}
}

func (s *sigBollPos) Evaluate(result BOLLResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	days := config.GetInt(indicator.ParamKeyDays, 0)
	idx, err := macdValIdx(len(result.PercentB), days)
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: err.Error()}
	}

	// 复用预计算的 %B 序列；带宽非正处为 NaN，视为无法计算
	pos := result.PercentB[idx]
	if math.IsNaN(pos) {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: fmt.Sprintf("距今日%d天布林带宽度非正，无法计算位置", days)}
	}

	label := fmt.Sprintf("距今日%d天%%B位置", days)
	return signalutil.EvalNumberOp(pos, label, "%.2f", "%.2f", sId, config)
}

// ============================================================================
//  sigBollBw — 布林带宽（BBW）数值比较自定义信号
//
//  判定规则:
//    取距今日 days 天前那根 K 线的布林带宽:
//      BBW = (UP - DN) / MB
//    将 BBW 与阈值做数值比较（>、<、≥、≤、区间内/外等）。
//    含义: BBW 衡量当日带宽绝对宽度，反映市场波动状态（大=波动剧烈、小=波动低迷）；
//    开口扩大/收窄等相对变化请用 03 带宽比值信号。
// ============================================================================

type sigBollBw struct {
	indicator.BaseSignal
}

// NewSignalBollBandWidth 创建布林带宽（BBW）数值比较自定义信号。
func NewSignalBollBandWidth() *sigBollBw {
	return &sigBollBw{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"布林带宽(BBW)",
			"取N天前布林带宽BBW=(UP-DN)/MB与阈值比较，衡量当日市场波动状态（高=波动剧烈、低=波动低迷）。经验参考：<0.05 挤压待变盘、0.06~0.08 收敛蓄势、>0.15 过度张开易回调/衰竭",
			indicator.ValNumber,
			bollBwOps(),
			&indicator.SignalConfig{
				Operator: indicator.OpGTE,
				Params: map[string]any{
					indicator.ParamKeyThreshold: float64(0.15),
					indicator.ParamKeyDays:      float64(0),
				},
			},
		),
	}
}

func (s *sigBollBw) Evaluate(result BOLLResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	days := config.GetInt(indicator.ParamKeyDays, 0)
	idx, err := macdValIdx(len(result.BandWidth), days)
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: err.Error()}
	}

	// 复用预计算的 BBW 序列；中轨非正处为 NaN，视为无法计算
	bbw := result.BandWidth[idx]
	if math.IsNaN(bbw) {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: fmt.Sprintf("距今日%d天中轨MB非正，无法计算带宽", days)}
	}

	label := fmt.Sprintf("距今日%d天BBW带宽", days)
	return signalutil.EvalNumberOp(bbw, label, "%.4f", "%.4f", sId, config)
}

// ============================================================================
//  sigBollBwRatio — 布林带宽比值（BBW 两点变化）数值比较自定义信号
//
//  判定规则:
//    取 N 天前的 BBW 与 N+1 天前的 BBW 做比值:
//      Ratio = BBW(N天前) ÷ BBW(N+1天前)
//    将 Ratio 与阈值做数值比较（默认 1）。
//    Ratio > 1 = 带宽开口扩大（波动率增加）；Ratio < 1 = 带宽收窄（波动率降低）。
// ============================================================================

type sigBollBwRatio struct {
	indicator.BaseSignal
}

// NewSignalBollBandWidthRatio 创建布林带宽比值（BBW 两点变化）数值比较自定义信号。
func NewSignalBollBandWidthRatio() *sigBollBwRatio {
	return &sigBollBwRatio{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"带宽比值(BBW)",
			"比较 BBW(N天前)÷BBW(N+1天前)：>1 带宽开口扩大(波动率升)，<1 收窄(波动率降)",
			indicator.ValNumber,
			bollBwRatioOps(),
			&indicator.SignalConfig{
				Operator: indicator.OpGTE,
				Params: map[string]any{
					indicator.ParamKeyThreshold: float64(1),
					indicator.ParamKeyDays:      float64(0),
				},
			},
		),
	}
}

func (s *sigBollBwRatio) Evaluate(result BOLLResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	days := config.GetInt(indicator.ParamKeyDays, 0)
	dataLen := len(result.BandWidth)

	// 近端 N 天前
	idx, err := macdValIdx(dataLen, days)
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: err.Error()}
	}
	// 远端 N+1 天前（作为分母）
	idxPrev, err := macdValIdx(dataLen, days+1)
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: err.Error()}
	}

	// 复用预计算的 BBW 序列；近端中轨非正处为 NaN，视为无法计算
	near := result.BandWidth[idx]
	if math.IsNaN(near) {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: fmt.Sprintf("距今日%d天中轨MB非正，无法计算带宽", days)}
	}

	// 远端 BBW(N+1天前) 作分母：中轨非正为 NaN、带宽为 0 均无法求比值
	far := result.BandWidth[idxPrev]
	if math.IsNaN(far) {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: fmt.Sprintf("距今日%d天中轨MB非正，无法计算带宽", days+1)}
	}
	if far == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: fmt.Sprintf("N+1天前(%d天前)BBW为0，无法计算比值", days+1)}
	}

	// Ratio = BBW(N天前) / BBW(N+1天前)
	ratio := near / far
	label := fmt.Sprintf("BBW(%d天前)÷BBW(%d天前)带宽比值", days, days+1)
	return signalutil.EvalNumberOp(ratio, label, "%.4f", "%.4f", sId, config)
}
