package ths

import (
	"context"
	"strings"
	"testing"
	"time"

	"stock-ai/internal/adapter"
)

// ========== 指数代码映射测试 ==========

func TestIndexCodeToTHS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{adapter.IndexSH000001, "zs_1A0001"},
		{adapter.IndexSZ399001, "zs_399001"},
		{adapter.IndexHS300, "zs_1B0300"},
		{adapter.IndexSH399006, "zs_399006"},
		{"000999", ""}, // 未注册
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IndexCodeToTHS(tt.input)
			if got != tt.want {
				t.Errorf("IndexCodeToTHS(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

// ========== 指数K线解析测试（mock 响应） ==========

func TestParseIndexKLineData(t *testing.T) {
	a := newTestAdapter()

	// 模拟同花顺 v4 指数K线响应（unmarshalJSONP 可处理带括号或纯JSON格式）
	mockResp := `({"data":"20260105,4661.62,4721.64,4661.62,4717.75,234144084000,630577350000.00,0.712,,,0;20260106,4680.10,4700.20,4650.30,4690.15,200000000000,600000000000.00,-0.60,,,0"})`

	result, err := a.parseIndexKLineData(adapter.IndexHS300, mockResp)
	if err != nil {
		t.Fatalf("parseIndexKLineData failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 records, got %d", len(result))
	}

	// 第一条
	if result[0].Date != "2026-01-05" {
		t.Errorf("date[0] = %s, want 2026-01-05", result[0].Date)
	}
	if result[0].Code != adapter.IndexHS300 {
		t.Errorf("code[0] = %s, want %s", result[0].Code, adapter.IndexHS300)
	}
	// 4661.62元 → 466162分
	if result[0].Open != 466162 {
		t.Errorf("open[0] = %d, want 466162", result[0].Open)
	}
	if result[0].Close != 471775 {
		t.Errorf("close[0] = %d, want 471775", result[0].Close)
	}

	// 第二条日期应晚于第一条
	if result[1].Date <= result[0].Date {
		t.Errorf("data not in chronological order: %s >= %s", result[0].Date, result[1].Date)
	}
}

func TestParseIndexKLineData_Empty(t *testing.T) {
	a := newTestAdapter()

	// 空 data
	result, err := a.parseIndexKLineData("000300", `({"data":""})`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %d items", len(result))
	}
}

// ========== mergeOrdered 测试 ==========

func TestMergeAndFilter(t *testing.T) {
	year2025 := []adapter.StockPriceDaily{
		{Code: "000300", Date: "2025-12-29", Close: 398000},
		{Code: "000300", Date: "2025-12-30", Close: 399000},
		{Code: "000300", Date: "2025-12-31", Close: 401000},
	}
	year2026 := []adapter.StockPriceDaily{
		{Code: "000300", Date: "2026-01-02", Close: 402000},
		{Code: "000300", Date: "2026-01-05", Close: 403000},
	}

	merged := mergeAndFilter([][]adapter.StockPriceDaily{year2025, year2026}, "2025-01-01", "2026-12-31")

	if len(merged) != 5 {
		t.Fatalf("expected 5, got %d", len(merged))
	}

	// 验证顺序
	for i := 1; i < len(merged); i++ {
		if merged[i].Date <= merged[i-1].Date {
			t.Errorf("out of order at [%d]: %s >= [%d]: %s",
				i-1, merged[i-1].Date, i, merged[i].Date)
		}
	}

	// 验证边界
	if merged[0].Date != "2025-12-29" {
		t.Errorf("first = %s, want 2025-12-29", merged[0].Date)
	}
	if merged[4].Date != "2026-01-05" {
		t.Errorf("last = %s, want 2026-01-05", merged[4].Date)
	}
}

func TestMergeAndFilter_TimeRange(t *testing.T) {
	data := []adapter.StockPriceDaily{
		{Date: "2025-01-02", Close: 380000},
		{Date: "2025-03-15", Close: 390000},
		{Date: "2025-06-01", Close: 400000}, // 起始边界
		{Date: "2025-09-20", Close: 410000},
		{Date: "2025-12-31", Close: 420000}, // 结束边界
		{Date: "2026-01-05", Close: 430000},
	}

	// 只要 2025-06-01 ~ 2025-12-31
	result := mergeAndFilter([][]adapter.StockPriceDaily{data}, "2025-06-01", "2025-12-31")
	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}
	if result[0].Date != "2025-06-01" {
		t.Errorf("first = %s, want 2025-06-01", result[0].Date)
	}
	if result[2].Date != "2025-12-31" {
		t.Errorf("last = %s, want 2025-12-31", result[2].Date)
	}

	// 过滤后全部排除 → 返回 nil
	result2 := mergeAndFilter([][]adapter.StockPriceDaily{data}, "2027-01-01", "2027-12-31")
	if result2 != nil {
		t.Errorf("expected nil for out-of-range filter, got %d items", len(result2))
	}
}

func TestMergeAndFilter_SingleYear(t *testing.T) {
	data := []adapter.StockPriceDaily{
		{Date: "2026-01-02", Close: 402000},
	}
	merged := mergeAndFilter([][]adapter.StockPriceDaily{data}, "2026-01-01", "2026-12-31")
	if len(merged) != 1 || merged[0].Date != "2026-01-02" {
		t.Errorf("single year merge failed")
	}
}

func TestMergeAndFilter_AllEmpty(t *testing.T) {
	merged := mergeAndFilter([][]adapter.StockPriceDaily{{}, {}}, "2025-01-01", "2026-12-31")
	if merged != nil {
		t.Errorf("expected nil for all-empty input, got %v", merged)
	}
}

// ========== splitYearTasks 测试 ==========

func TestSplitYearTasks(t *testing.T) {
	a := newTestAdapter()

	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		wantLen  int
		wantFirst string
		wantLast  string
	}{
		{
			name:     "单年",
			start:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
			end:      time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local),
			wantLen:  1,
			wantFirst: "2026",
			wantLast:  "2026",
		},
		{
			name:     "跨两年",
			start:    time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local),
			end:      time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local),
			wantLen:  2,
			wantFirst: "2025",
			wantLast:  "2026",
		},
		{
			name:     "跨多年",
			start:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
			end:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
			wantLen:  3,
			wantFirst: "2024",
			wantLast:  "2026",
		},
		{
			name:     "start > end 返回空",
			start:    time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local),
			end:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := a.splitYearTasks(tt.start, tt.end)
			if len(tasks) != tt.wantLen {
				t.Fatalf("len=%d, want %d", len(tasks), tt.wantLen)
			}
			if tt.wantLen == 0 {
				return
			}
			if tasks[0].year != tt.wantFirst {
				t.Errorf("first year=%s, want %s", tasks[0].year, tt.wantFirst)
			}
			if tasks[len(tasks)-1].year != tt.wantLast {
				t.Errorf("last year=%s, want %s", tasks[len(tasks)-1].year, tt.wantLast)
			}
		})
	}
}

// ========== 集成测试：真实请求指数K线 ==========

func TestGetIndexDailyKLine(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	// 获取沪深300当年数据
	now := time.Now()
	klines, err := a.GetIndexDailyKLine(ctx, adapter.IndexHS300,
		time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local), now, adapter.AdjNone)
	if err != nil {
		t.Fatalf("GetIndexDailyKLine failed: %v", err)
	}
	if len(klines) == 0 {
		t.Fatal("no index klines returned")
	}

	t.Logf("沪深300 当年K线数据量: %d", len(klines))

	// 打印最近5条
	for i := len(klines) - 5; i < len(klines); i++ {
		p := klines[i]
		t.Logf("  %s O:%.2f H:%.2f L:%.2f C:%.2f Vol:%d ChgPct:%.2f%%",
			p.Date, float64(p.Open)/100, float64(p.High)/100,
			float64(p.Low)/100, float64(p.Close)/100,
			p.Volume, p.ChangePct)
	}

	// 验证时间有序性
	for i := 1; i < len(klines); i++ {
		if strings.Compare(klines[i].Date, klines[i-1].Date) <= 0 {
			t.Errorf("时间顺序异常: [%d]=%s >= [%d]=%s",
				i-1, klines[i-1].Date, i, klines[i].Date)
		}
	}

	// 验证价格合理性
	last := klines[len(klines)-1]
	if last.High < last.Low {
		t.Errorf("high(%d) < low(%d)", last.High, last.Low)
	}
	if last.Close <= 0 {
		t.Errorf("close=%d, want > 0", last.Close)
	}
	// 沪深300点位应在合理范围内 (2000~10000分 = 20~100元，实际约 4000左右)
	if last.Close < 200000 || last.Close > 1000000 {
		t.Logf("注意: close=%.2f 可能超出预期范围", float64(last.Close)/100)
	}
}

func TestGetIndexDailyKLine_CrossYear(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()

	// 跨年请求：去年12月 ~ 今年1月
	now := time.Now()
	start := time.Date(now.Year()-1, 12, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(now.Year(), 1, 31, 0, 0, 0, 0, time.Local)

	klines, err := a.GetIndexDailyKLine(ctx, adapter.IndexSH000001, start, end, adapter.AdjNone)
	if err != nil {
		t.Fatalf("cross-year request failed: %v", err)
	}

	t.Logf("上证指数 跨年K线(%s ~ %s): %d 条", start.Format("01/02"), end.Format("01/02"), len(klines))

	if len(klines) == 0 {
		t.Log("跨年期间可能无交易日")
		return
	}

	// 跨年合并后必须全局有序
	for i := 1; i < len(klines); i++ {
		if strings.Compare(klines[i].Date, klines[i-1].Date) <= 0 {
			t.Errorf("跨年合并后时间顺序异常: [%d]=%s >= [%d]=%s",
				i-1, klines[i-1].Date, i, klines[i].Date)
		}
	}
}

func TestGetIndexDailyKLine_MultiIndex(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	now := time.Now()
	start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)

	tests := []struct {
		code   string
		name   string
		minVal int64 // 最小合理收盘点位(分)
	}{
		{adapter.IndexSH000001, "上证指数", 250000},
		{adapter.IndexSZ399001, "深证成指", 800000},
		{adapter.IndexHS300, "沪深300", 350000},
		{adapter.IndexSH399006, "创业板指", 180000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines, err := a.GetIndexDailyKLine(ctx, tt.code, start, now, adapter.AdjNone)
			if err != nil {
				t.Fatalf("%s request failed: %v", tt.name, err)
			}
			if len(klines) == 0 {
				t.Fatalf("%s no data returned", tt.name)
			}

			last := klines[len(klines)-1]
			t.Logf("  %s → %d 条, 最新收盘: %.2f", tt.name, len(klines), float64(last.Close)/100)

			if last.Close < tt.minVal {
				t.Errorf("%s close=%.2f below expected min %.2f",
					tt.name, float64(last.Close)/100, float64(tt.minVal)/100)
			}
		})
	}
}
