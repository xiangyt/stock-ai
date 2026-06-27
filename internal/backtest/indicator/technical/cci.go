package technical

import (
	"fmt"
	"math"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  CCI — 商品通道指标 (序列型)
//  ID: 01008 = CatCodeTechnical("01") + IndCciSeq("008")
//  数据源: GetDailyKline()
//
//  设计要点:
//    在 Cci.Evaluate 层统一计算 CCI 全量序列，
//    所有信号共享同一份 CCIResult，不重复计算。
//
//  公式 (默认参数 N=14):
//    TP  = (High + Low + Close) / 3       — 典型价格
//    MA  = MA(TP, N)                       — TP 的简单移动平均
//    MD  = Σ|TP[i] - MA| / N              — 平均偏差 (Mean Deviation)
//    CCI = (TP - MA) / (0.015 × MD)       — 0.015 为经验常数
//
//  信号:
//    01 超买 — CCI 上穿 +100
//    02 超卖 — CCI 下穿 -100
//    03 顶背离 — 价格创新高但CCI未创新高
//    04 底背离 — 价格创新低但CCI未创新低
// ============================================================================

// CCIResult CCI 预计算结果，供该指标下所有信号复用。
//
// 数据顺序: 从旧到新 ([0]=最旧, [len-1]=最新)
// 与 NormalizeLookback 的索引映射一致 ("N天前" → dataLen-1-N)
type CCIResult struct {
	CCI        []float64 // CCI值序列（从旧到新）
	ClosePrice []float64 // 收盘价（元，从旧到新）
}

// cciMinKlines 计算 CCI 所需的最少 K 线根数
const cciMinKlines = 33

// CCI 默认参数
const (
	cciDefaultN          = 14  // CCI 计算周期
	cciDefaultOverbought = 100 // 超买阈值
	cciDefaultOversold   = -100 // 超卖阈值
	cciConstant          = 0.015 // CCI 经验常数
)

// paramOverbought / paramOversold — 超买/超卖阈值参数 key
const (
	paramCciOverbought = "overbought"
	paramCciOversold   = "oversold"
)

type Cci struct {
	indicator.BaseIndicator
}

func NewCci() *Cci {
	i := &Cci{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndCciSeq,
			NameStr:     "CCI",
			CategoryVal: indicator.CatTechnical,
			Desc:        "商品通道指标（超买/超卖/背离等）",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalCciOverbought(),      // 01 超买
		NewSignalCciOversold(),        // 02 超卖
		NewSignalCciTopDivergence(),   // 03 顶背离
		NewSignalCciBottomDivergence(), // 04 底背离
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate CCI 指标评估入口
//
//  1. 固定计算 CCI 全量序列
//  2. 将结果传给各信号分发处理
func (i *Cci) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < cciMinKlines {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: configs[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", cciMinKlines, len(klines)),
		}
	}

	result := buildCCI(klines)

	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch v := s.(type) {
			case *SignalCciOverbought:
				res = v.Evaluate(result, cfg)
			case *SignalCciOversold:
				res = v.Evaluate(result, cfg)
			case *SignalCciTopDivergence:
				res = v.Evaluate(result, cfg)
			case *SignalCciBottomDivergence:
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

// buildCCI 计算 CCI 序列。
//
//	公式:
//	  TP  = (High + Low + Close) / 3
//	  MA  = MA(TP, N)
//	  MD  = Σ|TP[i] - MA| / N    (N周期平均偏差)
//	  CCI = (TP - MA) / (0.015 × MD)
//
//	klines 输入: [0]=最新, [len-1]=最旧
//	结果: oldest-first (从旧到新), [0]=最旧, [len-1]=最新
func buildCCI(klines []*model.DailyKline) CCIResult {
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

	// TP = (High + Low + Close) / 3
	tp := make([]float64, n)
	for i := 0; i < n; i++ {
		tp[i] = (highs[i] + lows[i] + closePrices[i]) / 3.0
	}

	// MA = MA(TP, N)
	tpMA := ma(tp, cciDefaultN)

	// MD = Σ|TP[i] - MA| / N，滑动窗口计算
	ccis := make([]float64, n)
	for i := 0; i < n; i++ {
		start := i - cciDefaultN + 1
		if start < 0 {
			start = 0
		}
		count := i - start + 1

		// 计算平均偏差 MD
		md := 0.0
		for j := start; j <= i; j++ {
			md += math.Abs(tp[j] - tpMA[i])
		}
		md /= float64(count)

		// CCI = (TP - MA) / (0.015 × MD)
		if md != 0 {
			ccis[i] = (tp[i] - tpMA[i]) / (cciConstant * md)
		} else {
			ccis[i] = 0 // 无偏差时CCI取0
		}
	}

	return CCIResult{
		CCI:        ccis,
		ClosePrice: closePrices,
	}
}

// ============================================================================
//  SignalCciOverbought — 01 超买
//
//  判定规则:
//    CCI 从下方上穿超买线
//    即: 前一日 CCI <= 超买线, 当日 CCI > 超买线
// ============================================================================

type SignalCciOverbought struct {
	indicator.BaseSignal
}

func NewSignalCciOverbought() *SignalCciOverbought {
	return &SignalCciOverbought{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"超买",
			"CCI上穿超买线（超买信号）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(5, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						{Key: paramCciOverbought, Label: "超买线", Type: "number", Required: false, Default: float64(cciDefaultOverbought), Min: 50, Max: 300, Step: 10, Unit: ""},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(5),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramCciOverbought:              float64(cciDefaultOverbought),
				},
			},
		),
	}
}

func (s *SignalCciOverbought) Evaluate(result CCIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	overbought := config.GetFloat64(paramCciOverbought, float64(cciDefaultOverbought))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.CCI))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口 [idxStart, idxEnd) 内查找超买信号
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue // 无前一日数据
		}
		// 超买条件: 前一日 CCI <= 超买线, 当日 CCI > 超买线
		if result.CCI[i] > overbought && result.CCI[i-1] <= overbought {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到CCI上穿超买线(%.0f)", start, end, overbought),
	}
}

// ============================================================================
//  SignalCciOversold — 02 超卖
//
//  判定规则:
//    CCI 从上方下穿超卖线
//    即: 前一日 CCI >= 超卖线, 当日 CCI < 超卖线
// ============================================================================

type SignalCciOversold struct {
	indicator.BaseSignal
}

func NewSignalCciOversold() *SignalCciOversold {
	return &SignalCciOversold{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"超卖",
			"CCI下穿超卖线（超卖信号）",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(5, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						{Key: paramCciOversold, Label: "超卖线", Type: "number", Required: false, Default: float64(cciDefaultOversold), Min: -300, Max: -50, Step: 10, Unit: ""},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(5),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramCciOversold:                float64(cciDefaultOversold),
				},
			},
		),
	}
}

func (s *SignalCciOversold) Evaluate(result CCIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	oversold := config.GetFloat64(paramCciOversold, float64(cciDefaultOversold))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.CCI))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 在窗口内查找超卖信号
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}
		// 超卖条件: 前一日 CCI >= 超卖线, 当日 CCI < 超卖线
		if result.CCI[i] < oversold && result.CCI[i-1] >= oversold {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到CCI下穿超卖线(%.0f)", start, end, oversold),
	}
}

// ============================================================================
//  SignalCciTopDivergence — 03 顶背离
//
//  判定规则:
//    股价一峰比一峰高（创新高），但 CCI 一峰比一峰低（未创新高）。
//    一般是股价高位即将反转下跌的卖出信号。
//
//  实现方式:
//    复用 findPeaks / checkDivergence（macd.go 同包公共函数），
//    使用 CCI 值替代 MACD 柱。
// ============================================================================

type SignalCciTopDivergence struct {
	indicator.BaseSignal
}

func NewSignalCciTopDivergence() *SignalCciTopDivergence {
	return &SignalCciTopDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"顶背离",
			"价格创新高但CCI未创新高（股价高位反转信号）",
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

func (s *SignalCciTopDivergence) Evaluate(result CCIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.CCI, idxStart, idxEnd, peakWindow, true); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}

// ============================================================================
//  SignalCciBottomDivergence — 04 底背离
//
//  判定规则:
//    股价一底比一底低（创新低），但 CCI 一底比一底高（未创新低）。
//    一般是股价低位可能反弹向上的买入信号。
//
//  实现方式:
//    与顶背离对称，使用 isTop=false
// ============================================================================

type SignalCciBottomDivergence struct {
	indicator.BaseSignal
}

func NewSignalCciBottomDivergence() *SignalCciBottomDivergence {
	return &SignalCciBottomDivergence{
		BaseSignal: indicator.NewBaseSignal(
			"04",
			"底背离",
			"价格创新低但CCI未创新低（股价低位反弹信号）",
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

func (s *SignalCciBottomDivergence) Evaluate(result CCIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 60))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	peakWindow := int(config.GetFloat64(paramPeakWindow, float64(defaultPeakWindow)))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.ClosePrice))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	if ok, msg := checkDivergence(result.ClosePrice, result.CCI, idxStart, idxEnd, peakWindow, false); ok {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID, Message: msg}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: msg}
	}
}
