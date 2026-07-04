package market

import (
	"testing"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/model"
)

// ============================================================================
//  公共函数 calcLimitUpInfo 单元测试
// ============================================================================

func TestCalcLimitUpInfo_MainBoard10Pct(t *testing.T) {
	// 上日收盘 1034分（10.34元），涨停价 = floor(1034 × 1.10) = floor(1137.4) = 1137分（11.37元）
	klines := []*model.DailyKline{
		{Close: 1137}, // 今天涨停 — [0]
		{Close: 1034}, // 昨天收盘 — [1]
	}

	infos := calcLimitUpInfo(klines, model.BoardMain)

	if len(infos) != 1 {
		t.Fatalf("期望 1 条信息，实际 %d", len(infos))
	}

	info := infos[0]
	if !info.IsLimitUp {
		t.Error("期望 IsLimitUp=true（Close=1137 >= 涨停价=1137），实际 false")
	}
	if info.LimitPrice != 1137 {
		t.Errorf("期望涨停价 1137（分），实际 %d", info.LimitPrice)
	}
}

func TestCalcLimitUpInfo_FloorRounding(t *testing.T) {
	// 向下取整: 1034*1.10=1137.4 → floor=1137，收盘 1136 不满足
	klines := []*model.DailyKline{
		{Close: 1136}, // 差 1 分
		{Close: 1034},
	}

	infos := calcLimitUpInfo(klines, model.BoardMain)
	if infos[0].IsLimitUp {
		t.Error("期望 IsLimitUp=false（Close=1136 < 涨停价=1137），实际 true")
	}
}

func TestCalcLimitUpInfo_Chinext20Pct(t *testing.T) {
	// 创业板：上日收盘 5338分（53.38元），涨停价 = floor(5338 × 1.20) = floor(6405.6) = 6405分
	klines := []*model.DailyKline{
		{Close: 6405}, // 涨停 — [0]
		{Close: 5338}, // 昨天收盘 — [1]
	}

	infos := calcLimitUpInfo(klines, model.BoardChiNext)

	if infos[0].LimitPrice != 6405 {
		t.Errorf("期望涨停价 6405（分），实际 %d", infos[0].LimitPrice)
	}
	if !infos[0].IsLimitUp {
		t.Error("期望 IsLimitUp=true（6405 >= 6405），实际 false")
	}
}

func TestCalcLimitUpInfo_StarMarket20Pct(t *testing.T) {
	// 科创板：同创业板 20%
	klines := []*model.DailyKline{
		{Close: 6405},
		{Close: 5338},
	}

	infos := calcLimitUpInfo(klines, model.BoardStar)

	if infos[0].LimitPrice != 6405 {
		t.Errorf("期望涨停价 6405（分），实际 %d", infos[0].LimitPrice)
	}
}

func TestCalcLimitUpInfo_BSE30Pct(t *testing.T) {
	// 北交所：上日收盘 1000分（10.00元），涨停价 = floor(1000 × 1.30) = 1300分
	klines := []*model.DailyKline{
		{Close: 1300},
		{Close: 1000},
	}

	infos := calcLimitUpInfo(klines, model.BoardBSE)

	if infos[0].LimitPrice != 1300 {
		t.Errorf("期望涨停价 1300（分），实际 %d", infos[0].LimitPrice)
	}
}

func TestCalcLimitUpInfo_NotLimitUp(t *testing.T) {
	// 收盘价 1136 < 涨停价 1137，不算涨停
	klines := []*model.DailyKline{
		{Close: 1136},
		{Close: 1034},
	}

	infos := calcLimitUpInfo(klines, model.BoardMain)

	if infos[0].IsLimitUp {
		t.Error("期望 IsLimitUp=false（9.99% 未涨停），实际 true")
	}
}

func TestCalcLimitUpInfo_NotEnoughKlines(t *testing.T) {
	klines := []*model.DailyKline{
		{Close: 1137}, // 仅有1条，无法计算涨停价
	}
	infos := calcLimitUpInfo(klines, model.BoardMain)
	if infos != nil {
		t.Errorf("期望 nil（数据不足），实际 %v", infos)
	}
}

func TestCalcLimitUpInfo_CappedAt60(t *testing.T) {
	// 构造 65 根 K 线，验证只返回 60 条涨停信息
	klines := make([]*model.DailyKline, 65)
	for i := range klines {
		klines[i] = &model.DailyKline{Close: 1000 + i}
	}

	infos := calcLimitUpInfo(klines, model.BoardMain)

	if len(infos) != 60 {
		t.Errorf("期望截断为 60 条，实际 %d", len(infos))
	}
}

// ============================================================================
//  信号评估单元测试
// ============================================================================

func TestSignalLimitUpCount_Pass_TwoLimitsIn5Days(t *testing.T) {
	// 构造 5 天中有 2 天涨停
	infos := []LimitUpInfo{
		{IsLimitUp: true},  // [0] 今天涨停
		{IsLimitUp: false}, // [1]
		{IsLimitUp: true},  // [2] 3天前涨停
		{IsLimitUp: false}, // [3]
		{IsLimitUp: false}, // [4]
	}

	sig := NewBuiltInLimitUpCount()
	config := &indicator.SignalConfig{
		Operator: indicator.OpCustom,
		Params: map[string]any{
			indicator.ParamKeyLookbackStart: float64(4),
			indicator.ParamKeyLookbackEnd:   float64(0),
			paramMinCount:                    float64(2),
		},
	}

	res := sig.Evaluate(infos, config)
	if res.Result != indicator.ResultPassed {
		t.Errorf("期望通过（2次涨停>=2），实际 %d: %s", res.Result, res.Message)
	}
}

func TestSignalLimitUpCount_Reject_OnlyOneLimit(t *testing.T) {
	// 仅有 1 次涨停，不满足 ≥2
	infos := []LimitUpInfo{
		{IsLimitUp: true},
		{IsLimitUp: false},
		{IsLimitUp: false},
		{IsLimitUp: false},
		{IsLimitUp: false},
	}

	sig := NewBuiltInLimitUpCount()
	config := &indicator.SignalConfig{
		Operator: indicator.OpCustom,
		Params: map[string]any{
			indicator.ParamKeyLookbackStart: float64(4),
			indicator.ParamKeyLookbackEnd:   float64(0),
			paramMinCount:                    float64(2),
		},
	}

	res := sig.Evaluate(infos, config)
	if res.Result != indicator.ResultRejected {
		t.Errorf("期望拒绝（仅1次涨停），实际 %d", res.Result)
	}
}

func TestSignalLimitUpCount_InsufficientData(t *testing.T) {
	// 仅 3 天数据，但窗口需要 5 天（start=4）
	infos := []LimitUpInfo{
		{IsLimitUp: true}, {IsLimitUp: true}, {IsLimitUp: true},
	}

	sig := NewBuiltInLimitUpCount()
	config := &indicator.SignalConfig{
		Operator: indicator.OpCustom,
		Params: map[string]any{
			indicator.ParamKeyLookbackStart: float64(4),
			indicator.ParamKeyLookbackEnd:   float64(0),
			paramMinCount:                    float64(2),
		},
	}

	res := sig.Evaluate(infos, config)
	if res.Result != indicator.ResultRejected {
		t.Errorf("期望拒绝（数据不足），实际 %d", res.Result)
	}
}

func TestSignalLimitUpCount_StartExceedsMax(t *testing.T) {
	// lookback_start >= maxLimitDays(=60)，即 start=60 才报错
	infos := make([]LimitUpInfo, 65)

	sig := NewBuiltInLimitUpCount()
	config := &indicator.SignalConfig{
		SignalID: "02005101",
		Operator: indicator.OpCustom,
		Params: map[string]any{
			indicator.ParamKeyLookbackStart: float64(60),
			indicator.ParamKeyLookbackEnd:   float64(0),
			paramMinCount:                    float64(1),
		},
	}

	res := sig.Evaluate(infos, config)
	if res.Result != indicator.ResultRejected {
		t.Errorf("期望拒绝（start=%d >= maxLimitDays=%d），实际 %d", 60, maxLimitDays, res.Result)
	}
}

func TestSignalLimitUpCount_CustomWindow(t *testing.T) {
	// 自定义窗口：近 3 日（start=2, end=0），1 次涨停即满足
	infos := []LimitUpInfo{
		{IsLimitUp: true},
		{IsLimitUp: false},
		{IsLimitUp: false},
	}

	sig := NewBuiltInLimitUpCount()
	config := &indicator.SignalConfig{
		SignalID: "02005101", // 完整自定义 SignalID（IsCustom=true）
		Operator: indicator.OpCustom,
		Params: map[string]any{
			indicator.ParamKeyLookbackStart: float64(2),
			indicator.ParamKeyLookbackEnd:   float64(0),
			paramMinCount:                    float64(1),
		},
	}

	res := sig.Evaluate(infos, config)
	if res.Result != indicator.ResultPassed {
		t.Errorf("期望通过（1次涨停>=1），实际 %d: %s", res.Result, res.Message)
	}
}

// ============================================================================
//  getLimitRatio 单元测试
// ============================================================================

func TestGetLimitRatio(t *testing.T) {
	tests := []struct {
		name, board string
		wantRatio   float64
	}{
		{"主板", model.BoardMain, 0.10},
		{"中小板", model.BoardSME, 0.10},
		{"创业板", model.BoardChiNext, 0.20},
		{"科创板", model.BoardStar, 0.20},
		{"北交所", model.BoardBSE, 0.30},
		{"未知板块", "", 0.10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLimitRatio(tt.board)
			if got != tt.wantRatio {
				t.Errorf("getLimitRatio(%q) = %.2f，期望 %.2f", tt.board, got, tt.wantRatio)
			}
		})
	}
}
