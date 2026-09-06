package technical

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"stock-ai/internal/backtest/indicator"
)

// newTestBollResult 构造测试用 BOLLResult：UP/DN/MB 常量，收盘价序列由调用方给定。
// MB 取 (UP+DN)/2 模拟中轨，并按 calcBollDerived 预计算 %B/BBW 派生序列。
// 数据顺序从旧到新 ([0]=最旧, [n-1]=最新)，与 buildBOLL 输出一致。
// 注意: 若测试随后修改了 MB/UP/DN/收盘价，需手动同步对应下标的 PercentB/BandWidth。
func newTestBollResult(n int, up, dn float64, closePx []float64) BOLLResult {
	mbs := make([]float64, n)
	ups := make([]float64, n)
	dns := make([]float64, n)
	for i := 0; i < n; i++ {
		mbs[i] = (up + dn) / 2
		ups[i] = up
		dns[i] = dn
	}
	cp := make([]float64, n)
	copy(cp, closePx)
	pB, bw := calcBollDerived(cp, mbs, ups, dns)
	return BOLLResult{
		MB:         mbs,
		UP:         ups,
		DN:         dns,
		ClosePrice: cp,
		PercentB:   pB,
		BandWidth:  bw,
	}
}

// bollValTestCfg 构造 BOLL 数值比较信号（%B/BBW）的比较配置。
// custom=true 时 SignalID 第 6 位为 '1'（自定义信号标记）。
func bollValTestCfg(op indicator.CompareOperator, custom bool, params map[string]any) *indicator.SignalConfig {
	id := "01007001" // 第 6 位 '0' = 内置信号
	if custom {
		id = "01007101" // 第 6 位 '1' = 自定义信号
	}
	return &indicator.SignalConfig{
		SignalID: id,
		Operator: op,
		Params:   params,
	}
}

// TestSigBollPos_LatestDayPass 最新一天 %B=0.9，GTE 0.8 应通过。
func TestSigBollPos_LatestDayPass(t *testing.T) {
	const n = 10
	// UP=10, DN=8 → 带宽 2；最新(idx9) close=9.8 → %B=0.9
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	sig := NewSignalBollPosition()

	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.8,
		indicator.ParamKeyDays:      float64(0),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultPassed, got.Result, "0.9 ≥ 0.8 应通过")

	cfg2 := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.95,
		indicator.ParamKeyDays:      float64(0),
	})
	got2 := sig.Evaluate(res, cfg2)
	assert.Equal(t, indicator.ResultRejected, got2.Result, "0.9 < 0.95 应拒绝")
}

// TestSigBollPos_LookupDays 指定取值天数应取对应的历史日 %B。
func TestSigBollPos_LookupDays(t *testing.T) {
	const n = 10
	// idx8(1天前) close=9.0 → %B=0.5；idx7(2天前) close=8.6 → %B=0.3
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 8.6))
	sig := NewSignalBollPosition()

	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.5,
		indicator.ParamKeyDays:      float64(1),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultPassed, got.Result, "1天前 %B=0.5 ≥ 0.5 应通过")

	cfg2 := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.4,
		indicator.ParamKeyDays:      float64(2),
	})
	got2 := sig.Evaluate(res, cfg2)
	assert.Equal(t, indicator.ResultRejected, got2.Result, "2天前 %B=0.3 < 0.4 应拒绝")

	cfg3 := bollValTestCfg(indicator.OpLTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.3,
		indicator.ParamKeyDays:      float64(2),
	})
	got3 := sig.Evaluate(res, cfg3)
	assert.Equal(t, indicator.ResultPassed, got3.Result, "2天前 %B=0.3 ≤ 0.3 应通过")
}

// TestSigBollPos_DaysOutOfRange 取值天数超出数据范围应拒绝。
func TestSigBollPos_DaysOutOfRange(t *testing.T) {
	const n = 10
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	sig := NewSignalBollPosition()

	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.5,
		indicator.ParamKeyDays:      float64(50),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultRejected, got.Result)
	assert.Contains(t, got.Message, "超出数据范围")
}

// TestSigBollPos_ZeroBand 布林带宽度为 0（UP==DN）时应拒绝并给出说明。
func TestSigBollPos_ZeroBand(t *testing.T) {
	const n = 10
	res := newTestBollResult(n, 8, 8, posClose(9.8, 9.0, 9.0))
	sig := NewSignalBollPosition()

	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.5,
		indicator.ParamKeyDays:      float64(0),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultRejected, got.Result)
	assert.Contains(t, got.Message, "布林带宽度非正")
}

// TestSigBollPos_NonCustomFallsBackToDefault 非自定义配置（无 marker）应回退到信号默认配置。
func TestSigBollPos_NonCustomFallsBackToDefault(t *testing.T) {
	const n = 10
	// 最新日 %B=0.9
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	sig := NewSignalBollPosition()

	// 若误用给定配置(OpLT 0.2)会拒绝；回退默认配置(OpGTE 0.8)后 0.9 ≥ 0.8 应通过
	cfg := bollValTestCfg(indicator.OpLT, false, map[string]any{
		indicator.ParamKeyThreshold: 0.2,
		indicator.ParamKeyDays:      float64(0),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultPassed, got.Result)
}

// TestSigBollBw_LatestDayPass 最新一天 BBW，GTE 阈值应通过/拒绝。
func TestSigBollBw_LatestDayPass(t *testing.T) {
	const n = 10
	// UP=10, DN=8, MB=9 → BBW = 2/9 ≈ 0.2222
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	sig := NewSignalBollBandWidth()

	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.2,
		indicator.ParamKeyDays:      float64(0),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultPassed, got.Result, "BBW≈0.222 ≥ 0.2 应通过")

	cfg2 := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.3,
		indicator.ParamKeyDays:      float64(0),
	})
	got2 := sig.Evaluate(res, cfg2)
	assert.Equal(t, indicator.ResultRejected, got2.Result, "BBW≈0.222 < 0.3 应拒绝")
}

// TestSigBollBw_LookupDays 指定取值天数应取对应的历史日 BBW。
func TestSigBollBw_LookupDays(t *testing.T) {
	const n = 10
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	// 2天前(idx7)上轨抬升至 10.6 → BBW=(10.6-8)/9≈0.289
	res.UP[7] = 10.6
	res.BandWidth[7] = (res.UP[7] - res.DN[7]) / res.MB[7] // 派生序列需与源数组同步
	sig := NewSignalBollBandWidth()

	// 2天前 BBW≈0.289 ≥ 0.28 应通过
	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.28,
		indicator.ParamKeyDays:      float64(2),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultPassed, got.Result, "2天前 BBW≈0.289 ≥ 0.28 应通过")

	// 1天前 BBW≈0.222 < 0.28 应拒绝
	cfg2 := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.28,
		indicator.ParamKeyDays:      float64(1),
	})
	got2 := sig.Evaluate(res, cfg2)
	assert.Equal(t, indicator.ResultRejected, got2.Result, "1天前 BBW≈0.222 < 0.28 应拒绝")

	// 收窄检测：BBW 变小 → LTE 较高阈值应通过
	cfg3 := bollValTestCfg(indicator.OpLTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.23,
		indicator.ParamKeyDays:      float64(0),
	})
	got3 := sig.Evaluate(res, cfg3)
	assert.Equal(t, indicator.ResultPassed, got3.Result, "BBW≈0.222 ≤ 0.23 收窄 应通过")
}

// TestSigBollBw_MBZero 中轨 MB 为 0（分母非法）时应拒绝并给出说明。
func TestSigBollBw_MBZero(t *testing.T) {
	const n = 10
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	res.MB[9] = 0
	res.BandWidth[9] = math.NaN() // MB 非正 → BBW 为 NaN
	sig := NewSignalBollBandWidth()

	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 0.2,
		indicator.ParamKeyDays:      float64(0),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultRejected, got.Result)
	assert.Contains(t, got.Message, "中轨MB非正")
}

// TestSigBollBwRatio_LatestDayOpen 今日 BBW 高于昨日 → 比值>1 开口扩大应通过；反之收窄应拒绝。
func TestSigBollBwRatio_LatestDayOpen(t *testing.T) {
	const n = 10
	// 基准: UP=10, DN=8, MB=9 → 每点 BBW=2/9≈0.222
	// 今日(idx9)上轨抬升至 10.6 → 今日 BBW=(10.6-8)/9≈0.289
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	res.UP[9] = 10.6
	res.BandWidth[9] = (res.UP[9] - res.DN[9]) / res.MB[9] // 派生序列需与源数组同步
	sig := NewSignalBollBandWidthRatio()

	// 今日/昨日 ≈ 0.289/0.222 ≈ 1.30 ≥ 1 应通过（开口扩大）
	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(0),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultPassed, got.Result, "比值≈1.30 ≥ 1 应通过")

	// 阈值调高到 1.5 后比值不足应拒绝
	cfg2 := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1.5,
		indicator.ParamKeyDays:      float64(0),
	})
	got2 := sig.Evaluate(res, cfg2)
	assert.Equal(t, indicator.ResultRejected, got2.Result, "比值≈1.30 < 1.5 应拒绝")
}

// TestSigBollBwRatio_Tighten 昨日 BBW 高于今日 → 比值<1 收窄应通过。
func TestSigBollBwRatio_Tighten(t *testing.T) {
	const n = 10
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	// 昨日(idx8)上轨抬升至 10.6 → 昨日 BBW≈0.289，今日为 0.222 → 比值≈0.769
	res.UP[8] = 10.6
	res.BandWidth[8] = (res.UP[8] - res.DN[8]) / res.MB[8] // 派生序列需与源数组同步
	sig := NewSignalBollBandWidthRatio()

	// 今日/昨日 ≈ 0.769 ≤ 1 应通过（收窄）
	cfg := bollValTestCfg(indicator.OpLTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(0),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultPassed, got.Result, "比值≈0.769 ≤ 1 应通过")

	// 小于比较 <1 也应通过
	cfg2 := bollValTestCfg(indicator.OpLT, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(0),
	})
	got2 := sig.Evaluate(res, cfg2)
	assert.Equal(t, indicator.ResultPassed, got2.Result, "比值≈0.769 < 1 应通过")
}

// TestSigBollBwRatio_LookupDays 指定取值天数时比较 N 天前与 N+1 天前两点的 BBW。
func TestSigBollBwRatio_LookupDays(t *testing.T) {
	const n = 10
	res := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	// 3天前(idx6)上轨抬升 → 3天前 BBW 较大，带宽自 4天前→3天前 扩大
	res.UP[6] = 10.6
	res.BandWidth[6] = (res.UP[6] - res.DN[6]) / res.MB[6] // 派生序列需与源数组同步
	sig := NewSignalBollBandWidthRatio()

	// days=3: 比较 idx6(≈0.289) ÷ idx5(0.222) ≈ 1.30 ≥ 1 应通过
	cfg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(3),
	})
	got := sig.Evaluate(res, cfg)
	assert.Equal(t, indicator.ResultPassed, got.Result, "3天前/4天前 比值≈1.30 ≥ 1 应通过")

	// days=2: 比较 idx7 与 idx6(≈0.289)，比值<1 应拒绝
	cfg2 := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(2),
	})
	got2 := sig.Evaluate(res, cfg2)
	assert.Equal(t, indicator.ResultRejected, got2.Result, "2天前/3天前 比值<1 应拒绝")
}

// TestSigBollBwRatio_Edges 分母/中轨非法及取值天数越界应拒绝。
func TestSigBollBwRatio_Edges(t *testing.T) {
	const n = 10
	sig := NewSignalBollBandWidthRatio()

	// 分母 BBW(N+1天前)=0（UP==DN）应拒绝
	resZeroFar := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	resZeroFar.DN[8] = 10       // 昨日带宽为 0
	resZeroFar.BandWidth[8] = 0 // 派生序列需与源数组同步
	cfgFar := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(0),
	})
	gotFar := sig.Evaluate(resZeroFar, cfgFar)
	assert.Equal(t, indicator.ResultRejected, gotFar.Result)
	assert.Contains(t, gotFar.Message, "BBW为0")

	// 中轨 MB 非法应拒绝
	resBadMB := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	resBadMB.MB[9] = 0
	resBadMB.BandWidth[9] = math.NaN() // MB 非正 → BBW 为 NaN
	cfgMB := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(0),
	})
	gotMB := sig.Evaluate(resBadMB, cfgMB)
	assert.Equal(t, indicator.ResultRejected, gotMB.Result)
	assert.Contains(t, gotMB.Message, "中轨MB非正")

	// N+1 天前超出数据范围（days 为最大合法值）应拒绝
	resRange := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	cfgRange := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(n - 1),
	})
	gotRange := sig.Evaluate(resRange, cfgRange)
	assert.Equal(t, indicator.ResultRejected, gotRange.Result)
	assert.Contains(t, gotRange.Message, "超出数据范围")

	// 负数 days 会越过序列上限，应视为越界拒绝而非 panic
	resNeg := newTestBollResult(n, 10, 8, posClose(9.8, 9.0, 9.0))
	cfgNeg := bollValTestCfg(indicator.OpGTE, true, map[string]any{
		indicator.ParamKeyThreshold: 1,
		indicator.ParamKeyDays:      float64(-1),
	})
	gotNeg := sig.Evaluate(resNeg, cfgNeg)
	assert.Equal(t, indicator.ResultRejected, gotNeg.Result)
	assert.Contains(t, gotNeg.Message, "超出数据范围")
}

// TestCalcBollDerived 校验 %B/BBW 派生序列公式及分母非法处为 NaN。
func TestCalcBollDerived(t *testing.T) {
	closePx := []float64{10, 10, 10, 12}
	mb := []float64{10, 10, 10, 11}
	up := []float64{11, 12, 10, 12.5}
	dn := []float64{9, 8, 10, 9.5}

	pB, bw := calcBollDerived(closePx, mb, up, dn)
	assert.Len(t, pB, 4)
	assert.Len(t, bw, 4)

	// i0: 带宽=2 → %B=(10-9)/2=0.5；BBW=2/10=0.2
	assert.InDelta(t, 0.5, pB[0], 1e-9)
	assert.InDelta(t, 0.2, bw[0], 1e-9)
	// i1: 带宽=4 → %B=(10-8)/4=0.5；BBW=4/10=0.4
	assert.InDelta(t, 0.5, pB[1], 1e-9)
	assert.InDelta(t, 0.4, bw[1], 1e-9)
	// i2: 带宽=0 → %B 为 NaN；BBW=0/10=0
	assert.True(t, math.IsNaN(pB[2]))
	assert.InDelta(t, 0, bw[2], 1e-9)
	// i3: %B=(12-9.5)/3≈0.8333；BBW=3/11≈0.2727
	assert.InDelta(t, 0.833333, pB[3], 1e-4)
	assert.InDelta(t, 3.0/11.0, bw[3], 1e-9)

	// 中轨非正 → BBW 为 NaN
	pB2, bw2 := calcBollDerived([]float64{1}, []float64{-1}, []float64{1}, []float64{0})
	assert.InDelta(t, 1, pB2[0], 1e-9)
	assert.True(t, math.IsNaN(bw2[0]))
}

// posClose 按长度 3 构造收盘价序列，其余位置用 mid 填充。
func posClose(latest, day1, day2 float64) []float64 {
	c := make([]float64, 10)
	for i := range c {
		c[i] = 9.0 // 默认 %B=0.5
	}
	c[9] = latest
	c[8] = day1
	c[7] = day2
	return c
}
