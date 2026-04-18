package eastmoney

import (
	"context"
	"strings"
	"testing"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"
)

// 测试用股票代码
const (
	testCode = "000001" // 平安银行
)

func newTestAdapter() *Adapter {
	a := New()
	_ = a.Init(nil)
	return a
}

// ========== 日K线测试 ==========

func TestGetDailyKLine(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	klines, err := a.GetDailyKLine(ctx, testCode, "", "", nil)
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
	klines, err := a.GetWeeklyKLine(ctx, testCode, "", "", nil)
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
	klines, err := a.GetMonthlyKLine(ctx, testCode, "", "", nil)
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
	klines, err := a.GetYearlyKLine(ctx, testCode, "", "", nil)
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

// ========== 日期范围查询测试 ==========

func TestGetDailyKLine_DateRange(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	// 查询2025年全年数据
	klines, err := a.GetDailyKLine(ctx, testCode, "2025-01-01", "2025-12-31", nil)
	if err != nil {
		t.Fatalf("date range query failed: %v", err)
	}

	t.Logf("日期范围(2025年) 日K线数量: %d", len(klines))
	if len(klines) == 0 {
		t.Fatal("expected data for 2025")
	}

	// 验证第一条在范围内
	firstDate := klines[0].Date
	if strings.Compare(firstDate, "2025-01-01") < 0 || strings.Compare(firstDate, "2026-01-01") >= 0 {
		t.Errorf("first date %s out of expected range [2025-01-01, 2025-12-31]", firstDate)
	}
	t.Log("  首条:", firstDate)

	lastDate := klines[len(klines)-1].Date
	t.Log("  尾条:", lastDate)
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
			klines, err := a.GetDailyKLine(ctx, tt.code, "", "", nil)
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

	daily, _ := a.GetDailyKLine(ctx, testCode, "", "", nil)
	weekly, _ := a.GetWeeklyKLine(ctx, testCode, "", "", nil)
	monthly, _ := a.GetMonthlyKLine(ctx, testCode, "", "", nil)
	yearly, _ := a.GetYearlyKLine(ctx, testCode, "", "", nil)

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

	// 东财K线字符串格式: 日期,开盘,收盘,最高,最低,成交量(手),成交额(元)
	tests := []struct {
		name       string
		input      string
		wantOpen   int64
		wantClose  int64
		wantVol    int64
	}{
		{
			name:      "正常数据",
			input:     "2026-04-18,10.50,11.20,11.30,10.30,123456,678901.00",
			wantOpen:  1050,
			wantClose: 1120,
			wantVol:   12345600,
		},
		{
			name:      "低价股",
			input:     "2026-04-18,3.25,3.38,3.40,3.20,50000,16500.00",
			wantOpen:  325,
			wantClose: 338,
			wantVol:   5000000,
		},
		{
			name:      "高价股",
			input:     "2026-04-18,1500.00,1520.00,1530.00,1490.00,10000,1510000.00",
			wantOpen:  150000,
			wantClose: 152000,
			wantVol:   1000000,
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

// ========== 进度回调测试 ==========

func TestGetDailyKLine_WithCallback(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	var callbackCalled bool
	cb := func(current, total int, message string) {
		callbackCalled = true
		t.Logf("  progress: %d/%d - %s", current, total, message)
	}

	_, err := a.GetDailyKLine(ctx, testCode, "2026-01-01", time.Now().Format("2006-01-02"), cb)
	if err != nil {
		t.Fatalf("GetDailyKLine with callback failed: %v", err)
	}
	if !callbackCalled {
		t.Error("callback was not called")
	}
}

// ========== 通用验证函数 ==========

// validateKLines 验证K线数据的完整性
// 注意：复权后的历史早期数据价格可能为负值（前复权正常现象）
func validateKLines(t *testing.T, klines []adapter.StockPriceDaily, period string) {
	// 时间顺序：从早到晚排列（东方财富默认返回升序）
	for i := 1; i < len(klines); i++ {
		if strings.Compare(klines[i].Date, klines[i-1].Date) <= 0 {
			t.Errorf("[%s] 时间顺序异常: [%d]=%s >= [%d]=%s",
				period, i-1, klines[i-1].Date, i, klines[i].Date)
		}
	}

	// OHLC逻辑校验（仅对非负价格做校验，负值/零值为复权或停牌等特殊数据）
	for i, p := range klines {
		isSpecialPrice := p.Open < 0 || p.Close < 0 || p.High < 0 || p.Low < 0 || p.Low == 0
		if isSpecialPrice {
			continue
		}
		if p.High < p.Low {
			t.Errorf("[%s] 第%d条(%s) High(%d) < Low(%d)",
				period, i, p.Date, p.High, p.Low)
		}
		if p.High < p.Open || p.High < p.Close {
			t.Errorf("[%s] 第%d条(%s) 非最高价: O=%d H:%d L:%d C:%d",
				period, i, p.Date, p.Open, p.High, p.Low, p.Close)
		}
		if p.Low > p.Open || p.Low > p.Close {
			t.Errorf("[%s] 第%d条(%s) 非最低价: O=%d H:%d L:%d C:%d",
				period, i, p.Date, p.Open, p.High, p.Low, p.Close)
		}
		if p.Volume < 0 {
			t.Errorf("[%s] 第%d条(%s) Volume=%d < 0", period, i, p.Date, p.Volume)
		}
		if p.Amount < 0 {
			t.Errorf("[%s] 第%d条(%s) Amount=%d < 0", period, i, p.Date, p.Amount)
		}
	}
}
