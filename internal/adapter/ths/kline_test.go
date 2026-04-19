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

// ========== THS K线解析器单元测试 ==========

func TestParseTHSDailyKline(t *testing.T) {
	parser := helpers.NewKLineParser()

	tests := []struct {
		name      string
		dates     []string
		prices    []string
		volumes   []string
		wantCount int
	}{
		{
			name: "正常多日数据",
			dates: []string{"20260414", "20260415", "20260416"},
			prices: []string{"10.5,11.2,11.3,10.3", "10.8,11.0,11.1,10.7", "10.9,11.5,11.6,10.8"},
			volumes: []string{"123456", "234567", "345678"},
			wantCount: 3,
		},
		{
			name: "单日数据",
			dates: []string{"20260418"},
			prices: []string{"1500,1520,1530,1490"},
			volumes: []string{"10000"},
			wantCount: 1,
		},
		{
			name:      "空数据",
			dates:     []string{},
			prices:    []string{},
			volumes:   []string{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseTHSDailyKline(testCode, tt.dates, tt.prices, tt.volumes)
			if err != nil {
				t.Fatalf("ParseTHSDailyKline failed: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("count = %d, want %d", len(result), tt.wantCount)
			}
			for i, p := range result {
				if p.Code != testCode {
					t.Errorf("[%d] Code = %q, want %q", i, p.Code, testCode)
				}
				if p.Date == "" && tt.wantCount > 0 {
					t.Errorf("[%d] Date is empty", i)
				}
				if p.Open <= 0 || p.Close <= 0 && tt.wantCount > 0 {
					t.Logf("[%d] O:%d H:%d L:%d C:%d (可能为停牌数据)", i, p.Open, p.High, p.Low, p.Close)
				}
			}
		})
	}
}

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
