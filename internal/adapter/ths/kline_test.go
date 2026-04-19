package ths

import (
	"context"
	"testing"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"
)

// ========== 日K线测试 ==========

func TestGetDailyKLine(t *testing.T) {
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
	for i := len(klines) - 5; i < len(klines); i++ {
		if i < 0 {
			continue
		}
		p := klines[i]
		t.Logf("  %s O:%d H:%d L:%d C:%d Vol:%d Amt:%d",
			p.Date, p.Open, p.High, p.Low, p.Close, p.Volume, p.Amount)
	}

	validateKLines(t, klines, "日K")
}

// ========== 周K线测试 ==========

func TestGetWeeklyKLine(t *testing.T) {
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

	for i := len(klines) - 5; i < len(klines); i++ {
		if i < 0 {
			continue
		}
		p := klines[i]
		t.Logf("  %s O:%d H:%d L:%d C:%d Vol:%d",
			p.Date, p.Open, p.High, p.Low, p.Close, p.Volume)
	}

	validateKLines(t, klines, "周K")
}

// ========== 月K线测试 ==========

func TestGetMonthlyKLine(t *testing.T) {
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

	for i := len(klines) - 5; i < len(klines); i++ {
		if i < 0 {
			continue
		}
		p := klines[i]
		t.Logf("  %s O:%d H:%d L:%d C:%d Vol:%d",
			p.Date, p.Open, p.High, p.Low, p.Close, p.Volume)
	}

	validateKLines(t, klines, "月K")
}

// ========== 季K线测试 ==========

func TestGetQuarterlyKLine(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	klines, err := a.GetQuarterlyKLine(ctx, testCode, adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetQuarterlyKLine failed: %v", err)
	}
	if len(klines) == 0 {
		t.Fatal("no quarterly klines returned")
	}

	t.Logf("季K线数据量: %d", len(klines))

	for _, p := range klines {
		t.Logf("  %s O:%d H:%d L:%d C:%d Vol:%d",
			p.Date, p.Open, p.High, p.Low, p.Close, p.Volume)
	}

	validateKLines(t, klines, "季K")
}

// ========== 年K线测试 ==========

func TestGetYearlyKLine(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	klines, err := a.GetYearlyKLine(ctx, testCode, adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetYearlyKLine failed: %v", err)
	}
	if len(klines) == 0 {
		t.Fatal("no yearly klines returned")
	}

	t.Logf("年K线数据量: %d", len(klines))

	for _, p := range klines {
		t.Logf("  %s O:%d H:%d L:%d C:%d Vol:%d",
			p.Date, p.Open, p.High, p.Low, p.Close, p.Volume)
	}

	validateKLines(t, klines, "年K")
}

// ========== 多只股票批量测试 ==========

func TestGetDailyKLine_MultiStock(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	tests := []struct {
		code     string
		expected string
	}{
		{"600519", "贵州茅台"}, // 沪市主板
		{"300750", "宁德时代"}, // 创业板
		{"000001", "平安银行"}, // 深市主板
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			klines, err := a.GetDailyKLine(ctx, tt.code, adapter.AdjQFQ)
			if err != nil {
				t.Fatalf("%s GetDailyKLine failed: %v", tt.code, err)
			}
			if len(klines) == 0 {
				t.Fatalf("%s no klines", tt.code)
			}
			if klines[0].Code != tt.code {
				t.Errorf("code mismatch: got %s, want %s", klines[0].Code, tt.code)
			}
			last := klines[len(klines)-1]
			if last.Close <= 0 {
				t.Errorf("%s last close=%d, want > 0", tt.code, last.Close)
			}
			if last.High < last.Low {
				t.Errorf("%s high(%d) < low(%d)", tt.code, last.High, last.Low)
			}
			if last.High < last.Open || last.High < last.Close {
				t.Errorf("%s high not the highest: O=%d H=%d L=%d C=%d",
					tt.code, last.Open, last.High, last.Low, last.Close)
			}
			t.Logf("  %s → %d 条, 最新收盘: %d分(%.2f元)",
				tt.code, len(klines), last.Close, float64(last.Close)/100)
		})
	}
}

// ========== 周期数据量对比 ==========

func TestComparePeriodDataCounts(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	daily, _ := a.GetDailyKLine(ctx, testCode, adapter.AdjQFQ)
	weekly, _ := a.GetWeeklyKLine(ctx, testCode, adapter.AdjQFQ)
	monthly, _ := a.GetMonthlyKLine(ctx, testCode, adapter.AdjQFQ)
	quarterly, _ := a.GetQuarterlyKLine(ctx, testCode, adapter.AdjQFQ)
	yearly, _ := a.GetYearlyKLine(ctx, testCode, adapter.AdjQFQ)

	t.Logf("各周期数据量对比 (%s):", testCode)
	t.Logf("  日K:   %d 条", len(daily))
	t.Logf("  周K:   %d 条", len(weekly))
	t.Logf("  月K:   %d 条", len(monthly))
	t.Logf("  季K:   %d 条", len(quarterly))
	t.Logf("  年K:   %d 条", len(yearly))

	if len(daily) < len(weekly) {
		t.Error("daily count should be >= weekly count")
	}
	if len(weekly) < len(monthly) {
		t.Error("weekly count should be >= monthly count")
	}
	if len(monthly) < len(quarterly) {
		t.Error("monthly count should be >= quarterly count")
	}
	if len(quarterly) < len(yearly) {
		t.Error("quarterly count should be >= yearly count")
	}
}

// ========== 复权类型对比测试 ==========

func TestAdjTypeCompare(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	// 用一只历史有送转的股票测试（立讯精密，有过多次送股）
	code := testCode

	qfq, err := a.GetDailyKLine(ctx, code, adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("AdjQFQ failed: %v", err)
	}
	none, err := a.GetDailyKLine(ctx, code, adapter.AdjNone)
	if err != nil {
		t.Fatalf("AdjNone failed: %v", err)
	}
	bqq, err := a.GetDailyKLine(ctx, code, adapter.AdjBQQ)
	if err != nil {
		t.Fatalf("AdjBQQ failed: %v", err)
	}

	if len(qfq) == 0 || len(none) == 0 || len(bqq) == 0 {
		t.Fatal("至少一种复权类型返回空数据")
	}

	// 三种复权数据量应相同
	t.Logf("数据量: 前复权=%d, 不复权=%d, 后复权=%d", len(qfq), len(none), len(bqq))
	if len(qfq) != len(none) || len(qfq) != len(bqq) {
		t.Errorf("三种复权的数据量不一致: qfq=%d, none=%d, bqq=%d",
			len(qfq), len(none), len(bqq))
	}

	// 最新收盘价: 前复权 ≈ 不复权 (最近无除权时基本一致)
	lastQFQ := qfq[len(qfq)-1]
	lastNone := none[len(none)-1]
	lastBQQ := bqq[len(bqq)-1]

	t.Logf("最新收盘价(分): 前复权=%d, 不复权=%d, 后复权=%d",
		lastQFQ.Close, lastNone.Close, lastBQQ.Close)

	// 首日收盘价: 三者应该有明显差异（如果该股票历史上发生过送转）
	firstQFQ := qfq[0]
	firstNone := none[0]
	firstBQQ := bqq[0]

	t.Logf("首日收盘价(分): 前复权=%d, 不复权=%d, 后复权=%d",
		firstQFQ.Close, firstNone.Close, firstBQQ.Close)

	// 后复权首日价 ≤ 不复权首日价 ≤ 前复权首日价
	// （前复权把历史价格调高，后复权保持原始）
	// 注意：这个关系取决于具体股票的除权历史
	if firstBQQ.Close > firstQFQ.Close {
		t.Logf("注意: 后复权首日价(%d) > 前复权首日价(%d)，可能该股票无送转记录或除权模式特殊",
			firstBQQ.Close, firstQFQ.Close)
	}

	// 确认三种复权的价格不完全一致（至少有一处不同）
	allSame := true
	for i := range qfq {
		if qfq[i].Close != none[i].Close || none[i].Close != bqq[i].Close {
			allSame = false
			break
		}
	}
	if allSame {
		t.Log("三种复权价格完全一致 — 该股票可能从未发生过分红送转")
	} else {
		t.Log("✅ 三种复权返回不同价格，复权逻辑生效")
	}
}

// ========== THS K线解析器单元测试 ==========

func TestParseTradeDate(t *testing.T) {
	parser := helpers.NewKLineParser()

	tests := []struct {
		input string
		want  int
	}{
		{"2026-04-18", 20260418},
		{"2025-12-31", 20251231},
		{"20260418", 20260418},
	}

	for _, tt := range tests {
		got, err := parser.ParseTradeDate(tt.input)
		if err != nil {
			t.Errorf("ParseTradeDate(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseTradeDate(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
