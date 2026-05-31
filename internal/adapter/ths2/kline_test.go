package ths2

import (
	"context"
	"testing"

	"stock-ai/internal/adapter"
)

// ========== 日K线测试 ==========

func TestGetDailyKLine(t *testing.T) {
	skipIfNoAuth(t)

	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	klines, err := a.GetDailyKLine(ctx, testCode, adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetDailyKLine failed: %v", err)
	}
	if len(klines) == 0 {
		t.Fatal("no daily klines returned")
	}

	t.Logf("日K线数据量: %d", len(klines))

	// 打印最近5条
	start := len(klines) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(klines); i++ {
		p := klines[i]
		t.Logf("  %s O:%d H:%d L:%d C:%d Vol:%d Amt:%d",
			p.Date, p.Open, p.High, p.Low, p.Close, p.Volume, p.Amount)
	}

	validateKLines(t, klines, "日K")
}

// ========== 周K线测试 ==========

func TestGetWeeklyKLine(t *testing.T) {
	skipIfNoAuth(t)

	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	klines, err := a.GetWeeklyKLine(ctx, testCode, adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetWeeklyKLine failed: %v", err)
	}
	if len(klines) == 0 {
		t.Fatal("no weekly klines returned")
	}

	t.Logf("周K线数据量: %d", len(klines))

	start := len(klines) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(klines); i++ {
		p := klines[i]
		t.Logf("  %s O:%d H:%d L:%d C:%d Vol:%d",
			p.Date, p.Open, p.High, p.Low, p.Close, p.Volume)
	}

	validateKLines(t, klines, "周K")
}

// ========== 月K线测试 ==========

func TestGetMonthlyKLine(t *testing.T) {
	// skipIfNoAuth(t)

	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	klines, err := a.GetMonthlyKLine(ctx, testCode, adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetMonthlyKLine failed: %v", err)
	}
	if len(klines) == 0 {
		t.Fatal("no monthly klines returned")
	}

	t.Logf("月K线数据量: %d", len(klines))

	start := len(klines) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(klines); i++ {
		p := klines[i]
		t.Logf("  %s O:%d H:%d L:%d C:%d Vol:%d",
			p.Date, p.Open, p.High, p.Low, p.Close, p.Volume)
	}

	validateKLines(t, klines, "月K")
}

// ========== 沪市股票测试 ==========

func TestGetDailyKLine_SH(t *testing.T) {
	skipIfNoAuth(t)

	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	klines, err := a.GetDailyKLine(ctx, testCodeSH, adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetDailyKLine(SH) failed: %v", err)
	}
	if len(klines) == 0 {
		t.Fatal("no klines for SH stock")
	}

	t.Logf("沪市 %s 日K: %d 条", testCodeSH, len(klines))

	last := klines[len(klines)-1]
	if last.Close <= 0 {
		t.Errorf("close=%d, want > 0", last.Close)
	}
	if last.High < last.Low {
		t.Errorf("high(%d) < low(%d)", last.High, last.Low)
	}
}

// ========== 复权类型测试 ==========
// 注: quota-h API adjust_type="none" 返回空数据，仅测前/后复权

func TestGetDailyKLine_AdjTypes(t *testing.T) {
	skipIfNoAuth(t)

	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	testCases := []struct {
		adjType string
		label   string
	}{
		{adapter.AdjQFQ, "前复权"},
		{adapter.AdjBQQ, "后复权"},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			klines, err := a.GetDailyKLine(ctx, testCode, tc.adjType)
			if err != nil {
				t.Fatalf("GetDailyKLine(%s) failed: %v", tc.label, err)
			}
			if len(klines) == 0 {
				t.Fatal("no klines returned")
			}
			last := klines[len(klines)-1]
			t.Logf("  %s: %d条, 最新C:%d分(%.2f元)",
				tc.label, len(klines), last.Close, float64(last.Close)/100)
		})
	}
}

// ========== 周期数据量对比 ==========

func TestComparePeriodCounts(t *testing.T) {
	skipIfNoAuth(t)

	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	daily, _ := a.GetDailyKLine(ctx, testCode, adapter.AdjQFQ)
	weekly, _ := a.GetWeeklyKLine(ctx, testCode, adapter.AdjQFQ)
	monthly, _ := a.GetMonthlyKLine(ctx, testCode, adapter.AdjQFQ)

	t.Logf("各周期数据量对比 (%s):", testCode)
	t.Logf("  日K: %d 条", len(daily))
	t.Logf("  周K: %d 条", len(weekly))
	t.Logf("  月K: %d 条", len(monthly))

	if len(daily) < len(weekly) {
		t.Error("daily count should be >= weekly count")
	}
	if len(weekly) < len(monthly) {
		t.Error("weekly count should be >= monthly count")
	}
}
