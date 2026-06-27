package technical

import (
	"fmt"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  ChanLongShort — 缠论多空指标
//  ID: 01102 = CatCodeTechnical("01") + IndLongShortSeq("102")
//  数据源: GetDailyKline()
//
//  包含8种信号:
//    BuySignals (4个):
//      1. 准备买入 — TrendLine < 11 (超卖区域)
//      2. 等待底分型 — 持续超卖 + 收盘价低于中线
//      3. 下单买入 — 趋势线从超卖区回升 + 收盘价低于中线
//      4. ★买多 — 趋势线上穿11 + 收盘价低于中线
//    SellSignals (4个):
//      5. 准备卖出 — TrendLine > 89 (超买区域)
//      6. 等待顶分型 — 持续超买 + 收盘价高于中线
//      7. 下单卖出 — 趋势线从超买区回落 + 收盘价高于中线
//      8. ★沽空 — 趋势线下穿89 + 收盘价高于中线
//
//  参考通达信"缠论多空"公式，一次计算共享给全部8个信号。
// ============================================================================

const (
	longShortMinKlineLen = 60 // 最小K线根数（55天计算需求 + 缓冲）
	longShortTopLevel    = 89 // 超买阈值
	longShortBottomLevel = 11 // 超卖阈值
	longShortFilterDays  = 15 // FILTER 持续天数
)

// longShortResult 缠论多空预计算结果，供该指标下所有信号复用
type longShortResult struct {
	TrendLine  []float64 // 趋势线 (EMA(V11, 3))
	CenterLine []float64 // 中线
	// 信号数组：100 = 触发，0 = 未触发（oldest-first 顺序）
	PrepareBuy  []float64
	ReadyBuy    []float64
	ActualBuy   []float64
	BuyStars    []float64
	PrepareSell []float64
	ReadySell   []float64
	ActualSell  []float64
	SellStars   []float64
}

// ChanLongShort 缠论多空指标
type ChanLongShort struct {
	indicator.BaseIndicator
}

func NewChanLongShort() *ChanLongShort {
	i := &ChanLongShort{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndLongShortSeq,
			NameStr:     "缠论多空",
			CategoryVal: indicator.CatTechnical,
			Desc:        "缠论多空指标，基于动态支撑阻力计算趋势线，提供准备/等待/下单/星级四级买卖信号",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		// BuySignals (4个)
		NewSignalLongShortPrepareBuy(),
		NewSignalLongShortReadyBuy(),
		NewSignalLongShortActualBuy(),
		NewSignalLongShortBuyStars(),
		// SellSignals (4个)
		NewSignalLongShortPrepareSell(),
		NewSignalLongShortReadySell(),
		NewSignalLongShortActualSell(),
		NewSignalLongShortSellStars(),
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate 缠论多空指标评估入口
//
//  1. 统一计算 TrendLine / CenterLine 及全部8个信号点位
//  2. 将结果分发给各信号
func (i *ChanLongShort) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < longShortMinKlineLen {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少%d根，当前%d根", longShortMinKlineLen, len(klines)),
		}
	}

	result := calcLongShort(klines)

	for _, v := range config {
		if s, ok := i.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *SignalLongShortPrepareBuy:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalLongShortReadyBuy:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalLongShortActualBuy:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalLongShortBuyStars:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalLongShortPrepareSell:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalLongShortReadySell:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalLongShortActualSell:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalLongShortSellStars:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
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

// ============================================================================
//  缠论多空核心计算（私有函数）
//
//  算法来源: 通达信"缠论多空"公式
//  klines[0]=最新，计算时内部反转为 oldest-first 后再套用原公式
// ============================================================================

func calcLongShort(klines []*model.DailyKline) *longShortResult {
	n := len(klines)

	// 反转为 oldest-first 顺序（klines[0]=最新 → data[0]=最旧）
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	opens := make([]float64, n)
	for i := 0; i < n; i++ {
		k := klines[n-1-i]
		closes[i] = float64(k.Close)
		highs[i] = float64(k.High)
		lows[i] = float64(k.Low)
		opens[i] = float64(k.Open)
	}

	// 1. 计算支撑阻力线（基于最新 K 线的动态价格范围）
	// H1 = MAX(Open, High), L1 = MIN(Open, Low), P1 = H1 - L1
	latestOpen := opens[n-1]
	latestHigh := highs[n-1]
	latestLow := lows[n-1]
	h1 := maxFloat64(latestOpen, latestHigh)
	l1 := minVal(latestOpen, latestLow)
	p1 := h1 - l1

	// 阻力: L1 + P1 * 7/8, 支撑: L1 + P1 * 0.5/8, 中线: 均值
	resistance := l1 + p1*7/8
	support := l1 + p1*0.5/8
	center := (support + resistance) / 2

	centerLine := make([]float64, n)
	for i := 0; i < n; i++ {
		centerLine[i] = center
	}

	// 2. 计算趋势线
	// V11 = 3*SMA(raw,5,1) - 2*SMA(SMA(raw,5,1),3,1)
	// raw = (Close-LLV(Low,55)) / (HHV(High,55)-LLV(Low,55)) * 100
	llv55 := llv(lows, 55)
	hhv55 := hhv(highs, 55)
	rawValue := make([]float64, n)
	for i := 0; i < n; i++ {
		if hhv55[i] != llv55[i] {
			rawValue[i] = (closes[i] - llv55[i]) / (hhv55[i] - llv55[i]) * 100
		} else {
			rawValue[i] = 50
		}
	}
	sma1 := sma(rawValue, 5, 1)
	sma2 := sma(sma1, 3, 1)
	v11 := make([]float64, n)
	for i := 0; i < n; i++ {
		v11[i] = 3*sma1[i] - 2*sma2[i]
	}

	// TrendLine = EMA(V11, 3)
	trendLine := ema(v11, 3)

	// 3. 计算全部8个信号点位
	result := &longShortResult{
		TrendLine:   trendLine,
		CenterLine:  centerLine,
		PrepareBuy:  make([]float64, n),
		ReadyBuy:    make([]float64, n),
		ActualBuy:   make([]float64, n),
		BuyStars:    make([]float64, n),
		PrepareSell: make([]float64, n),
		ReadySell:   make([]float64, n),
		ActualSell:  make([]float64, n),
		SellStars:   make([]float64, n),
	}

	// --- 买入侧信号 ---

	// PrepareBuy: 趋势线 < 11
	for i := 0; i < n; i++ {
		if trendLine[i] < longShortBottomLevel {
			result.PrepareBuy[i] = 100
		}
	}

	// ReadyBuy: (趋势线<11) AND FILTER(趋势线<=11, 15) AND Close < 中线
	for i := longShortFilterDays - 1; i < n; i++ {
		trend := trendLine[i]
		if trend >= longShortBottomLevel {
			continue
		}
		// 检查连续15周期趋势线均 <= 11
		filterOK := true
		for j := i - (longShortFilterDays - 1); j <= i; j++ {
			if trendLine[j] > longShortBottomLevel {
				filterOK = false
				break
			}
		}
		if filterOK && closes[i] < centerLine[i] {
			result.ReadyBuy[i] = 100
		}
	}

	// ActualBuy + BuyStars: 需要 REF(TrendLine, 1)
	refTrend := ref(trendLine, 1)
	for i := 1; i < n; i++ {
		trend := trendLine[i]
		prevTrend := refTrend[i]

		// ★买多 (BuyStars): REF(趋势线,1)<11 AND CROSS(趋势线,11) AND Close<中线
		if prevTrend < longShortBottomLevel &&
			prevTrend <= longShortBottomLevel && trend > longShortBottomLevel &&
			closes[i] < centerLine[i] {
			result.BuyStars[i] = 100
		}

		// ActualBuy: BB1~BB5 条件
		bb1 := prevTrend < longShortBottomLevel && prevTrend > 6 && trend > longShortBottomLevel
		bb2 := prevTrend < 6 && prevTrend > 3 && trend > 6
		bb3 := prevTrend < 3 && prevTrend > 1 && trend > 3
		bb4 := prevTrend < 1 && prevTrend > 0 && trend > 1
		bb5 := prevTrend < 0 && trend > 0
		if (bb1 || bb2 || bb3 || bb4 || bb5) && closes[i] < centerLine[i] {
			result.ActualBuy[i] = 100
		}
	}

	// --- 卖出侧信号 ---

	// PrepareSell: 趋势线 > 89
	for i := 0; i < n; i++ {
		if trendLine[i] > longShortTopLevel {
			result.PrepareSell[i] = 100
		}
	}

	// ReadySell: (趋势线>89) AND FILTER(趋势线>89, 15) AND Close > 中线
	for i := longShortFilterDays - 1; i < n; i++ {
		trend := trendLine[i]
		if trend <= longShortTopLevel {
			continue
		}
		filterOK := true
		for j := i - (longShortFilterDays - 1); j <= i; j++ {
			if trendLine[j] <= longShortTopLevel {
				filterOK = false
				break
			}
		}
		if filterOK && closes[i] > centerLine[i] {
			result.ReadySell[i] = 100
		}
	}

	// ActualSell + SellStars
	for i := 1; i < n; i++ {
		trend := trendLine[i]
		prevTrend := refTrend[i]

		// ★沽空 (SellStars): REF(趋势线,1)>89 AND CROSS(89,趋势线) AND Close>中线
		if prevTrend > longShortTopLevel &&
			prevTrend >= longShortTopLevel && trend < longShortTopLevel &&
			closes[i] > centerLine[i] {
			result.SellStars[i] = 100
		}

		// ActualSell: DD1~DD5 条件
		dd1 := prevTrend > longShortTopLevel && prevTrend < 94 && trend < longShortTopLevel
		dd2 := prevTrend > 94 && prevTrend < 97 && trend < 94
		dd3 := prevTrend > 97 && prevTrend < 99 && trend < 97
		dd4 := prevTrend > 99 && prevTrend < 100 && trend < 99
		dd5 := prevTrend > 100 && trend < 100
		if (dd1 || dd2 || dd3 || dd4 || dd5) && closes[i] > centerLine[i] {
			result.ActualSell[i] = 100
		}
	}

	return result
}

// minVal 返回两个float64中的较小值
func minVal(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
//  SignalLongShortPrepareBuy — 准备买入 (Seq: 01 → 01102001)
//
//  判定: TrendLine < 11（超卖区域），近 lookback_days 日内出现即通过
// ============================================================================

type SignalLongShortPrepareBuy struct {
	indicator.BaseSignal
}

func NewSignalLongShortPrepareBuy() *SignalLongShortPrepareBuy {
	return &SignalLongShortPrepareBuy{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"准备买入",
			"准备买入", // 趋势线进入超卖区域(TrendLine<11)，可准备买入
			indicator.ValNumber,
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

func (s *SignalLongShortPrepareBuy) Evaluate(r *longShortResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	return evalLongShortSignal(r.PrepareBuy, "准备买入", config)
}

// ============================================================================
//  SignalLongShortReadyBuy — 等待底分型 (Seq: 02 → 01102002)
//
//  判定: TrendLine<11 且连续15日<=11 且 Close<中线，近 lookback_days 日内出现即通过
// ============================================================================

type SignalLongShortReadyBuy struct {
	indicator.BaseSignal
}

func NewSignalLongShortReadyBuy() *SignalLongShortReadyBuy {
	return &SignalLongShortReadyBuy{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"等待底分型",
			"等待底分型", // 趋势线持续超卖15日以上且收盘价低于中线，需等待底部形态确认
			indicator.ValNumber,
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

func (s *SignalLongShortReadyBuy) Evaluate(r *longShortResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	return evalLongShortSignal(r.ReadyBuy, "等待底分型", config)
}

// ============================================================================
//  SignalLongShortActualBuy — 下单买入 (Seq: 03 → 01102003)
//
//  判定: 趋势线从超卖区逐级回升(BB1~BB5) 且 收盘价<中线
// ============================================================================

type SignalLongShortActualBuy struct {
	indicator.BaseSignal
}

func NewSignalLongShortActualBuy() *SignalLongShortActualBuy {
	return &SignalLongShortActualBuy{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"下单买入",
			"下单买入", // 趋势线从超卖区逐级回升且收盘价仍低于中线，可下单买入
			indicator.ValNumber,
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

func (s *SignalLongShortActualBuy) Evaluate(r *longShortResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	return evalLongShortSignal(r.ActualBuy, "下单买入", config)
}

// ============================================================================
//  SignalLongShortBuyStars — ★买多 (Seq: 04 → 01102004)
//
//  判定: TrendLine 上穿 11（金叉）且 收盘价<中线
// ============================================================================

type SignalLongShortBuyStars struct {
	indicator.BaseSignal
}

func NewSignalLongShortBuyStars() *SignalLongShortBuyStars {
	return &SignalLongShortBuyStars{
		BaseSignal: indicator.NewBaseSignal(
			"04",
			"★买多",
			"★买多", // 趋势线上穿11（金叉）且收盘价低于中线，明确的买入信号
			indicator.ValNumber,
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

func (s *SignalLongShortBuyStars) Evaluate(r *longShortResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	return evalLongShortSignal(r.BuyStars, "★买多", config)
}

// ============================================================================
//  SignalLongShortPrepareSell — 准备卖出 (Seq: 05 → 01102005)
//
//  判定: TrendLine > 89（超买区域）
// ============================================================================

type SignalLongShortPrepareSell struct {
	indicator.BaseSignal
}

func NewSignalLongShortPrepareSell() *SignalLongShortPrepareSell {
	return &SignalLongShortPrepareSell{
		BaseSignal: indicator.NewBaseSignal(
			"05",
			"准备卖出",
			"准备卖出", // 趋势线进入超买区域(TrendLine>89)，可准备卖出
			indicator.ValNumber,
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

func (s *SignalLongShortPrepareSell) Evaluate(r *longShortResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	return evalLongShortSignal(r.PrepareSell, "准备卖出", config)
}

// ============================================================================
//  SignalLongShortReadySell — 等待顶分型 (Seq: 06 → 01102006)
//
//  判定: TrendLine>89 且连续15日>89 且 Close>中线
// ============================================================================

type SignalLongShortReadySell struct {
	indicator.BaseSignal
}

func NewSignalLongShortReadySell() *SignalLongShortReadySell {
	return &SignalLongShortReadySell{
		BaseSignal: indicator.NewBaseSignal(
			"06",
			"等待顶分型",
			"等待顶分型", // 趋势线持续超买15日以上且收盘价高于中线，需等待顶部形态确认
			indicator.ValNumber,
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

func (s *SignalLongShortReadySell) Evaluate(r *longShortResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	return evalLongShortSignal(r.ReadySell, "等待顶分型", config)
}

// ============================================================================
//  SignalLongShortActualSell — 下单卖出 (Seq: 07 → 01102007)
//
//  判定: 趋势线从超买区逐级回落(DD1~DD5) 且 收盘价>中线
// ============================================================================

type SignalLongShortActualSell struct {
	indicator.BaseSignal
}

func NewSignalLongShortActualSell() *SignalLongShortActualSell {
	return &SignalLongShortActualSell{
		BaseSignal: indicator.NewBaseSignal(
			"07",
			"下单卖出",
			"下单卖出", // 趋势线从超买区逐级回落且收盘价仍高于中线，可下单卖出
			indicator.ValNumber,
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

func (s *SignalLongShortActualSell) Evaluate(r *longShortResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	return evalLongShortSignal(r.ActualSell, "下单卖出", config)
}

// ============================================================================
//  SignalLongShortSellStars — ★沽空 (Seq: 08 → 01102008)
//
//  判定: TrendLine 下穿 89（死叉）且 收盘价>中线
// ============================================================================

type SignalLongShortSellStars struct {
	indicator.BaseSignal
}

func NewSignalLongShortSellStars() *SignalLongShortSellStars {
	return &SignalLongShortSellStars{
		BaseSignal: indicator.NewBaseSignal(
			"08",
			"★沽空",
			"★沽空", // 趋势线下穿89（死叉）且收盘价高于中线，明确的卖出信号
			indicator.ValNumber,
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

func (s *SignalLongShortSellStars) Evaluate(r *longShortResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	return evalLongShortSignal(r.SellStars, "★沽空", config)
}

// ============================================================================
//  evalLongShortSignal — 8个信号的公共评估逻辑
//
//  检查信号数组（100=触发）在指定窗口内是否有触发点。
//  start/end 均为"N天前"；若 start < end 则自动交换（容错）。
// ============================================================================

func evalLongShortSignal(signalData []float64, signalName string, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(signalData))
	if err != nil {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  err.Error(),
		}
	}

	for i := idxStart; i < idxEnd; i++ {
		if signalData[i] == 100 {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}
	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未出现%s信号", start, end, signalName),
	}
}
