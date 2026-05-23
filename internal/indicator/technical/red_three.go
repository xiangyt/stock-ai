package technical

import (
	"fmt"

	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
)

// ============================================================================
//  RedThree — 红三角抄底指标
//  ID: 01101 = CatCodeTechnical("01") + IndRedThreeSeq("101")
//  数据源: GetDailyKline()
//
//  包含3种信号:
//    1. 买入 — QUSHI 上穿 JIANDI(1)，出现金叉
//    2. 大底 — 价格严重偏离均线（MA5 < Close*0.96 且 MA10 < MA5*0.96）
//    3. 警告 — QUSHI 下穿 100，出现死叉
//
//  计算逻辑参考通达信"红三角"公式，一次计算共享给全部3个信号。
// ============================================================================

const (
	redThreeMinKlineLen = 60 // 最小K线根数（超过33天计算需求，留足缓冲）
)

// redThreeResult 红三角指标预计算结果，供该指标下所有信号复用
type redThreeResult struct {
	QUSHI []float64 // 趋势指标
	MAIRU []float64 // 买入信号 (100=触发)
	DADI  []float64 // 大底信号 (100=触发)
}

// RedThree 红三角指标
type RedThree struct {
	indicator.BaseIndicator
}

func NewRedThree() *RedThree {
	i := &RedThree{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndRedThreeSeq,
			NameStr:     "红三角",
			CategoryVal: indicator.CatTechnical,
			Desc:        "红三角抄底指标，包含买入（QUSHI金叉）、大底（价格偏离均线）、警告（QUSHI死叉）三种信号",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalRedThreeBuy(),
		NewSignalRedThreeDaDi(),
		NewSignalRedThreeWarning(),
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate 红三角指标评估入口
//
//  1. 统一计算 QUSHI / MAIRU / DADI
//  2. 将结果分发给各信号
func (i *RedThree) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < redThreeMinKlineLen {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少%d根，当前%d根", redThreeMinKlineLen, len(klines)),
		}
	}

	result := calcRedThree(klines)

	for _, v := range config {
		if s, ok := i.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *SignalRedThreeBuy:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalRedThreeDaDi:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalRedThreeWarning:
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
//  红三角核心计算（私有函数）
//
//  算法来源: 通达信"红三角"公式
//  klines[0]=最新，计算时内部反转为 oldest-first 后再套用原公式
// ============================================================================

func calcRedThree(klines []*model.DailyKline) *redThreeResult {
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

	// 1. VAR1 = REF((LOW+OPEN+CLOSE+HIGH)/4, 1)
	priceAvg := make([]float64, n)
	for i := 0; i < n; i++ {
		priceAvg[i] = (lows[i] + opens[i] + closes[i] + highs[i]) / 4
	}
	var1 := ref(priceAvg, 1)

	// 2. VAR2 = SMA(ABS(LOW-VAR1), 13, 1) / SMA(MAX(LOW-VAR1, 0), 10, 1)
	lowMinusVAR1 := make([]float64, n)
	for i := 0; i < n; i++ {
		lowMinusVAR1[i] = lows[i] - var1[i]
	}
	absLow := vecAbs(lowMinusVAR1)
	maxLow := make([]float64, n)
	for i := 0; i < n; i++ {
		maxLow[i] = maxFloat64(lowMinusVAR1[i], 0)
	}
	numerator := sma(absLow, 13, 1)
	denominator := sma(maxLow, 10, 1)
	var2 := make([]float64, n)
	for i := 0; i < n; i++ {
		if denominator[i] != 0 {
			var2[i] = numerator[i] / denominator[i]
		}
	}

	// 3. VAR3 = EMA(VAR2, 10)
	var3 := ema(var2, 10)

	// 4. VAR4 = LLV(LOW, 33)
	var4 := llv(lows, 33)

	// 5. QUSHI = 3*SMA(A,5,1) - 2*SMA(SMA(A,5,1),9,1)
	//    A = (CLOSE-LLV(LOW,27)) / (HHV(HIGH,27)-LLV(LOW,27)) * 100
	llv27 := llv(lows, 27)
	hhv27 := hhv(highs, 27)
	A := make([]float64, n)
	for i := 0; i < n; i++ {
		den := hhv27[i] - llv27[i]
		if den != 0 {
			A[i] = (closes[i] - llv27[i]) / den * 100
		}
	}
	smaA5 := sma(A, 5, 1)
	smaSmaA59 := sma(smaA5, 9, 1)
	qushi := make([]float64, n)
	for i := 0; i < n; i++ {
		qushi[i] = 3*smaA5[i] - 2*smaSmaA59[i]
	}

	// 6. MAIRU = IF(CROSS(QUSHI, JIANDI), 100, 0)  — JIANDI 恒为 1
	jiandi := make([]float64, n)
	for i := 0; i < n; i++ {
		jiandi[i] = 1
	}
	crossBuy := cross(qushi, jiandi)
	mairu := make([]float64, n)
	for i, c := range crossBuy {
		if c {
			mairu[i] = 100
		}
	}

	// 7. VAR5 = EMA(IF(LOW<=VAR4, VAR3, 0), 3)  — 未直接用于信号，用于完整性
	_ = var3
	_ = var4

	// 8. DADI = IF((MA(C,5)-C)/C > 0.04 AND (MA(C,10)-MA(C,5))/MA(C,5) > 0.04, 100, 0)
	ma5 := ma(closes, 5)
	ma10 := ma(closes, 10)
	dadi := make([]float64, n)
	for i := 0; i < n; i++ {
		cond1 := false
		if closes[i] != 0 {
			cond1 = (ma5[i]-closes[i])/closes[i] > 0.04
		}
		cond2 := false
		if ma5[i] != 0 {
			cond2 = (ma10[i]-ma5[i])/ma5[i] > 0.04
		}
		if cond1 && cond2 {
			dadi[i] = 100
		}
	}

	return &redThreeResult{
		QUSHI: qushi,
		MAIRU: mairu,
		DADI:  dadi,
	}
}

// ============================================================================
//  SignalRedThreeBuy — 买入信号
//
//  判定规则: QUSHI 上穿 JIANDI(1)，即 MAIRU == 100
//  参数: lookback_days — 回看天数（默认 1），在近 N 日内出现买入信号即通过
//
//  Seq: "01" → 完整内置SignalID = 01101001
// ============================================================================

const paramRedThreeBuyLookback = "lookback_days"

type SignalRedThreeBuy struct {
	indicator.BaseSignal
}

func NewSignalRedThreeBuy() *SignalRedThreeBuy {
	return &SignalRedThreeBuy{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"红三角",
			"红三角",
			indicator.ValNumber,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpRising,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						{Key: paramRedThreeBuyLookback, Label: "回看天数", Type: "number", Required: false, Default: 1, Min: 1, Max: 20, Unit: "日"},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpRising,
				Params: map[string]any{
					paramRedThreeBuyLookback: float64(1),
				},
			},
		),
	}
}

func (s *SignalRedThreeBuy) Evaluate(r *redThreeResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	lookback := int(config.GetFloat64(paramRedThreeBuyLookback, 1))
	n := len(r.MAIRU)

	// 检查最近 lookback 天（data[n-1]=最新）是否有买入信号
	start := n - lookback
	if start < 0 {
		start = 0
	}
	for i := start; i < n; i++ {
		if r.MAIRU[i] == 100 {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}
	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("近%d日内未出现QUSHI金叉买入信号", lookback),
	}
}

// ============================================================================
//  SignalRedThreeDaDi — 大底信号
//
//  判定规则: (MA5-Close)/Close > 4% 且 (MA10-MA5)/MA5 > 4%，即 DADI == 100
//  参数: lookback_days — 回看天数（默认 1），在近 N 日内出现大底信号即通过
//
//  Seq: "02" → 完整内置SignalID = 01101002
// ============================================================================

const paramRedThreeDaDiLookback = "lookback_days"

type SignalRedThreeDaDi struct {
	indicator.BaseSignal
}

func NewSignalRedThreeDaDi() *SignalRedThreeDaDi {
	return &SignalRedThreeDaDi{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"红箭头",
			"红箭头",
			indicator.ValNumber,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpRising,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						{Key: paramRedThreeDaDiLookback, Label: "回看天数", Type: "number", Required: false, Default: 1, Min: 1, Max: 20, Unit: "日"},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpRising,
				Params: map[string]any{
					paramRedThreeDaDiLookback: float64(1),
				},
			},
		),
	}
}

func (s *SignalRedThreeDaDi) Evaluate(r *redThreeResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	lookback := int(config.GetFloat64(paramRedThreeDaDiLookback, 1))
	n := len(r.DADI)

	start := n - lookback
	if start < 0 {
		start = 0
	}
	for i := start; i < n; i++ {
		if r.DADI[i] == 100 {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}
	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("近%d日内未出现大底信号", lookback),
	}
}

// ============================================================================
//  SignalRedThreeWarning — 警告信号
//
//  判定规则: 100 下穿 QUSHI（即 QUSHI 从 >=100 跌到 <100，出现死叉）
//  计算方式: CROSS(100, QUSHI)，即前一日 100 <= QUSHI 且 今日 100 > QUSHI
//  参数: lookback_days — 回看天数（默认 1），在近 N 日内出现警告信号即通过
//
//  Seq: "03" → 完整内置SignalID = 01101003
// ============================================================================

const paramRedThreeWarningLookback = "lookback_days"

type SignalRedThreeWarning struct {
	indicator.BaseSignal
}

func NewSignalRedThreeWarning() *SignalRedThreeWarning {
	return &SignalRedThreeWarning{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"注意",
			"注意",
			indicator.ValNumber,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpRising,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						{Key: paramRedThreeWarningLookback, Label: "回看天数", Type: "number", Required: false, Default: 1, Min: 1, Max: 20, Unit: "日"},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpRising,
				Params: map[string]any{
					paramRedThreeWarningLookback: float64(1),
				},
			},
		),
	}
}

func (s *SignalRedThreeWarning) Evaluate(r *redThreeResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	lookback := int(config.GetFloat64(paramRedThreeWarningLookback, 1))
	n := len(r.QUSHI)

	// 构造恒为 100 的数组，调用 CROSS(100, QUSHI) — 100 上穿 QUSHI，即 QUSHI 从 >=100 跌破 100
	hundred := make([]float64, n)
	for i := 0; i < n; i++ {
		hundred[i] = 100
	}
	warningCross := cross(hundred, r.QUSHI)

	start := n - lookback
	if start < 0 {
		start = 0
	}
	for i := start; i < n; i++ {
		if warningCross[i] {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}
	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("近%d日内未出现QUSHI死叉警告信号", lookback),
	}
}
