package technical

import (
	"fmt"

	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  RSI — 相对强弱指标 (序列型)
//  ID: 01005 = CatCodeTechnical("01") + IndRsiSeq("005")
//  数据源: GetDailyKline()
//
//  设计要点:
//    在 Rsi.Evaluate 层统一计算 RSI 全量序列，
//    所有信号共享同一份 RSIResult，不重复计算。
//
//  公式 (默认参数 N=14):
//    Delta = Close - Ref(Close, 1)
//    Up   = Max(Delta, 0)
//    Down = Max(-Delta, 0)  即 Abs(Min(Delta, 0))
//    RSI  = SMA(Up, N, 1) / (SMA(Up, N, 1) + SMA(Down, N, 1)) * 100
//
//  信号:
//    01 超买 — RSI 上穿超买线 (默认70)
//    02 超卖 — RSI 下穿超卖线 (默认30)
//    03 顶背离 — 价格创新高但RSI未创新高
//    04 底背离 — 价格创新低但RSI未创新低
// ============================================================================

// RSIResult RSI 预计算结果，供该指标下所有信号复用。
//
// 数据顺序: 从旧到新 ([0]=最旧, [len-1]=最新)
// 与 NormalizeLookback 的索引映射一致 ("N天前" → dataLen-1-N)
type RSIResult struct {
	RSI        []float64 // RSI值序列（从旧到新）
	ClosePrice []float64 // 收盘价（元，从旧到新）
}

// rsiMinKlines 计算 RSI 所需的最少 K 线根数
const rsiMinKlines = 33

// RSI 默认参数
const (
	rsiDefaultPeriod     = 14  // RSI 计算周期
	rsiDefaultOverbought = 70  // 超买阈值
	rsiDefaultOversold   = 30  // 超卖阈值
)

// paramOverbought / paramOversold — 超买/超卖阈值参数 key
const (
	paramOverbought = "overbought"
	paramOversold   = "oversold"
)

type Rsi struct {
	indicator.BaseIndicator
}

func NewRsi() *Rsi {
	i := &Rsi{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndRsiSeq,
			NameStr:     "RSI",
			CategoryVal: indicator.CatTechnical,
			Desc:        "相对强弱指标（超买/超卖/背离等）",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalRsiOverbought(),      // 01 超买
		NewSignalRsiOversold(),        // 02 超卖
		NewSignalRsiTopDivergence(),   // 03 顶背离
		NewSignalRsiBottomDivergence(), // 04 底背离
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate RSI 指标评估入口
//
//  1. 固定计算 RSI 全量序列
//  2. 将结果传给各信号分发处理
func (i *Rsi) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < rsiMinKlines {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: configs[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", rsiMinKlines, len(klines)),
		}
	}

	result := buildRSI(klines)

	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch v := s.(type) {
			case *SignalRsiOverbought:
				res = v.Evaluate(result, cfg)
			case *SignalRsiOversold:
				res = v.Evaluate(result, cfg)
			case *SignalRsiTopDivergence:
				res = v.Evaluate(result, cfg)
			case *SignalRsiBottomDivergence:
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

// buildRSI 计算 RSI 序列。
//
//	公式 (SMA 版本，通达信标准):
//	  Delta[i] = Close[i] - Close[i-1]
//	  Up[i]    = Max(Delta[i], 0)
//	  Down[i]  = Max(-Delta[i], 0)
//	  AvgUp    = SMA(Up, N, 1)    即 (Up + (N-1)*PrevAvgUp) / N
//	  AvgDown  = SMA(Down, N, 1)
//	  RSI      = AvgUp / (AvgUp + AvgDown) * 100
//
//	第一根K线: Delta=0, RSI=50（中性）
//	klines 输入: [0]=最新, [len-1]=最旧
//	结果: oldest-first (从旧到新), [0]=最旧, [len-1]=最新
func buildRSI(klines []*model.DailyKline) RSIResult {
	n := len(klines)

	// klines[0] = 最新 → 拷贝并反转为 oldest-first，同时分→元
	closePrices := make([]float64, n)
	for i := range klines {
		closePrices[n-1-i] = float64(klines[i].Close) / 100.0
	}

	// 计算涨跌幅
	up := make([]float64, n)
	down := make([]float64, n)
	for i := 1; i < n; i++ {
		delta := closePrices[i] - closePrices[i-1]
		if delta > 0 {
			up[i] = delta
		} else {
			down[i] = -delta
		}
	}
	// 第一根K线: 无前值，up/down 均为 0

	// SMA(Up, N, 1) / SMA(Down, N, 1)
	avgUp := sma(up, rsiDefaultPeriod, 1)
	avgDown := sma(down, rsiDefaultPeriod, 1)

	// RSI = AvgUp / (AvgUp + AvgDown) * 100
	rsi := make([]float64, n)
	for i := range rsi {
		sum := avgUp[i] + avgDown[i]
		if sum != 0 {
			rsi[i] = avgUp[i] / sum * 100
		} else {
			rsi[i] = 50 // 无涨跌时取中性值
		}
	}

	return RSIResult{
		RSI:        rsi,
		ClosePrice: closePrices,
	}
}

// ============================================================================
//  SignalRsiOverbought — 01 超买
//
//  判定规则:
//    RSI 从下方上穿超买线
//    即: 前一日 RSI <= 超买线, 当日 RSI > 超买线
// ============================================================================

type SignalRsiOverbought struct {
	indicator.BaseSignal
}

func NewSignalRsiOverbought() *SignalRsiOverbought {
	return &SignalRsiOverbought{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"超买",
			"RSI上穿超买线（超买信号）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(5, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						{Key: paramOverbought, Label: "超买线", Type: "number", Required: false, Default: float64(rsiDefaultOverbought), Min: 50, Max: 100, Step: 1, Unit: ""},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(5),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramOverbought:                 float64(rsiDefaultOverbought),
				},
			},
		),
	}
}

func (s *SignalRsiOverbought) Evaluate(result RSIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	overbought := config.GetFloat64(paramOverbought, float64(rsiDefaultOverbought))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.RSI))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口 [idxStart, idxEnd) 内查找超买信号
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue // 无前一日数据
		}
		// 超买条件: 前一日 RSI <= 超买线, 当日 RSI > 超买线
		if result.RSI[i] > overbought && result.RSI[i-1] <= overbought {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到RSI上穿超买线(%.0f)", start, end, overbought),
	}
}

// ============================================================================
//  SignalRsiOversold — 02 超卖
//
//  判定规则:
//    RSI 从上方下穿超卖线
//    即: 前一日 RSI >= 超卖线, 当日 RSI < 超卖线
// ============================================================================

type SignalRsiOversold struct {
	indicator.BaseSignal
}

func NewSignalRsiOversold() *SignalRsiOversold {
	return &SignalRsiOversold{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"超卖",
			"RSI下穿超卖线（超卖信号）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(5, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						{Key: paramOversold, Label: "超卖线", Type: "number", Required: false, Default: float64(rsiDefaultOversold), Min: 0, Max: 50, Step: 1, Unit: ""},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(5),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramOversold:                   float64(rsiDefaultOversold),
				},
			},
		),
	}
}

func (s *SignalRsiOversold) Evaluate(result RSIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	oversold := config.GetFloat64(paramOversold, float64(rsiDefaultOversold))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.RSI))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口内查找超卖信号
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}
		// 超卖条件: 前一日 RSI >= 超卖线, 当日 RSI < 超卖线
		if result.RSI[i] < oversold && result.RSI[i-1] >= oversold {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到RSI下穿超卖线(%.0f)", start, end, oversold),
	}
}

// ============================================================================
//  SignalRsiTopDivergence — 03 顶背离
//
//  判定规则:
//    股价一峰比一峰高（创新高），但 RSI 一峰比一峰低（未创新高）。
//    一般是股价高位即将反转下跌的卖出信号。
//
//  实现方式:
//    复用 findPeaks / checkDivergence（macd.go 同包公共函数），
//    使用 RSI 值替代 MACD 柱。
// ============================================================================

type SignalRsiTopDivergence struct {
	indicator.BaseSignal
}

func NewSignalRsiTopDivergence() *SignalRsiTopDivergence {
	return &SignalRsiTopDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"顶背离",
			"价格创新高但RSI未创新高（股价高位反转信号）",
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

func (s *SignalRsiTopDivergence) Evaluate(result RSIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.RSI, idxStart, idxEnd, peakWindow, true); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}

// ============================================================================
//  SignalRsiBottomDivergence — 04 底背离
//
//  判定规则:
//    股价一底比一底低（创新低），但 RSI 一底比一底高（未创新低）。
//    一般是股价低位可能反弹向上的买入信号。
//
//  实现方式:
//    与顶背离对称，使用 isTop=false
// ============================================================================

type SignalRsiBottomDivergence struct {
	indicator.BaseSignal
}

func NewSignalRsiBottomDivergence() *SignalRsiBottomDivergence {
	return &SignalRsiBottomDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"04",
			"底背离",
			"价格创新低但RSI未创新低（股价低位反弹信号）",
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

func (s *SignalRsiBottomDivergence) Evaluate(result RSIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.RSI, idxStart, idxEnd, peakWindow, false); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}
