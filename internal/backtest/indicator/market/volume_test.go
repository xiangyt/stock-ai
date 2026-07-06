package market

import (
	"testing"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/model"
)

// ============================================================================
//  newVolumeTrendSignal 构造函数测试
// ============================================================================

func TestNewVolumeTrendSignal_ContainsLookbackParams(t *testing.T) {
	sig := newVolumeTrendSignal("02", "连续放量", "描述", trendUp)

	ops := sig.Operators()
	if len(ops) == 0 {
		t.Fatal("Operators 为空")
	}

	op := ops[0]

	// 验证 Operator 为 OpCustom
	if op.Operator != indicator.OpCustom {
		t.Errorf("Operator = %s, want %s", op.Operator, indicator.OpCustom)
	}

	foundThreshold := false
	foundStart := false
	foundEnd := false

	for _, p := range op.Params {
		switch p.Key {
		case indicator.ParamKeyThreshold:
			foundThreshold = true
		case indicator.ParamKeyLookbackStart:
			foundStart = true
		case indicator.ParamKeyLookbackEnd:
			foundEnd = true
		}
	}

	if !foundThreshold {
		t.Error("未找到 threshold 参数")
	}
	if !foundStart {
		t.Error("未找到 lookback_start 参数")
	}
	if !foundEnd {
		t.Error("未找到 lookback_end 参数")
	}
}

func TestNewVolumeTrendSignal_Trend(t *testing.T) {
	sigUp := newVolumeTrendSignal("02", "连续放量", "描述", trendUp)
	if sigUp.Trend() != trendUp {
		t.Errorf("Trend() = %s, want up", sigUp.Trend())
	}

	sigDown := newVolumeTrendSignal("03", "连续缩量", "描述", trendDown)
	if sigDown.Trend() != trendDown {
		t.Errorf("Trend() = %s, want down", sigDown.Trend())
	}
}

// ============================================================================
//  volumeTrendSignal.EvaluateWithWindow 测试
// ============================================================================

func TestVolumeTrendSignal_EvaluateWithWindow_Up_Pass(t *testing.T) {
	// 放量：当日/前日 > 1.0
	sig := newVolumeTrendSignal("02", "连续放量", "描述", trendUp)

	klines := []*model.DailyKline{
		{Volume: 8_000_000}, // [0] 今天 → 比值=8/5=1.6 > 1.0 ✓
		{Volume: 5_000_000}, // [1] 昨天
	}

	cfg := &indicator.SignalConfig{
		SignalID: "02001002",
		Params: map[string]any{
			indicator.ParamKeyThreshold:      1.0,
			indicator.ParamKeyLookbackStart: float64(0),
			indicator.ParamKeyLookbackEnd:   float64(0),
		},
	}

	res := sig.EvaluateWithWindow(klines, cfg)
	if res.Result != indicator.ResultPassed {
		t.Errorf("放量通过: Result=%v, msg=%s", res.Result, res.Message)
	}
}

func TestVolumeTrendSignal_EvaluateWithWindow_Up_Fail(t *testing.T) {
	// 放量：当日/前日 > 1.0，不满足
	sig := newVolumeTrendSignal("02", "连续放量", "描述", trendUp)

	klines := []*model.DailyKline{
		{Volume: 3_000_000}, // [0] 今天 → 比值=3/5=0.6 < 1.0 ✗
		{Volume: 5_000_000}, // [1] 昨天
	}

	cfg := &indicator.SignalConfig{
		SignalID: "02001002",
		Params: map[string]any{
			indicator.ParamKeyThreshold:      1.0,
			indicator.ParamKeyLookbackStart: float64(0),
			indicator.ParamKeyLookbackEnd:   float64(0),
		},
	}

	res := sig.EvaluateWithWindow(klines, cfg)
	if res.Result == indicator.ResultPassed {
		t.Error("期望不放量时返回 Rejected")
	}
}

func TestVolumeTrendSignal_EvaluateWithWindow_Down_Pass(t *testing.T) {
	// 缩量：当日/前日 < 1.0
	sig := newVolumeTrendSignal("03", "连续缩量", "描述", trendDown)

	klines := []*model.DailyKline{
		{Volume: 2_000_000}, // [0] 今天 → 比值=2/3=0.67 < 1.0 ✓
		{Volume: 3_000_000}, // [1] 昨天 → 比值=3/4=0.75 < 1.0 ✓
		{Volume: 4_000_000}, // [2] 前天 → 比值=4/5=0.80 < 1.0 ✓
		{Volume: 5_000_000}, // [3] 大前天
	}

	cfg := &indicator.SignalConfig{
		SignalID: "02001003",
		Params: map[string]any{
			indicator.ParamKeyThreshold:      1.0,
			indicator.ParamKeyLookbackStart: float64(2),
			indicator.ParamKeyLookbackEnd:   float64(0),
		},
	}

	res := sig.EvaluateWithWindow(klines, cfg)
	if res.Result != indicator.ResultPassed {
		t.Errorf("缩量全通过: Result=%v, msg=%s", res.Result, res.Message)
	}
}

func TestVolumeTrendSignal_EvaluateWithWindow_Down_Fail(t *testing.T) {
	// 缩量：某天比值 >= 1.0 不满足
	sig := newVolumeTrendSignal("03", "连续缩量", "描述", trendDown)

	klines := []*model.DailyKline{
		{Volume: 2_000_000}, // [0] 今天 → 比值=2/5=0.40 < 1.0 ✓
		{Volume: 5_000_000}, // [1] 昨天 → 比值=5/3=1.67 > 1.0 ✗
		{Volume: 3_000_000}, // [2] 前天 → 比值=3/4=0.75 < 1.0
		{Volume: 4_000_000}, // [3] 大前天
	}

	cfg := &indicator.SignalConfig{
		SignalID: "02001003",
		Params: map[string]any{
			indicator.ParamKeyThreshold:      1.0,
			indicator.ParamKeyLookbackStart: float64(2),
			indicator.ParamKeyLookbackEnd:   float64(0),
		},
	}

	res := sig.EvaluateWithWindow(klines, cfg)
	if res.Result == indicator.ResultPassed {
		t.Error("期望某天不满足时返回 Rejected")
	}
}

func TestVolumeTrendSignal_EvaluateWithWindow_InsufficientData(t *testing.T) {
	sig := newVolumeTrendSignal("02", "连续放量", "描述", trendUp)

	klines := []*model.DailyKline{
		{Volume: 10_000_000},
	}

	cfg := &indicator.SignalConfig{
		SignalID: "02001002",
		Params: map[string]any{
			indicator.ParamKeyThreshold:      1.0,
			indicator.ParamKeyLookbackStart: float64(0), // 需要2条(start+2)
			indicator.ParamKeyLookbackEnd:   float64(0),
		},
	}

	res := sig.EvaluateWithWindow(klines, cfg)
	if res.Result == indicator.ResultPassed {
		t.Error("数据不足时应返回 Rejected")
	}
}

func TestVolumeTrendSignal_EvaluateWithWindow_ZeroPrevVolume(t *testing.T) {
	sig := newVolumeTrendSignal("02", "连续放量", "描述", trendUp)

	klines := []*model.DailyKline{
		{Volume: 5_000_000},
		{Volume: 0},
	}

	cfg := &indicator.SignalConfig{
		SignalID: "02001002",
		Params: map[string]any{
			indicator.ParamKeyThreshold:      1.0,
			indicator.ParamKeyLookbackStart: float64(0),
			indicator.ParamKeyLookbackEnd:   float64(0),
		},
	}

	res := sig.EvaluateWithWindow(klines, cfg)
	if res.Result == indicator.ResultPassed {
		t.Error("前日成交为0时应返回 Rejected")
	}
}

// ============================================================================
//  NewVolume 完整性测试
// ============================================================================

func TestNewVolume_HasThreeCustomSignals(t *testing.T) {
	v := NewVolume()
	customSigs := v.CustomSignals()

	if len(customSigs) != 3 {
		t.Fatalf("自定义信号数量=%d, want=3", len(customSigs))
	}

	hasTrend := false
	for _, s := range customSigs {
		if _, ok := s.(*volumeTrendSignal); ok {
			hasTrend = true
		}
	}
	if !hasTrend {
		t.Error("缺少 volumeTrendSignal 类型的自定义信号")
	}
}

func TestNewVolume_TrendSignalsDefaultConfig(t *testing.T) {
	v := NewVolume()
	customSigs := v.CustomSignals()

	for _, s := range customSigs {
		ts, ok := s.(*volumeTrendSignal)
		if !ok {
			continue
		}

		cfg := ts.DefaultConfig()
		if cfg == nil {
			t.Error("DefaultConfig 不应为 nil")
			continue
		}

		// 验证 Operator 为 OpCustom
		if cfg.Operator != indicator.OpCustom {
			t.Errorf("Operator = %s, want %s", cfg.Operator, indicator.OpCustom)
		}

		// 验证倍数阈值默认为 1.0
		threshVal, ok := cfg.Params[indicator.ParamKeyThreshold]
		if !ok {
			t.Error("缺少 threshold 参数")
		} else if threshVal != float64(1.0) {
			t.Errorf("threshold 默认值=%.1f, want=1.0", threshVal)
		}

		// 验证时间窗口参数
		startVal, ok := cfg.Params[indicator.ParamKeyLookbackStart]
		if !ok {
			t.Error("缺少 lookback_start 参数")
		} else if startVal != float64(2) {
			t.Errorf("lookback_start 默认值=%.1f, want=2", startVal)
		}

		endVal, ok := cfg.Params[indicator.ParamKeyLookbackEnd]
		if !ok {
			t.Error("缺少 lookback_end 参数")
		} else if endVal != float64(0) {
			t.Errorf("lookback_end 默认值=%.1f, want=0", endVal)
		}
	}
}

func TestNewVolume_TrendSignals_HasTrendField(t *testing.T) {
	v := NewVolume()
	customSigs := v.CustomSignals()

	sawUp := false
	sawDown := false

	for _, s := range customSigs {
		ts, ok := s.(*volumeTrendSignal)
		if !ok {
			continue
		}
		switch ts.Trend() {
		case trendUp:
			sawUp = true
		case trendDown:
			sawDown = true
		}
	}

	if !sawUp {
		t.Error("缺少 trend=up 的放量信号")
	}
	if !sawDown {
		t.Error("缺少 trend=down 的缩量信号")
	}
}
