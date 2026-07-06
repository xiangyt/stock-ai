package market

import (
	"testing"

	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  calcChangePctAtIndex 纯函数测试
// ============================================================================

func TestCalcChangePctAtIndex_SingleDay(t *testing.T) {
	tests := []struct {
		name   string
		klines []*model.DailyKline
		idx    int
		want   float64
	}{
		{
			name: "正常涨跌幅",
			klines: []*model.DailyKline{
				{Close: 1100}, // 今天收盘 11.00 元
				{Close: 1000}, // 昨天收盘 10.00 元
			},
			idx:  0,
			want: 10.0, // (1100-1000)/1000*100 = 10%
		},
		{
			name: "下跌",
			klines: []*model.DailyKline{
				{Close: 900},
				{Close: 1000},
			},
			idx:  0,
			want: -10.0,
		},
		{
			name: "平盘",
			klines: []*model.DailyKline{
				{Close: 1000},
				{Close: 1000},
			},
			idx:  0,
			want: 0.0,
		},
		{
			name: "索引越界返回0",
			klines: []*model.DailyKline{
				{Close: 1000}, // 只有一条数据
			},
			idx:  0,
			want: 0.0,
		},
		{
			name: "计算第二天涨跌幅",
			klines: []*model.DailyKline{
				{Close: 1100}, // [0] 今天
				{Close: 1050}, // [1] 昨天
				{Close: 1000}, // [2] 前天
			},
			idx:  1,
			want: 5.0, // (1050-1000)/1000*100 = 5%
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcChangePctAtIndex(tt.klines, tt.idx)
			if got != tt.want {
				t.Errorf("calcChangePctAtIndex() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

// ============================================================================
//  EvaluateWithWindow 时间窗口测试
// ============================================================================

func TestChangePctSignal_EvaluateWithWindow_SingleDay(t *testing.T) {
	sig := &changePctSignal{}
	sig.BaseSignal = indicator.NewBaseSignal(
		"04", "大于5%", "大涨",
		indicator.ValNumber,
		[]indicator.OperatorOption{
			{
				Operator: indicator.OpGT,
				Label:    "大于",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", 5, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
		},
		&indicator.SignalConfig{
			Operator: indicator.OpGT,
			Params: map[string]any{
				indicator.ParamKeyThreshold:    float64(5),
				indicator.ParamKeyLookbackStart: float64(0),
				indicator.ParamKeyLookbackEnd:   float64(0),
			},
		},
	)

	tests := []struct {
		name       string
		klines     []*model.DailyKline
		config     *indicator.SignalConfig
		wantResult indicator.EvaluatedResult
	}{
		{
			name: "今天涨幅6%满足大于5%",
			klines: []*model.DailyKline{
				{Close: 1060}, // +6% → 满足
				{Close: 1000},
			},
			config: &indicator.SignalConfig{
				SignalID: "02004004",
				Params: map[string]any{
					indicator.ParamKeyThreshold:    float64(5),
					indicator.ParamKeyLookbackStart: float64(0),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
			wantResult: indicator.ResultPassed,
		},
		{
			name: "今天涨幅3%不满足大于5%",
			klines: []*model.DailyKline{
				{Close: 1030}, // +3% → 不满足
				{Close: 1000},
			},
			config: &indicator.SignalConfig{
				SignalID: "02004004",
				Params: map[string]any{
					indicator.ParamKeyThreshold:    float64(5),
					indicator.ParamKeyLookbackStart: float64(0),
					indicator.ParamKeyLookbackEnd:   float64(0),
				},
			},
			wantResult: indicator.ResultRejected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sig.EvaluateWithWindow(tt.klines, tt.config)
			if res.Result != tt.wantResult {
				t.Errorf("EvaluateWithWindow() Result = %v, want %v, msg=%s", res.Result, tt.wantResult, res.Message)
			}
		})
	}
}

func TestChangePctSignal_EvaluateWithWindow_MultiDay(t *testing.T) {
	sig := &changePctSignal{}
	sig.BaseSignal = indicator.NewBaseSignal(
		"03", "0~5%", "小涨",
		indicator.ValNumber,
		[]indicator.OperatorOption{
			{
				Operator: indicator.OpBetween,
				Label:    "区间内",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyMin, "下限", 0, "%"),
					signalutil.ParamNumber(indicator.ParamKeyMax, "上限", 5, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
		},
		&indicator.SignalConfig{
			Operator: indicator.OpBetween,
			Params: map[string]any{
				indicator.ParamKeyMin:           float64(0),
				indicator.ParamKeyMax:           float64(5),
				indicator.ParamKeyLookbackStart: float64(2),
				indicator.ParamKeyLookbackEnd:   float64(0),
			},
		},
	)

	// 近3天（2~0天前）每天涨幅都在 0~5% 范围内
	// 需要4条K线：klines[0]=今天, [1]=昨天, [2]=前天, [3]=大前天(用于计算前天涨跌幅)
	klinesAllPass := []*model.DailyKline{
		{Close: 1030}, // [0] 今天 +3.00% (相对[1])
		{Close: 1000}, // [1] 昨天 +2.04% (相对[2])
		{Close: 980},  // [2] 前天
		{Close: 960},  // [3] 大前天 (基准)
	}

	// 第二天涨幅超过5%，应该不通过
	klinesOneFail := []*model.DailyKline{
		{Close: 1030}, // [0] 今天 +3.00% (相对[1])
		{Close: 1050}, // [1] 昨天 +7.14% (相对[2]) → 超过5%
		{Close: 980},  // [2] 前天
		{Close: 960},  // [3] 大前天 (基准)
	}

	tests := []struct {
		name       string
		klines     []*model.DailyKline
		start      int
		end        int
		wantResult indicator.EvaluatedResult
	}{
		{
			name:       "近3天都在0~5%范围内",
			klines:     klinesAllPass,
			start:      2,
			end:        0,
			wantResult: indicator.ResultPassed,
		},
		{
			name:       "第2天超过5%范围",
			klines:     klinesOneFail,
			start:      2,
			end:        0,
			wantResult: indicator.ResultRejected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &indicator.SignalConfig{
				SignalID: "02004003",
				Params: map[string]any{
					indicator.ParamKeyMin:           float64(0),
					indicator.ParamKeyMax:           float64(5),
					indicator.ParamKeyLookbackStart: float64(float64(tt.start)),
					indicator.ParamKeyLookbackEnd:   float64(float64(tt.end)),
				},
			}
			res := sig.EvaluateWithWindow(tt.klines, cfg)
			if res.Result != tt.wantResult {
				t.Errorf("EvaluateWithWindow() Result = %v, want %v, msg=%s", res.Result, tt.wantResult, res.Message)
			}
		})
	}
}

func TestChangePctSignal_EvaluateWithWindow_InsufficientData(t *testing.T) {
	sig := &changePctSignal{}
	sig.BaseSignal = indicator.NewBaseSignal(
		"01", "测试信号", "测试",
		indicator.ValNumber,
		[]indicator.OperatorOption{
			{
				Operator: indicator.OpGT,
				Label:    "大于",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
		},
		&indicator.SignalConfig{
			Operator: indicator.OpGT,
			Params: map[string]any{
				indicator.ParamKeyThreshold:    float64(0),
				indicator.ParamKeyLookbackStart: float64(100),
				indicator.ParamKeyLookbackEnd:   float64(0),
			},
		},
	)

	klines := []*model.DailyKline{
		{Close: 1000},
		{Close: 900},
	}

	cfg := &indicator.SignalConfig{
		SignalID: "02004001",
		Params: map[string]any{
			indicator.ParamKeyLookbackStart: float64(100), // 远超数据长度
			indicator.ParamKeyLookbackEnd:   float64(0),
		},
	}

	res := sig.EvaluateWithWindow(klines, cfg)
	if res.Result == indicator.ResultPassed {
		t.Error("期望数据不足返回 ResultRejected")
	}
}

// ============================================================================
//  newChangePctSignal 单元测试 — 验证信号包含时间窗口参数
// ============================================================================

func TestNewChangePctSignal_ContainsLookbackParams(t *testing.T) {
	def := signalutil.RangeDef{
		Seq:         "04",
		Desc:        "大于5%",
		Alias:       "大涨",
		Operator:    indicator.OpGT,
		MinThreshold: 5,
		MaxThreshold: 0,
	}

	sig := newChangePctSignal(def, "涨跌幅")
	if sig == nil {
		t.Fatal("newChangePctSignal 返回 nil")
	}

	// 验证 Operators 包含时间窗口参数
	ops := sig.Operators()
	if len(ops) == 0 {
		t.Fatal("Operators 为空")
	}

	op := ops[0]
	foundLookbackStart := false
	foundLookbackEnd := false
	for _, p := range op.Params {
		if p.Key == indicator.ParamKeyLookbackStart {
			foundLookbackStart = true
		}
		if p.Key == indicator.ParamKeyLookbackEnd {
			foundLookbackEnd = true
		}
	}

	if !foundLookbackStart {
		t.Error("Operators 缺少 lookback_start 参数")
	}
	if !foundLookbackEnd {
		t.Error("Operators 缺少 lookback_end 参数")
	}
}

// ============================================================================
//  NewChangePct 集成测试：验证默认时间窗口为 0-0
// ============================================================================

func TestNewChangePct_DefaultWindowIsZeroZero(t *testing.T) {
	ind := NewChangePct()

	// 内置信号的默认配置应该是 0-0 天
	for _, sig := range ind.BuiltInSignals() {
		cfg := sig.DefaultConfig()
		if cfg == nil {
			t.Fatal("内置信号默认配置不应为空")
		}
		start := cfg.GetFloat64(indicator.ParamKeyLookbackStart, -1)
		end := cfg.GetFloat64(indicator.ParamKeyLookbackEnd, -1)
		if start != 0 || end != 0 {
			t.Errorf("信号 %q 默认窗口应为 0-0，实际 %d-%d", sig.Name(), int(start), int(end))
		}

		// 同时验证 Operators 包含时间窗口参数（前端可渲染）
		for _, op := range sig.Operators() {
			hasLookbackStart := false
			hasLookbackEnd := false
			for _, p := range op.Params {
				if p.Key == indicator.ParamKeyLookbackStart {
					hasLookbackStart = true
				}
				if p.Key == indicator.ParamKeyLookbackEnd {
					hasLookbackEnd = true
				}
			}
			if !hasLookbackStart || !hasLookbackEnd {
				t.Errorf("信号 %q 的操作符 %q 缺少时间窗口参数", sig.Name(), op.Label)
			}
		}
	}

	// 自定义信号的默认配置也应该是 0-0 天
	for _, sig := range ind.CustomSignals() {
		cfg := sig.DefaultConfig()
		if cfg == nil {
			t.Fatal("自定义信号默认配置不应为空")
		}
		start := cfg.GetFloat64(indicator.ParamKeyLookbackStart, -1)
		end := cfg.GetFloat64(indicator.ParamKeyLookbackEnd, -1)
		if start != 0 || end != 0 {
			t.Errorf("自定义信号 %q 默认窗口应为 0-0，实际 %d-%d", sig.Name(), int(start), int(end))
		}
	}
}
