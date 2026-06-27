package technical

import (
	"fmt"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  RedTop — 红顶底指标
//  ID: 01103 = CatCodeTechnical("01") + IndRedTopSeq("103")
//  数据源: GetDailyKline()
//
//  包含6种信号（参考原公式注释中标记√的信号）:
//    1. 极底 — Info 三连阴转阳 + 平均线 < 0.5
//    2. 升   — Info 三连阴转阳 + 走强逆转 + 放量
//    3. 见底 — 跳空低开后高走，假阴线形态
//    4. 绝底 — 收阳 + O/L > 1.05 + 价格创 20 日新低
//    5. 见涨 — HXN 事件 + 放量上涨 + 30 日首次出现
//    6. 金叉 — MACD 金叉 + KDJ 金叉 双重确认
//
//  计算逻辑参考通达信"红顶"公式，一次计算共享给全部6个信号。
//  仅计算标记 √ 的信号，跳过标记 ×/? 的信号以减少计算量。
// ============================================================================

const (
	redTopMinKlineLen = 38 // 最小K线根数（RSI8/MA20/MA10 最大值）
)

// redTopResult 红顶指标预计算结果，供该指标下所有信号复用
type redTopResult struct {
	// Basic lines (oldest-first)
	MinValue    []float64 // LLV(LOW, 10)
	MaxValue    []float64 // HHV(HIGH, 25)
	WaveLine    []float64 // EMA((C-Min)/(Max-Min)*4, 4)
	AverageLine []float64 // EMA(WaveLine, 3)
	Info        []bool    // AverageLine >= REF(AverageLine, 1)
	Strengthen  []bool    // C > MA20 && C > MA5
	VolumeSig   []bool    // Volume > MA(Volume, 5)

	// Special computations
	LLV20 []float64 // LLV(LOW, 20) — for AbsoluteBottom
	HXN   []float64 // 91 or 0 — for SeeRise
	// MACD
	DIF []float64
	DEA []float64

	// KDJ
	K  []float64
	D_ []float64

	// Raw prices (oldest-first) for pattern signals
	Closes  []float64
	Opens   []float64
	Highs   []float64
	Lows    []float64
	Volumes []float64
}

// RedTop 红顶指标
type RedTop struct {
	indicator.BaseIndicator
}

func NewRedTop() *RedTop {
	i := &RedTop{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndRedTopSeq,
			NameStr:     "红顶底",
			CategoryVal: indicator.CatTechnical,
			Desc:        "红顶底指标，综合基本线、MACD、KDJ 多维度分析，提供极底/升/见底/绝底/见涨/金叉六种交易信号",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignalRedTopExtremeBottom(),
		NewSignalRedTopRise(),
		NewSignalRedTopBottom(),
		NewSignalRedTopAbsoluteBottom(),
		NewSignalRedTopSeeRise(),
		NewSignalRedTopGoldenCross(),
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate 红顶指标评估入口
func (i *RedTop) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < redTopMinKlineLen {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少%d根，当前%d根", redTopMinKlineLen, len(klines)),
		}
	}

	result := calcRedTop(klines)

	for _, v := range config {
		if s, ok := i.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *SignalRedTopExtremeBottom:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalRedTopRise:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalRedTopBottom:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalRedTopAbsoluteBottom:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalRedTopSeeRise:
				if res := vv.Evaluate(result, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			case *SignalRedTopGoldenCross:
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
//  红顶核心计算（私有函数）
//
//  仅计算 6 个 √ 标记信号所需数据，跳过以下中间计算：
//    — RSI5 / ADX / WR10 / BestBuy（供"建仓"信号，标记？）
//    — RiskCoefficient（供"抄底"信号，缺失）
//    — Escape（供"逃"信号，标记？）
//    — MustRise（供"必涨"信号，标记×）
//
//  klines[0]=最新 → 内部反转为 oldest-first
// ============================================================================

func calcRedTop(klines []*model.DailyKline) *redTopResult {
	n := len(klines)

	// 反转 klines[0]=最新 → data[0]=最旧
	closes := make([]float64, n)
	opens := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	volumes := make([]float64, n)
	for i := 0; i < n; i++ {
		k := klines[n-1-i]
		closes[i] = float64(k.Close)
		opens[i] = float64(k.Open)
		highs[i] = float64(k.High)
		lows[i] = float64(k.Low)
		volumes[i] = float64(k.Volume)
	}

	r := &redTopResult{
		Closes:  closes,
		Opens:   opens,
		Highs:   highs,
		Lows:    lows,
		Volumes: volumes,
	}

	// ---- 基本线 ----
	calcRedTopBasics(r, n)

	// ---- MACD / KDJ ----
	calcRedTopMACD(r, closes, n)
	calcRedTopKDJ(r, highs, lows, closes, n)

	// ---- LLV20 用于绝底 ----
	r.LLV20 = llv(lows, 20)

	// ---- HXN 用于见涨 ----
	r.HXN = calcRedTopHXN(closes, highs)

	return r
}

// calcRedTopBasics — 基本线计算
func calcRedTopBasics(r *redTopResult, n int) {
	// MinValue = LLV(LOW, 10)
	r.MinValue = llv(r.Lows, 10)
	// MaxValue = HHV(HIGH, 25)
	r.MaxValue = hhv(r.Highs, 25)

	// WaveLine = EMA((CLOSE - MinValue) / (MaxValue - MinValue) * 4, 4)
	waveRaw := make([]float64, n)
	for i := range waveRaw {
		den := r.MaxValue[i] - r.MinValue[i]
		if den != 0 {
			waveRaw[i] = (r.Closes[i] - r.MinValue[i]) / den * 4
		}
	}
	r.WaveLine = ema(waveRaw, 4)

	// AverageLine = EMA(WaveLine, 3)
	r.AverageLine = ema(r.WaveLine, 3)

	// Info = AverageLine >= REF(AverageLine, 1)
	refAvg := ref(r.AverageLine, 1)
	r.Info = make([]bool, n)
	for i := 1; i < n; i++ {
		r.Info[i] = r.AverageLine[i] >= refAvg[i]
	}

	// Strengthen = Close > MA(Close, 20) && Close > MA(Close, 5)
	ma5 := ma(r.Closes, 5)
	ma20 := ma(r.Closes, 20)
	r.Strengthen = make([]bool, n)
	for i := range r.Strengthen {
		r.Strengthen[i] = r.Closes[i] > ma20[i] && r.Closes[i] > ma5[i]
	}

	// VolumeSignal = Volume > MA(Volume, 5)
	maVol5 := ma(r.Volumes, 5)
	r.VolumeSig = make([]bool, n)
	for i := range r.VolumeSig {
		r.VolumeSig[i] = r.Volumes[i] > maVol5[i]
	}
}

// calcRedTopMACD — MACD 计算 (DIF, DEA)
func calcRedTopMACD(r *redTopResult, closes []float64, n int) {
	ema12 := ema(closes, 12)
	ema26 := ema(closes, 26)
	r.DIF = make([]float64, n)
	for i := range r.DIF {
		r.DIF[i] = (ema12[i] - ema26[i]) * 100
	}
	r.DEA = ema(r.DIF, 9)
}

// calcRedTopKDJ — KDJ 计算 (K, D)
func calcRedTopKDJ(r *redTopResult, highs, lows, closes []float64, n int) {
	llv9 := llv(lows, 9)
	hhv9 := hhv(highs, 9)
	rsv := make([]float64, n)
	for i := range rsv {
		den := hhv9[i] - llv9[i]
		if den != 0 {
			rsv[i] = (closes[i] - llv9[i]) / den * 100
		} else {
			rsv[i] = 50
		}
	}
	r.K = sma(rsv, 9, 3)
	r.D_ = sma(r.K, 9, 3)
}

// calcRedTopHXN — 计算 HXN 指标
// HXN: IF(CLOSE/REF(CLOSE,1)>1.05 AND HIGH/CLOSE<1.01 AND CLOSE>REF(CLOSE,1), 91, 0)
func calcRedTopHXN(closes, highs []float64) []float64 {
	n := len(closes)
	hxn := make([]float64, n)
	refCloses := ref(closes, 1)

	for i := 1; i < n; i++ {
		c1 := refCloses[i] != 0 && closes[i]/refCloses[i] > 1.05
		c2 := closes[i] != 0 && highs[i]/closes[i] < 1.01
		c3 := closes[i] > refCloses[i]
		if c1 && c2 && c3 {
			hxn[i] = 91
		}
	}
	return hxn
}

// ============================================================================
//  evalRedTopSignal — 公共信号评估函数
//  在 [start, end) 窗口内，signals[i]==100 则通过
//  start/end 均为"N天前"，对应 data 数组索引 [n-start, n-end)
//  若 start < end 则自动交换（容错）。
// ============================================================================

func evalRedTopSignal(signals []float64, config *indicator.SignalConfig, label string) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(signals))
	if err != nil {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  err.Error(),
		}
	}

	for i := idxStart; i < idxEnd; i++ {
		if signals[i] == 100 {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
		}
	}
	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未出现%s信号", start, end, label),
	}
}

// ============================================================================
//  SignalRedTopExtremeBottom — 极底信号 (Seq: 01 → 01103001)
// ============================================================================

type SignalRedTopExtremeBottom struct {
	indicator.BaseSignal
}

func NewSignalRedTopExtremeBottom() *SignalRedTopExtremeBottom {
	return &SignalRedTopExtremeBottom{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"极底",
			"极底",
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
				Params:   map[string]any{indicator.ParamKeyLookbackStart: float64(0), indicator.ParamKeyLookbackEnd: float64(0)},
			},
		),
	}
}

func (s *SignalRedTopExtremeBottom) Evaluate(r *redTopResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	n := len(r.Info)
	ref1 := refBool(r.Info, 1)
	ref2 := refBool(r.Info, 2)
	ref3 := refBool(r.Info, 3)

	signals := make([]float64, n)
	for i := 3; i < n; i++ {
		if r.Info[i] && !ref1[i] && !ref2[i] && !ref3[i] && r.AverageLine[i] < 0.5 {
			signals[i] = 100
		}
	}
	return evalRedTopSignal(signals, config, "极底")
}

// ============================================================================
//  SignalRedTopRise — 升信号 (Seq: 02 → 01103002)
// ============================================================================

type SignalRedTopRise struct {
	indicator.BaseSignal
}

func NewSignalRedTopRise() *SignalRedTopRise {
	return &SignalRedTopRise{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"升",
			"升", // Info三连阴转阳+走强逆转+放量，出现启动上升信号
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
				Params:   map[string]any{indicator.ParamKeyLookbackStart: float64(0), indicator.ParamKeyLookbackEnd: float64(0)},
			},
		),
	}
}

func (s *SignalRedTopRise) Evaluate(r *redTopResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	n := len(r.Info)
	refInfo1 := refBool(r.Info, 1)
	refInfo2 := refBool(r.Info, 2)
	refInfo3 := refBool(r.Info, 3)
	refStrengthen1 := refBool(r.Strengthen, 1)

	signals := make([]float64, n)
	for i := 3; i < n; i++ {
		if r.Info[i] && !refInfo1[i] && !refInfo2[i] && !refInfo3[i] &&
			r.Strengthen[i] && !refStrengthen1[i] && r.VolumeSig[i] {
			signals[i] = 100
		}
	}
	return evalRedTopSignal(signals, config, "升")
}

// ============================================================================
//  SignalRedTopBottom — 见底信号 (Seq: 03 → 01103003)
// ============================================================================

const redTopBottomLowThreshold = 688.0 // REF(L,1) 最低价阈值

type SignalRedTopBottom struct {
	indicator.BaseSignal
}

func NewSignalRedTopBottom() *SignalRedTopBottom {
	return &SignalRedTopBottom{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"见底",
			"见底", // 跳空低开假阴线+前日跳空高开，出现见底反转信号
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
				Params:   map[string]any{indicator.ParamKeyLookbackStart: float64(0), indicator.ParamKeyLookbackEnd: float64(0)},
			},
		),
	}
}

func (s *SignalRedTopBottom) Evaluate(r *redTopResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	n := len(r.Closes)
	refOpens := ref(r.Opens, 1)
	refCloses := ref(r.Closes, 1)
	refLows := ref(r.Lows, 1)

	signals := make([]float64, n)
	for i := 1; i < n; i++ {
		c1 := refCloses[i] != 0 && refOpens[i]/refCloses[i] > 1.04
		c2 := refLows[i] <= redTopBottomLowThreshold
		c3 := r.Opens[i] > refCloses[i]
		c4 := r.Closes[i] < refOpens[i]
		c5 := r.Opens[i] != 0 && r.Closes[i]/r.Opens[i] >= 1.01
		if c1 && c2 && c3 && c4 && c5 {
			signals[i] = 100
		}
	}
	return evalRedTopSignal(signals, config, "见底")
}

// ============================================================================
//  SignalRedTopAbsoluteBottom — 绝底信号 (Seq: 04 → 01103004)
// ============================================================================

type SignalRedTopAbsoluteBottom struct {
	indicator.BaseSignal
}

func NewSignalRedTopAbsoluteBottom() *SignalRedTopAbsoluteBottom {
	return &SignalRedTopAbsoluteBottom{
		BaseSignal: indicator.NewBaseSignal(
			"04",
			"绝底",
			"绝底", // 收阳+下影线>5%+价格创20日新低，出现绝对底部信号
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
				Params:   map[string]any{indicator.ParamKeyLookbackStart: float64(0), indicator.ParamKeyLookbackEnd: float64(0)},
			},
		),
	}
}

func (s *SignalRedTopAbsoluteBottom) Evaluate(r *redTopResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	n := len(r.Closes)

	signals := make([]float64, n)
	for i := 0; i < n; i++ {
		c1 := r.Closes[i] >= r.Opens[i]
		c2 := r.Lows[i] != 0 && r.Opens[i]/r.Lows[i] > 1.05
		c3 := r.Lows[i] <= r.LLV20[i]
		if c1 && c2 && c3 {
			signals[i] = 100
		}
	}
	return evalRedTopSignal(signals, config, "绝底")
}

// ============================================================================
//  SignalRedTopSeeRise — 见涨信号 (Seq: 05 → 01103005)
// ============================================================================

type SignalRedTopSeeRise struct {
	indicator.BaseSignal
}

func NewSignalRedTopSeeRise() *SignalRedTopSeeRise {
	return &SignalRedTopSeeRise{
		BaseSignal: indicator.NewBaseSignal(
			"05",
			"见涨",
			"见涨", // HXN事件+放量上涨+30日首次出现，出现见涨启动信号
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
				Params:   map[string]any{indicator.ParamKeyLookbackStart: float64(0), indicator.ParamKeyLookbackEnd: float64(0)},
			},
		),
	}
}

func (s *SignalRedTopSeeRise) Evaluate(r *redTopResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	n := len(r.Closes)
	refCloses := ref(r.Closes, 1)
	refVolumes := ref(r.Volumes, 1)

	signals := make([]float64, n)
	for i := 1; i < n; i++ {
		if r.HXN[i] <= 90 {
			continue
		}
		if !(r.Volumes[i] > refVolumes[i] && r.Closes[i] > refCloses[i]) {
			continue
		}
		count := 0
		start := i - 29
		if start < 0 {
			start = 0
		}
		for j := start; j <= i; j++ {
			if r.HXN[j] > 90 {
				count++
			}
		}
		if count == 1 {
			signals[i] = 100
		}
	}
	return evalRedTopSignal(signals, config, "见涨")
}

// ============================================================================
//  SignalRedTopGoldenCross — 金叉信号 (Seq: 06 → 01103006)
// ============================================================================

type SignalRedTopGoldenCross struct {
	indicator.BaseSignal
}

func NewSignalRedTopGoldenCross() *SignalRedTopGoldenCross {
	return &SignalRedTopGoldenCross{
		BaseSignal: indicator.NewBaseSignal(
			"06",
			"金叉",
			"金叉", // MACD金叉+KDJ金叉双重确认，出现金叉买入信号
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
				Params:   map[string]any{indicator.ParamKeyLookbackStart: float64(0), indicator.ParamKeyLookbackEnd: float64(0)},
			},
		),
	}
}

func (s *SignalRedTopGoldenCross) Evaluate(r *redTopResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	n := len(r.DIF)

	signals := make([]float64, n)
	for i := 1; i < n; i++ {
		macdGolden := r.DIF[i] > r.DEA[i] && r.DIF[i-1] <= r.DEA[i-1]
		kdjGolden := r.K[i] > r.D_[i] && r.K[i-1] <= r.D_[i-1]
		if macdGolden && kdjGolden {
			signals[i] = 100
		}
	}
	return evalRedTopSignal(signals, config, "金叉")
}
