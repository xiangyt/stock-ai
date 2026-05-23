package technical

import (
	"fmt"

	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
)

// ============================================================================
//  TopBottom — 顶底信号指标
//  ID: 01100 = CatCodeTechnical("01") + IndTopBottomSeq("100")
//  数据源: GetDailyKline()
//
//  包含2种信号:
//    1. 红柱 — A > 5，出现红色柱状买入信号
//    2. 绿柱 — A1 > 0，出现绿色柱状卖出信号
//
//  计算逻辑参考通达信"顶底信号"公式，一次计算共享给全部信号。
//
//  算法核心:
//    - AS1 时间控制因子（2038年后归零）
//    - AS2~AS8 / AD2~AD8 八级递推计算支撑/阻力
//    - A = min(AS8, 100) * AS1  → 红柱线
//    - A1 = min(AD8, 50) * AS1   → 绿柱线
// ============================================================================

const (
	topBottomMinKlineLen = 58 // 最小K线根数（MA(58)计算需求）
)

// topBottomResult 顶底信号预计算结果，供该指标下所有信号复用
type topBottomResult struct {
	RedLine   []float64 // A — 红柱线（最大值100）
	GreenLine []float64 // A1 — 绿柱线（最大值50）
}

// TopBottom 顶底信号指标
type TopBottom struct {
	indicator.BaseIndicator
}

func NewTopBottom() *TopBottom {
	i := &TopBottom{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndTopBottomSeq,
			NameStr:     "顶底信号",
			CategoryVal: indicator.CatTechnical,
			Desc:        "顶底信号指标，基于八级时间控制递推算法计算支撑/阻力，提供红柱（买入）和绿柱（卖出）信号",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalTopBottomRed(),
		NewSignalTopBottomGreen(),
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate 顶底信号指标评估入口
//
//  1. 统一计算 RedLine / GreenLine
//  2. 将结果分发给各信号
func (i *TopBottom) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < topBottomMinKlineLen {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少%d根，当前%d根", topBottomMinKlineLen, len(klines)),
		}
	}

	result := calcTopBottom(klines)

	for _, v := range config {
		if s, ok := i.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *SignalTopBottomRed:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalTopBottomGreen:
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
//  顶底信号核心计算（私有函数）
//
//  算法来源: 通达信"顶底信号"公式
//  klines[0]=最新，计算时内部反转为 oldest-first 后再套用原公式
//
//  计算流程:
//    AS1（时间控制）→ AS2/AD2 → AS3/AD3 → ... → AS8/AD8 → A/A1
// ============================================================================

func calcTopBottom(klines []*model.DailyKline) *topBottomResult {
	n := len(klines)

	// 反转为 oldest-first 顺序（klines[0]=最新 → data[0]=最旧）
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	dates := make([]int, n)
	for i := 0; i < n; i++ {
		k := klines[n-1-i]
		closes[i] = float64(k.Close)
		highs[i] = float64(k.High)
		lows[i] = float64(k.Low)
		dates[i] = k.TradeDate
	}

	// ---- Step 1: AS1 时间控制因子 ----
	// 原公式: IF(YEAR>=2038 AND MONTH>=1, 0, 1)
	as1 := calcAS1(dates)

	// ---- Step 2: AS2 = REF(LOW,1)*AS1, AD2 = REF(HIGH,1)*AS1 ----
	refLow := ref(lows, 1)
	as2 := vecMul(refLow, as1)
	refHigh := ref(highs, 1)
	ad2 := vecMul(refHigh, as1)

	// ---- Step 3: AS3 = SMA(ABS(LOW-AS2),3,1)/SMA(MAX,3,1)*100*AS1 ----
	as3 := calcAsAd3(lows, as2, as1)
	// AD3 = SMA(ABS(HIGH-AD2),3,1)/SMA(MAX,3,1)*100*AS1
	ad3 := calcAsAd3(highs, ad2, as1)

	// ---- Step 4: AS4 = EMA(AS3*10,3)*AS1  (close*1.3 恒为正，取 *10 分支) ----
	as4input := vecScale(as3, 10)
	as4 := vecMul(ema(as4input, 3), as1)
	ad4input := vecScale(ad3, 10)
	ad4 := vecMul(ema(ad4input, 3), as1)

	// ---- Step 5: AS5 = LLV(LOW,30)*AS1, AD5 = HHV(HIGH,30)*AS1 ----
	as5 := vecMul(llv(lows, 30), as1)
	ad5 := vecMul(hhv(highs, 30), as1)

	// ---- Step 6: AS6 = HHV(AS4,30)*AS1, AD6 = LLV(AD4,30)*AS1 ----
	as6 := vecMul(hhv(as4, 30), as1)
	ad6 := vecMul(llv(ad4, 30), as1)

	// ---- Step 7: AS7 = IF(MA(C,58)>0,1,0)*AS1, AD7 同 ----
	ma58 := ma(closes, 58)
	as7 := make([]float64, n)
	for i := range as7 {
		if ma58[i] > 0 {
			as7[i] = 1 * as1[i]
		}
	}
	ad7 := as7

	// ---- Step 8: AS8 = EMA(IF(LOW<=AS5,(AS4+AS6*2)/2,0),3)/618*AS7*AS1 ----
	as8 := calcAsAd8(lows, as5, as4, as6, as7, as1, "low")
	// AD8 = EMA(IF(HIGH>=AD5,(AD4+AD6*2)/2,0),3)/618*AD7*AS1
	ad8 := calcAsAd8(highs, ad5, ad4, ad6, ad7, as1, "high")

	// ---- Step 9: A = IF(AS8>100,100,AS8)*AS1, A1 = IF(AD8>50,50,AD8)*AS1 ----
	redLine := vecCap(vecMul(as8, as1), 100)
	greenLine := vecCap(vecMul(ad8, as1), 50)

	return &topBottomResult{
		RedLine:   redLine,
		GreenLine: greenLine,
	}
}

// ============================================================================
//  顶底信号子计算函数（私有）
// ============================================================================

// calcAS1 时间控制因子: IF(YEAR>=2038 AND MONTH>=1, 0, 1)
func calcAS1(dates []int) []float64 {
	as1 := make([]float64, len(dates))
	for i, d := range dates {
		year := d / 10000
		if year >= 2038 {
			as1[i] = 0
		} else {
			as1[i] = 1
		}
	}
	return as1
}

// calcAsAd3 计算 AS3/AD3 的公共逻辑
// 公式: SMA(ABS(data-AS2), 3, 1) / SMA(MAX(data-AS2, 0), 3, 1) * 100 * AS1
func calcAsAd3(data, as2, as1 []float64) []float64 {
	n := len(data)
	diff := vecSub(data, as2)
	absDiff := vecAbs(diff)
	maxDiff := make([]float64, n)
	for i := range maxDiff {
		maxDiff[i] = maxFloat64(diff[i], 0)
	}

	num := sma(absDiff, 3, 1)
	den := sma(maxDiff, 3, 1)

	result := make([]float64, n)
	for i := range result {
		if den[i] != 0 {
			result[i] = (num[i] / den[i] * 100) * as1[i]
		}
	}
	return result
}

// calcAsAd8 计算 AS8/AD8 的公共逻辑
// mode="low":  IF(data <= line, (v4 + v6*2)/2, 0)
// mode="high": IF(data >= line, (v4 + v6*2)/2, 0)
// 然后 EMA(..., 3) / 618 * v7 * as1
func calcAsAd8(data, line, v4, v6, v7, as1 []float64, mode string) []float64 {
	n := len(data)
	input := make([]float64, n)
	for i := range input {
		trigger := false
		if mode == "low" {
			trigger = data[i] <= line[i]
		} else {
			trigger = data[i] >= line[i]
		}
		if trigger {
			input[i] = (v4[i] + v6[i]*2) / 2
		}
	}

	ema3 := ema(input, 3)
	result := make([]float64, n)
	for i := range result {
		result[i] = (ema3[i] / 618) * v7[i] * as1[i]
	}
	return result
}

// ============================================================================
//  向量辅助函数（私有，跨指标共享）
// ============================================================================

// vecMul 向量逐元素乘法
func vecMul(a, b []float64) []float64 {
	result := make([]float64, len(a))
	for i := range result {
		result[i] = a[i] * b[i]
	}
	return result
}

// vecSub 向量逐元素减法
func vecSub(a, b []float64) []float64 {
	result := make([]float64, len(a))
	for i := range result {
		result[i] = a[i] - b[i]
	}
	return result
}

// vecScale 向量逐元素缩放
func vecScale(a []float64, scale float64) []float64 {
	result := make([]float64, len(a))
	for i := range result {
		result[i] = a[i] * scale
	}
	return result
}

// vecCap 向量逐元素上限裁剪
func vecCap(a []float64, cap float64) []float64 {
	result := make([]float64, len(a))
	for i := range result {
		if a[i] > cap {
			result[i] = cap
		} else {
			result[i] = a[i]
		}
	}
	return result
}

// ============================================================================
//  SignalTopBottomRed — 红柱信号
//
//  判定规则: A > 5（红色柱状有效买入信号，参考原公式 Buy 阈值）
//  参数: lookback_days — 回看天数（默认 5），在近 N 日内出现红柱信号即通过
//
//  Seq: "01" → 完整内置SignalID = 01100001
// ============================================================================

const paramTopBottomRedLookback = "lookback_days"

type SignalTopBottomRed struct {
	indicator.BaseSignal
}

func NewSignalTopBottomRed() *SignalTopBottomRed {
	return &SignalTopBottomRed{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"红柱",
			"顶底信号红柱，A>5 出现红色柱状买入信号",
			indicator.ValNumber,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpRising,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						{Key: paramTopBottomRedLookback, Label: "回看天数", Type: "number", Required: false, Default: 5, Min: 1, Max: 20, Unit: "日"},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpRising,
				Params: map[string]any{
					paramTopBottomRedLookback: float64(5),
				},
			},
		),
	}
}

func (s *SignalTopBottomRed) Evaluate(r *topBottomResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	lookback := int(config.GetFloat64(paramTopBottomRedLookback, 5))
	n := len(r.RedLine)

	start := n - lookback
	if start < 0 {
		start = 0
	}
	for i := start; i < n; i++ {
		if r.RedLine[i] > 5 {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}
	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("近%d日内未出现红柱信号（A>5）", lookback),
	}
}

// ============================================================================
//  SignalTopBottomGreen — 绿柱信号
//
//  判定规则: A1 > 0（绿色柱状出现）
//  参数: lookback_days — 回看天数（默认 5），在近 N 日内出现绿柱信号即通过
//
//  Seq: "02" → 完整内置SignalID = 01100002
// ============================================================================

const paramTopBottomGreenLookback = "lookback_days"

type SignalTopBottomGreen struct {
	indicator.BaseSignal
}

func NewSignalTopBottomGreen() *SignalTopBottomGreen {
	return &SignalTopBottomGreen{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"绿柱",
			"顶底信号绿柱，A1>0 出现绿色柱状卖出信号",
			indicator.ValNumber,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpRising,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						{Key: paramTopBottomGreenLookback, Label: "回看天数", Type: "number", Required: false, Default: 5, Min: 1, Max: 20, Unit: "日"},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpRising,
				Params: map[string]any{
					paramTopBottomGreenLookback: float64(5),
				},
			},
		),
	}
}

func (s *SignalTopBottomGreen) Evaluate(r *topBottomResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	lookback := int(config.GetFloat64(paramTopBottomGreenLookback, 5))
	n := len(r.GreenLine)

	start := n - lookback
	if start < 0 {
		start = 0
	}
	for i := start; i < n; i++ {
		if r.GreenLine[i] > 0 {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}
	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("近%d日内未出现绿柱信号（A1>0）", lookback),
	}
}
