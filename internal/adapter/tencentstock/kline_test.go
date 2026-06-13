package tencentstock

import (
	"context"
	"strings"
	"testing"
	"time"

	"stock-ai/internal/adapter"
)

// ========== 日K线测试 ==========

func TestGetDailyKLine_HFQ(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetDailyKLine(context.Background(), "688010", adapter.AdjBQQ)
	if err != nil {
		t.Fatalf("GetDailyKLine(hfq) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	first := data[0]
	t.Logf("日K后复权: 共%d条, 最新=%s, open=%.2f, close=%.2f, low=%.2f, high=%.2f, vol=%d",
		len(data), first.Date,
		float64(first.Open)/100, float64(first.Close)/100,
		float64(first.Low)/100, float64(first.High)/100, first.Volume)

	// 校验字段非零
	if first.Open == 0 || first.Close == 0 {
		t.Errorf("价格字段不应为0: %+v", first)
	}
	if first.High < first.Low {
		t.Errorf("最高价(%d) < 最低价(%d)", first.High, first.Low)
	}
	if !strings.HasPrefix(first.Code, "sh") && !strings.HasPrefix(first.Code, "sz") {
		t.Errorf("Code 应为腾讯格式(sh/sz): %s", first.Code)
	}
}

func TestGetDailyKLine_QFQ(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetDailyKLine(context.Background(), "688010", adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetDailyKLine(qfq) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("日K前复权: 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

func TestGetDailyKLine_None(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetDailyKLine(context.Background(), "688010", adapter.AdjNone)
	if err != nil {
		t.Fatalf("GetDailyKLine(none) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("日K不复权: 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

// ========== 周K线测试 ==========

func TestGetWeeklyKLine_HFQ(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetWeeklyKLine(context.Background(), "688010", adapter.AdjBQQ)
	if err != nil {
		t.Fatalf("GetWeeklyKLine(hfq) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	last := data[len(data)-1]
	t.Logf("周K后复权: 共%d条, 最新=%s, open=%.2f, close=%.2f, vol=%d",
		len(data), last.Date,
		float64(last.Open)/100, float64(last.Close)/100, last.Volume)
}

func TestGetWeeklyKLine_QFQ(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetWeeklyKLine(context.Background(), "600519", adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetWeeklyKLine(qfq) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("周K前复权(茅台): 共%d条, 最新=%s, close=%.2f",
		len(data), data[len(data)-1].Date, float64(data[len(data)-1].Close)/100)
}

func TestGetWeeklyKLine_None(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetWeeklyKLine(context.Background(), "688010", adapter.AdjNone)
	if err != nil {
		t.Fatalf("GetWeeklyKLine(none) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("周K不复权: 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

// ========== 月K线测试 ==========

func TestGetMonthlyKLine_HFQ(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetMonthlyKLine(context.Background(), "688010", adapter.AdjBQQ)
	if err != nil {
		t.Fatalf("GetMonthlyKLine(hfq) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("月K后复权: 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

func TestGetMonthlyKLine_QFQ(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetMonthlyKLine(context.Background(), "002404", adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetMonthlyKLine(qfq) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("月K前复权(002404): 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

// ========== 月K线补充测试 ==========

func TestGetMonthlyKLine_None(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetMonthlyKLine(context.Background(), "688010", adapter.AdjNone)
	if err != nil {
		t.Fatalf("GetMonthlyKLine(none) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("月K不复权: 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

// ========== 季K线测试（腾讯 newfqkline API 不支持 quarter 周期）==========

func TestGetQuarterlyKLine_Unsupported(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetQuarterlyKLine(context.Background(), "688010", adapter.AdjBQQ)
	if err == nil {
		t.Fatal("预期返回 ErrNotImplemented，实际 nil")
	}
	if err != adapter.ErrNotImplemented {
		t.Fatalf("预期 ErrNotImplemented，实际: %v", err)
	}
	t.Logf("季K不支持(预期): %v", err)
}

// ========== 年K线测试 ==========

func TestGetYearlyKLine_HFQ(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetYearlyKLine(context.Background(), "688010", adapter.AdjBQQ)
	if err != nil {
		t.Fatalf("GetYearlyKLine(hfq) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("年K后复权: 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

func TestGetYearlyKLine_QFQ(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetYearlyKLine(context.Background(), "688010", adapter.AdjQFQ)
	if err != nil {
		t.Fatalf("GetYearlyKLine(qfq) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("年K前复权: 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

func TestGetYearlyKLine_None(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetYearlyKLine(context.Background(), "688010", adapter.AdjNone)
	if err != nil {
		t.Fatalf("GetYearlyKLine(none) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	t.Logf("年K不复权: 共%d条, 最新=%s", len(data), data[len(data)-1].Date)
}

// ========== 深交所股票测试 ==========

func TestSZKLine(t *testing.T) {
	a := newTestAdapter(t)
	data, err := a.GetWeeklyKLine(context.Background(), "002404", adapter.AdjBQQ)
	if err != nil {
		t.Fatalf("GetWeeklyKLine(sz002404) 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("预期返回数据，实际为空")
	}
	first := data[0]
	if !strings.HasPrefix(first.Code, "sz") {
		t.Errorf("深交所股票 Code 应以 sz 开头: %s", first.Code)
	}
	t.Logf("sz002404 周K: 共%d条", len(data))
}

// ========== 边界测试 ==========

func TestGetWeeklyKLine_InvalidCode(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetWeeklyKLine(context.Background(), "999999", adapter.AdjBQQ)
	if err == nil {
		t.Fatal("预期无效代码返回错误，实际 nil")
	}
	t.Logf("无效代码错误(预期): %v", err)
}

func TestGetIndexDailyKLine_Unsupported(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetIndexDailyKLine(context.Background(), "000001", time.Time{}, time.Time{}, "")
	if err == nil {
		t.Fatal("预期返回错误，实际 nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("错误信息不匹配: %v", err)
	}
}
