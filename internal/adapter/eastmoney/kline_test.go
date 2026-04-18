package eastmoney

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

	// 打印最近5条
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

// ========== 复权模式对比测试 ==========

func TestAdjTypeCompare(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	qfq, _ := a.GetDailyKLine(ctx, testCode, adapter.AdjQFQ)
	none, _ := a.GetDailyKLine(ctx, testCode, adapter.AdjNone)

	if len(qfq) != len(none) {
		t.Errorf("前复权(%d)和不复权(%d)数据量不同", len(qfq), len(none))
		return
	}

	// 前复权和不复权的最新价应该相同，历史价格不同
	lastQFQ := qfq[len(qfq)-1].Close
	lastNone := none[len(none)-1].Close
	if lastQFQ != lastNone {
		t.Errorf("最新收盘不一致: 前复权=%d 不复权=%d", lastQFQ, lastNone)
	}

	firstQFQ := qfq[0].Close
	firstNone := none[0].Close
	if firstQFQ == firstNone {
		t.Log("历史首条价格相同(可能该区间内无除权)")
	} else {
		t.Logf("历史首条价格不同(符合预期): 前复权=%d(%d元) 不复权=%d(%d元)",
			firstQFQ, firstQFQ/100, firstNone, firstNone/100)
	}

	t.Logf("前复权 最新=%.2f元 首日=%.2f元", float64(lastQFQ)/100, float64(firstQFQ)/100)
	t.Logf("不复权 最新=%.2f元 首日=%.2f元", float64(lastNone)/100, float64(firstNone)/100)
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
		{"600519", "贵州茅台"}, // 沪市
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
	yearly, _ := a.GetYearlyKLine(ctx, testCode, adapter.AdjQFQ)

	t.Logf("各周期数据量对比 (%s):", testCode)
	t.Logf("  日K:   %d 条", len(daily))
	t.Logf("  周K:   %d 条", len(weekly))
	t.Logf("  月K:   %d 条", len(monthly))
	t.Logf("  年K:   %d 条", len(yearly))

	if len(daily) < len(weekly) {
		t.Error("daily count should be >= weekly count")
	}
	if len(weekly) < len(monthly) {
		t.Error("weekly count should be >= monthly count")
	}
	if len(monthly) < len(yearly) {
		t.Error("monthly count should be >= yearly count")
	}
}

// ========== K线解析器单元测试 ==========

func TestParseDailyKline(t *testing.T) {
	parser := helpers.NewKLineParser()

	// 东财K线字符串格式(11字段): 日期,开盘,收盘,最高,最低,成交量(手),成交额(元),?,?,换手率
	tests := []struct {
		name         string
		input        string
		wantOpen     int64
		wantClose    int64
		wantVol      int64
		wantTurnover float64
	}{
		{
			name:         "正常数据",
			input:        "2026-04-18,10.50,11.20,11.30,10.30,123456,678901.00,-0.50,-0.05,105.20,3.25",
			wantOpen:     1050,
			wantClose:    1120,
			wantVol:      12345600,
			wantTurnover: 3.25,
		},
		{
			name:         "低价股",
			input:        "2026-04-18,3.25,3.38,3.40,3.20,50000,16500.00,-4.00,-0.14,98.50,2.15",
			wantOpen:     325,
			wantClose:    338,
			wantVol:      5000000,
			wantTurnover: 2.15,
		},
		{
			name:         "高价股",
			input:        "2026-04-18,1500.00,1520.00,1530.00,1490.00,10000,1510000.00,1.33,0.01,102.10,0.67",
			wantOpen:     150000,
			wantClose:    152000,
			wantVol:      1000000,
			wantTurnover: 0.67,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseDailyKline(testCode, tt.input)
			if err != nil {
				t.Fatalf("ParseDailyKline failed: %v", err)
			}
			if result.Open != tt.wantOpen {
				t.Errorf("Open = %d, want %d", result.Open, tt.wantOpen)
			}
			if result.Close != tt.wantClose {
				t.Errorf("Close = %d, want %d", result.Close, tt.wantClose)
			}
			if result.Volume != tt.wantVol {
				t.Errorf("Volume = %d, want %d", result.Volume, tt.wantVol)
			}
			if result.Turnover != tt.wantTurnover {
				t.Errorf("Turnover = %.2f, want %.2f", result.Turnover, tt.wantTurnover)
			}
			if result.Code != testCode {
				t.Errorf("Code = %q, want %q", result.Code, testCode)
			}
			// OHLC 逻辑校验
			if result.High < result.Low {
				t.Errorf("High(%d) < Low(%d)", result.High, result.Low)
			}
			if result.High < result.Open || result.High < result.Close {
				t.Error("High should be max(O,H,L,C)")
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

// ========== 通用验证函数（已移至 test_helper.go） ==========
