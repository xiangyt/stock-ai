package ths

import (
	"context"
	"testing"

	"stock-ai/internal/adapter"
)

// ========== 实时数据测试 ==========

func TestGetTodayData(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	data, err := a.GetTodayData(ctx, testCode)
	if err != nil {
		t.Fatalf("GetTodayData(%s) failed: %v", testCode, err)
	}
	if data == nil {
		t.Fatal("GetTodayData returned nil")
	}

	if data.Date == "" {
		t.Error("Date is empty")
	}
	if data.Close <= 0 {
		t.Errorf("Close = %d, want > 0", data.Close)
	}

	t.Logf("%s 今日: O:%d H:%d L:%d C:%d Vol:%d",
		testCode, data.Open, data.High, data.Low, data.Close, data.Volume)
}

func TestGetThisWeekData(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	data, err := a.GetThisWeekData(ctx, testCode)
	if err != nil {
		t.Fatalf("GetThisWeekData(%s) failed: %v", testCode, err)
	}
	if data == nil {
		t.Fatal("GetThisWeekData returned nil")
	}

	t.Logf("本周: O:%d H:%d L:%d C:%d Vol:%d", data.Open, data.High, data.Low, data.Close, data.Volume)

	if data.Close <= 0 {
		t.Errorf("Close = %d, want > 0", data.Close)
	}
	if data.High < data.Low {
		t.Errorf("High(%d) < Low(%d)", data.High, data.Low)
	}
}

func TestGetThisMonthData(t *testing.T) {
	data := mustGetPeriodData(t, testCode, func(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
		a := newTestAdapter()
		defer a.Close()
		return a.GetThisMonthData(ctx, code)
	})

	t.Logf("本月: O:%d H:%d L:%d C:%d Vol:%d", data.Open, data.High, data.Low, data.Close, data.Volume)

	if data.Close <= 0 {
		t.Errorf("Close = %d, want > 0", data.Close)
	}
}

func TestGetThisQuarterData(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	data, err := a.GetThisQuarterData(ctx, testCode)
	if err != nil {
		t.Fatalf("GetThisQuarterData failed: %v", err)
	}
	if data == nil {
		t.Fatal("returned nil")
	}

	t.Logf("本季: O:%d H:%d L:%d C:%d Vol:%d", data.Open, data.High, data.Low, data.Close, data.Volume)
}

func TestGetThisYearData(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	data, err := a.GetThisYearData(ctx, testCode)
	if err != nil {
		t.Fatalf("GetThisYearData failed: %v", err)
	}
	if data == nil {
		t.Fatal("returned nil")
	}

	t.Logf("本年: O:%d H:%d L:%d C:%d Vol:%d", data.Open, data.High, data.Low, data.Close, data.Volume)
}

// ========== 多只股票实时数据批量测试 ==========

func TestRealtimeMultiStock(t *testing.T) {
	tests := []struct {
		code string
		name string
	}{
		{"600519", "贵州茅台"},
		{"300750", "宁德时代"},
		{"000001", "平安银行"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			data := mustGetTodayData(t, tt.code)
			if data.Code != tt.code {
				t.Errorf("Code = %q, want %q", data.Code, tt.code)
			}
			if data.Close <= 0 {
				t.Errorf("Close = %d, want > 0", data.Close)
			}
		})
	}
}

// ========== Adapter 基础信息测试 ==========

func TestAdapterNameAndType(t *testing.T) {
	a := New()

	if a.Name() != "tonghuashun" {
		t.Errorf("Name() = %q, want %q", a.Name(), "tonghuashun")
	}
	if a.DisplayName() != "同花顺" {
		t.Errorf("DisplayName() = %q, want %q", a.DisplayName(), "同花顺")
	}
	if a.Type() != "web_crawl" {
		t.Errorf("Type() = %q, want %q", a.Type(), "web_crawl")
	}
}

func TestQuotaInfo(t *testing.T) {
	a := New()
	q := a.GetQuotaInfo()

	if q.DailyLimit != -1 {
		t.Errorf("DailyLimit = %d, want -1", q.DailyLimit)
	}
	if q.RateLimit <= 0 {
		t.Errorf("RateLimit = %d, want > 0", q.RateLimit)
	}
	if q.Burst <= 0 {
		t.Errorf("Burst = %d, want > 0", q.Burst)
	}

	r, b := q.LimiterConfig()
	if r <= 0 {
		t.Errorf("LimiterConfig rate = %f, want > 0", r)
	}
	if b <= 0 {
		t.Errorf("LimiterConfig burst = %d, want > 0", b)
	}

	t.Logf("Quota: RateLimit=%d Burst=%d (limiter: %.1frps burst=%d)",
		q.RateLimit, q.Burst, r, b)
}
