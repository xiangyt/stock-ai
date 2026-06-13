package technical

import (
	"fmt"

	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  KDJ — 随机指标 (序列型)
//  ID: 01004 = CatCodeTechnical("01") + IndKdjSeq("004")
//  数据源: GetDailyKline()
//
//  设计要点:
//    在 Kdj.Evaluate 层统一计算 K/D/J 序列，
//    所有信号共享同一份 KDJResult，不重复计算。
//
//  公式 (默认参数 N=9, M1=3, M2=3):
//    RSV = (Close - LLV(Low,9)) / (HHV(High,9) - LLV(Low,9)) * 100
//    K   = SMA(RSV, M1, 1)  即 (RSV + (M1-1)*PrevK) / M1
//    D   = SMA(K,   M2, 1)  即 (K   + (M2-1)*PrevD) / M2
//    J   = 3*K - 2*D
// ============================================================================

// KDJResult KDJ 预计算结果，供该指标下所有信号复用。
//
// 数据顺序: 从旧到新 ([0]=最旧, [len-1]=最新)
// 与 NormalizeLookback 的索引映射一致 ("N天前" → dataLen-1-N)
type KDJResult struct {
	K          []float64 // K值序列（从旧到新）
	D          []float64 // D值序列（从旧到新）
	J          []float64 // J值序列（从旧到新）
	ClosePrice []float64 // 收盘价（元，从旧到新）
}

// kdjMinKlines 计算 KDJ 所需的最少 K 线根数
const kdjMinKlines = 33

// KDJ 默认参数
const (
	kdjDefaultN  = 9
	kdjDefaultM1 = 3
	kdjDefaultM2 = 3
)

type Kdj struct {
	indicator.BaseIndicator
}

func NewKdj() *Kdj {
	i := &Kdj{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndKdjSeq,
			NameStr:     "KDJ",
			CategoryVal: indicator.CatTechnical,
			Desc:        "随机指标（金叉/死叉/背离等）",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalKdjGoldenCross(),      // 01 金叉
		NewSignalKdjDeathCross(),       // 02 死叉
		NewSignalKdjBottomDivergence(), // 03 底背离
		NewSignalKdjTopDivergence(),    // 04 顶背离
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate KDJ 指标评估入口
//
//  1. 固定计算 K/D/J 全量序列
//  2. 将结果和原始 klines 传给各信号分发处理
func (i *Kdj) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < kdjMinKlines {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: configs[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", kdjMinKlines, len(klines)),
		}
	}

	result := buildKDJ(klines)

	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch v := s.(type) {
			case *SignalKdjGoldenCross:
				res = v.Evaluate(result, cfg)
			case *SignalKdjDeathCross:
				res = v.Evaluate(result, cfg)
			case *SignalKdjBottomDivergence:
				res = v.Evaluate(result, cfg)
			case *SignalKdjTopDivergence:
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

// buildKDJ 计算 K / D / J 序列。
//
//	klines 输入: [0]=最新, [len-1]=最旧
//	结果: oldest-first (从旧到新), [0]=最旧, [len-1]=最新
func buildKDJ(klines []*model.DailyKline) KDJResult {
	n := len(klines)

	// klines[0] = 最新 → 拷贝并反转为 oldest-first，同时分→元
	closePrices := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i := range klines {
		closePrices[n-1-i] = float64(klines[i].Close) / 100.0
		highs[n-1-i] = float64(klines[i].High) / 100.0
		lows[n-1-i] = float64(klines[i].Low) / 100.0
	}

	// RSV = (Close - LLV(Low,9)) / (HHV(High,9) - LLV(Low,9)) * 100
	llv9 := llv(lows, kdjDefaultN)
	hhv9 := hhv(highs, kdjDefaultN)
	rsv := make([]float64, n)
	for i := range rsv {
		den := hhv9[i] - llv9[i]
		if den != 0 {
			rsv[i] = (closePrices[i] - llv9[i]) / den * 100
		} else {
			rsv[i] = 50
		}
	}

	// K = SMA(RSV, M1, 1), D = SMA(K, M2, 1), J = 3*K - 2*D
	K := sma(rsv, kdjDefaultM1, 1)
	D := sma(K, kdjDefaultM2, 1)
	J := make([]float64, n)
	for i := range J {
		J[i] = 3*K[i] - 2*D[i]
	}

	return KDJResult{
		K:          K,
		D:          D,
		J:          J,
		ClosePrice: closePrices,
	}
}

// ============================================================================
//  SignalKdjGoldenCross — 01 金叉
//
//  判定规则:
//    K 从下方上穿 D
//    即: 前一日 K <= D, 当日 K > D
// ============================================================================

type SignalKdjGoldenCross struct {
	indicator.BaseSignal
}

func NewSignalKdjGoldenCross() *SignalKdjGoldenCross {
	return &SignalKdjGoldenCross{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"金叉",
			"K上穿D（金叉买入信号）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCrossAbove,
					Label:    "金叉",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(5, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCrossAbove,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(5),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
		),
	}
}

func (s *SignalKdjGoldenCross) Evaluate(result KDJResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.K))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口 [idxStart, idxEnd) 内查找金叉
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue // 无前一日数据
		}
		// 金叉条件: 前一日 K <= D, 当日 K > D
		if result.K[i] > result.D[i] && result.K[i-1] <= result.D[i-1] {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到KDJ金叉", start, end),
	}
}

// ============================================================================
//  SignalKdjDeathCross — 02 死叉
//
//  判定规则:
//    K 从上方下穿 D
//    即: 前一日 K >= D, 当日 K < D
// ============================================================================

type SignalKdjDeathCross struct {
	indicator.BaseSignal
}

func NewSignalKdjDeathCross() *SignalKdjDeathCross {
	return &SignalKdjDeathCross{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"死叉",
			"K下穿D（死叉卖出信号）",
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

func (s *SignalKdjDeathCross) Evaluate(result KDJResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.K))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口内查找死叉
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}
		// 死叉条件: 前一日 K >= D, 当日 K < D
		if result.K[i] < result.D[i] && result.K[i-1] >= result.D[i-1] {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到KDJ死叉", start, end),
	}
}

// ============================================================================
//  SignalKdjBottomDivergence — 03 底背离
//
//  判定规则:
//    股价一底比一底低（创新低），但 KDJ 的 K 值一底比一底高（未创新低）。
//    一般是股价低位可能反弹向上的买入信号。
//
//  实现方式:
//    复用 findPeaks / checkDivergence（macd.go 同包公共函数），
//    使用 KDJ K 值替代 MACD 柱。
// ============================================================================

type SignalKdjBottomDivergence struct {
	indicator.BaseSignal
}

func NewSignalKdjBottomDivergence() *SignalKdjBottomDivergence {
	return &SignalKdjBottomDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"底背离",
			"价格创新低但KDJ的K值未创新低（股价低位反弹信号）",
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

func (s *SignalKdjBottomDivergence) Evaluate(result KDJResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.K, idxStart, idxEnd, peakWindow, false); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}

// ============================================================================
//  SignalKdjTopDivergence — 04 顶背离
//
//  判定规则:
//    股价一峰比一峰高（创新高），但 KDJ 的 K 值一峰比一峰低（未创新高）。
//    一般是股价高位即将反转下跌的卖出信号。
//
//  实现方式:
//    与底背离对称，使用 isTop=true
// ============================================================================

type SignalKdjTopDivergence struct {
	indicator.BaseSignal
}

func NewSignalKdjTopDivergence() *SignalKdjTopDivergence {
	return &SignalKdjTopDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"04",
			"顶背离",
			"价格创新高但KDJ的K值未创新高（股价高位反转信号）",
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

func (s *SignalKdjTopDivergence) Evaluate(result KDJResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.K, idxStart, idxEnd, peakWindow, true); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}
