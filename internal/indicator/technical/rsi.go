// ============================================================================
//  RSI — 相对强弱指标 (三线系统: 6/12/24)
//  ID: 01005 = CatCodeTechnical("01") + IndRsiSeq("005")
//  数据源: GetDailyKline()
//
//  设计:
//    在 Evaluate 层统一计算 6日/12日/24日 三条 RSI 序列，
//    所有信号共享同一份 RSIResult，不重复计算。
//
//  三条线:
//    快线 (RSI6)  — 短期 RSI，对价格波动最敏感
//    中线 (RSI12) — 中期 RSI，走势相对平滑
//    慢线 (RSI24) — 长期 RSI，稳定性最强
//
//  信号:
//    01 金叉    — 短期 RSI 由下向上穿过中期 RSI（买入信号）
//    02 死叉    — 短期 RSI 由上向下穿过中期 RSI（卖出信号）
//    03 三线合一 — 三条线在低位(30以下)由发散转粘合并同步向上形成金叉（强烈买入信号）
//    04 金叉    — 同01，允许自定义时间窗口
//    05 死叉    — 同02，允许自定义时间窗口
//    06 三线合一 — 同03，允许自定义时间窗口+低位阈值
// ============================================================================

package technical

import (
	"fmt"

	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  RSIResult — RSI 预计算结果，供所有信号复用
//  数据顺序: 从旧到新 ([0]=最旧, [len-1]=最新)
// ============================================================================

type RSIResult struct {
	RSI6       []float64 // 6日RSI序列
	RSI12      []float64 // 12日RSI序列
	RSI24      []float64 // 24日RSI序列
	ClosePrice []float64 // 收盘价（元，从旧到新）
}

// RSI 默认参数
const (
	rsiDefaultFastPeriod = 6  // 快线周期
	rsiDefaultMidPeriod  = 12 // 中线周期
	rsiDefaultSlowPeriod = 24 // 慢线周期
	rsiDefaultLowZone    = 30 // 低位阈值
	rsiMinKlines         = 50 // 最少K线根数（max(6,12,24) + lookback + 缓冲）
)

// 自定义参数 key
const (
	paramLowZone = "low_zone" // 低位阈值（三线合一信号）
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
			Desc:        "相对强弱指标 6/12/24 三线系统（金叉/死叉/三线合一）",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		newSignalRsiGoldenCross("01", "金叉"),             // 01 金叉
		newSignalRsiDeathCross("02", "死叉"),              // 02 死叉
		newSignalRsiTripleConvergence("03", "三线合一"),     // 03 三线合一
	})

	i.SetCustomSignals([]indicator.Signal{
		newSignalRsiGoldenCross("04", "金叉"),             // 04 自定义金叉（可调时间窗口）
		newSignalRsiDeathCross("05", "死叉"),              // 05 自定义死叉（可调时间窗口）
		newSignalRsiTripleConvergence("06", "三线合一"),     // 06 自定义三线合一（可调时间窗口+低位阈值）
	})
	return i
}

// ============================================================================
//  Evaluate — RSI 指标评估入口
//  1. 计算 RSI6/RSI12/RSI24 三条序列
//  2. 分发给各信号评估
// ============================================================================

func (i *Rsi) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: err.Error()}
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
			case *SignalRsiGoldenCross:
				res = v.Evaluate(result, cfg)
			case *SignalRsiDeathCross:
				res = v.Evaluate(result, cfg)
			case *SignalRsiTripleConvergence:
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

// ============================================================================
//  buildRSI — 计算 RSI6/RSI12/RSI24 三条序列
//
//  公式 (SMA 版本，通达信标准):
//    Delta[i] = Close[i] - Close[i-1]
//    Up[i]    = Max(Delta[i], 0)
//    Down[i]  = Max(-Delta[i], 0)
//    AvgUp    = SMA(Up, N, 1) / N
//    AvgDown  = SMA(Down, N, 1) / N
//    RSI      = AvgUp / (AvgUp + AvgDown) * 100
//
//  klines 输入: [0]=最新, [len-1]=最旧
//  结果: oldest-first (从旧到新), [0]=最旧, [len-1]=最新
// ============================================================================

func buildRSI(klines []*model.DailyKline) RSIResult {
	n := len(klines)

	// klines[0] = 最新 → 反转为 oldest-first，分→元
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

	// 分别计算三个周期的 RSI
	rsi6 := computeRSI(up, down, rsiDefaultFastPeriod)
	rsi12 := computeRSI(up, down, rsiDefaultMidPeriod)
	rsi24 := computeRSI(up, down, rsiDefaultSlowPeriod)

	return RSIResult{
		RSI6:       rsi6,
		RSI12:      rsi12,
		RSI24:      rsi24,
		ClosePrice: closePrices,
	}
}

// computeRSI 根据 up/down 序列和周期 N 计算 RSI 序列
func computeRSI(up, down []float64, period int) []float64 {
	n := len(up)
	avgUp := sma(up, period, 1)
	avgDown := sma(down, period, 1)

	rsi := make([]float64, n)
	for i := range rsi {
		sum := avgUp[i] + avgDown[i]
		if sum != 0 {
			rsi[i] = avgUp[i] / sum * 100
		} else {
			rsi[i] = 50
		}
	}
	return rsi
}

// ============================================================================
//  SignalRsiGoldenCross — 金叉信号
//
//  内置 ID: 01 (默认时间窗口 5天前~今天)
//  自定义 ID: 04 (可调时间窗口)
//
//  判定: 短期RSI(6)由下向上穿过中期RSI(12)
//  条件: 前一日 RSI6 <= RSI12, 当日 RSI6 > RSI12
// ============================================================================

type SignalRsiGoldenCross struct {
	indicator.BaseSignal
}

func newSignalRsiGoldenCross(id, name string) *SignalRsiGoldenCross {
	return &SignalRsiGoldenCross{
		BaseSignal: indicator.NewBaseSignal(
			id,
			name,
			"短期RSI(6日)由下向上穿过中期RSI(12日)——买入信号",
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

func (s *SignalRsiGoldenCross) Evaluate(result RSIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.RSI6))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 从新到旧扫描窗口 [idxStart, idxEnd)
	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}
		// 金叉: 前一日 RSI6 <= RSI12, 当日 RSI6 > RSI12
		if result.RSI6[i] > result.RSI12[i] && result.RSI6[i-1] <= result.RSI12[i-1] {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultPassed,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("金叉：RSI6(%.1f)上穿RSI12(%.1f)", result.RSI6[i], result.RSI12[i]),
			}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到RSI金叉", start, end),
	}
}

// ============================================================================
//  SignalRsiDeathCross — 死叉信号
//
//  内置 ID: 02 (默认时间窗口 5天前~今天)
//  自定义 ID: 05 (可调时间窗口)
//
//  判定: 短期RSI(6)由上向下穿过中期RSI(12)
//  条件: 前一日 RSI6 >= RSI12, 当日 RSI6 < RSI12
// ============================================================================

type SignalRsiDeathCross struct {
	indicator.BaseSignal
}

func newSignalRsiDeathCross(id, name string) *SignalRsiDeathCross {
	return &SignalRsiDeathCross{
		BaseSignal: indicator.NewBaseSignal(
			id,
			name,
			"短期RSI(6日)由上向下穿过中期RSI(12日)——卖出信号",
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

func (s *SignalRsiDeathCross) Evaluate(result RSIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 5))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.RSI6))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	for i := idxEnd - 1; i >= idxStart; i-- {
		if i <= 0 {
			continue
		}
		// 死叉: 前一日 RSI6 >= RSI12, 当日 RSI6 < RSI12
		if result.RSI6[i] < result.RSI12[i] && result.RSI6[i-1] >= result.RSI12[i-1] {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultPassed,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("死叉：RSI6(%.1f)下穿RSI12(%.1f)", result.RSI6[i], result.RSI12[i]),
			}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到RSI死叉", start, end),
	}
}

// ============================================================================
//  SignalRsiTripleConvergence — 三线合一信号
//
//  内置 ID: 03 (默认时间窗口 20天前~今天, low_zone=30)
//  自定义 ID: 06 (可调时间窗口+低位阈值)
//
//  判定: 三线在低位区域(30以下)由发散转为粘合并同步向上形成金叉
//
//  检测逻辑（从新到旧扫描 lookback 窗口）:
//    1. 三条线(RSI6/RSI12/RSI24)均在低位阈值以下
//    2. 前段发散 → 后段粘合（三条线之间的最大间距缩小）
//    3. 最新位置出现金叉（RSI6上穿RSI12）
//    4. 三条线同步向上（RSI6 > 前一日RSI6, RSI12 > 前一日RSI12）
// ============================================================================

// rsiTripleConvergenceMinDays 三线合一至少需要的低位天数
const rsiTripleConvergenceMinDays = 5

type SignalRsiTripleConvergence struct {
	indicator.BaseSignal
}

func newSignalRsiTripleConvergence(id, name string) *SignalRsiTripleConvergence {
	return &SignalRsiTripleConvergence{
		BaseSignal: indicator.NewBaseSignal(
			id,
			name,
			"三条RSI线在低位(30以下)由发散转粘合并同步向上金叉——强烈买入信号",
			indicator.ValSeries,
			[]indicator.OperatorOption{
				{
					Operator: indicator.OpCustom,
					Label:    "参数设置",
					Params: []indicator.ParamDef{
						signalutil.ParamLookbackStart(20, "天前"),
						signalutil.ParamLookbackEnd(0, "天前"),
						{Key: paramLowZone, Label: "低位阈值", Type: "number", Required: false,
							Default: rsiDefaultLowZone, Min: 10, Max: 50, Step: 1, Unit: ""},
					},
				},
			},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyLookbackStart: float64(20),
					indicator.ParamKeyLookbackEnd:   float64(0),
					paramLowZone:                    float64(rsiDefaultLowZone),
				},
			},
		),
	}
}

func (s *SignalRsiTripleConvergence) Evaluate(result RSIResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 20))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	lowZone := config.GetFloat64(paramLowZone, rsiDefaultLowZone)

	idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(result.RSI6))
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: err.Error()}
	}

	// 从新到旧扫描，找满足三线合一条件的区间
	for i := idxEnd - 1; i >= idxStart+rsiTripleConvergenceMinDays; i-- {
		if !isTripleConvergence(result, i, idxStart, lowZone) {
			continue
		}

		return &indicator.EvaluatedStock{
			Result:   indicator.ResultPassed,
			SignalID: config.SignalID,
			Message: fmt.Sprintf("三线合一：RSI6(%.1f)/RSI12(%.1f)/RSI24(%.1f) 低位粘合并金叉向上",
				result.RSI6[i], result.RSI12[i], result.RSI24[i]),
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("在[%d天前, %d天前]窗口内未检测到三线合一信号", start, end),
	}
}

// isTripleConvergence 判断从 endIdx 往回 rsiTripleConvergenceMinDays 天是否构成三线合一。
//
// 参数:
//   - endIdx: 当前检查的最迟索引（窗口最右侧）
//   - windowStart: lookback 窗口最早索引
//   - lowZone: 低位阈值
func isTripleConvergence(result RSIResult, endIdx, windowStart int, lowZone float64) bool {
	// endIdx 需要往前 rsiTripleConvergenceMinDays 天
	if endIdx-rsiTripleConvergenceMinDays+1 < windowStart {
		return false
	}

	rangeStart := endIdx - rsiTripleConvergenceMinDays + 1
	rangeEnd := endIdx

	// 条件1: 窗口内所有三条线都在低位阈值以下
	for d := rangeStart; d <= rangeEnd; d++ {
		if result.RSI6[d] >= lowZone || result.RSI12[d] >= lowZone || result.RSI24[d] >= lowZone {
			return false
		}
	}

	// 条件2: 由发散转为粘合 — 前半段的最大间距 > 后半段的最大间距
	// 前半段: [rangeStart, mid]   后半段: [mid+1, rangeEnd]
	mid := (rangeStart + rangeEnd) / 2
	frontSpread := maxTripleSpread(result, rangeStart, mid)
	backSpread := maxTripleSpread(result, mid+1, rangeEnd)
	if frontSpread < backSpread*1.2 {
		// 收敛不明显（允许 20% 容差）
		return false
	}

	// 条件3: 最新位置出现金叉
	// RSI6 从下方穿过 RSI12: 前一日 RSI6 <= RSI12, 当日 RSI6 > RSI12
	if result.RSI6[rangeEnd] <= result.RSI12[rangeEnd] {
		return false
	}
	if result.RSI6[rangeEnd-1] > result.RSI12[rangeEnd-1] {
		// 前一日已在上方，不是刚发生的金叉
		// 放宽条件：只要当前 RSI6 > RSI12 且前一日在下方或附近，也算
		if result.RSI6[rangeEnd-1] > result.RSI12[rangeEnd-1]+2 {
			return false
		}
	}

	// 条件4: 三条线同步向上（RSI6 和 RSI12 都在涨）
	if result.RSI6[rangeEnd] <= result.RSI6[rangeEnd-1] {
		return false
	}
	if result.RSI12[rangeEnd] <= result.RSI12[rangeEnd-1] {
		return false
	}

	return true
}

// maxTripleSpread 计算三条RSI线在 [start, end] 范围内的最大间距
func maxTripleSpread(result RSIResult, start, end int) float64 {
	maxSpread := 0.0
	for d := start; d <= end; d++ {
		maxVal := max3(result.RSI6[d], result.RSI12[d], result.RSI24[d])
		minVal := min3(result.RSI6[d], result.RSI12[d], result.RSI24[d])
		spread := maxVal - minVal
		if spread > maxSpread {
			maxSpread = spread
		}
	}
	return maxSpread
}

// max3 返回三个 float64 的最大值
func max3(a, b, c float64) float64 {
	if a > b {
		if a > c {
			return a
		}
		return c
	}
	if b > c {
		return b
	}
	return c
}

// min3 返回三个 float64 的最小值
func min3(a, b, c float64) float64 {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
