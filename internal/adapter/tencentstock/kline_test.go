package tencentstock

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGetDailyKLine_Unsupported(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetDailyKLine(context.Background(), "002404", "")
	if err == nil {
		t.Fatal("预期返回错误，实际 nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("错误信息不匹配: %v", err)
	}
}

func TestGetWeeklyKLine_Unsupported(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetWeeklyKLine(context.Background(), "002404", "")
	if err == nil {
		t.Fatal("预期返回错误，实际 nil")
	}
}

func TestGetMonthlyKLine_Unsupported(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetMonthlyKLine(context.Background(), "002404", "")
	if err == nil {
		t.Fatal("预期返回错误，实际 nil")
	}
}

func TestGetQuarterlyKLine_Unsupported(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetQuarterlyKLine(context.Background(), "002404", "")
	if err == nil {
		t.Fatal("预期返回错误，实际 nil")
	}
}

func TestGetYearlyKLine_Unsupported(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetYearlyKLine(context.Background(), "002404", "")
	if err == nil {
		t.Fatal("预期返回错误，实际 nil")
	}
}

func TestGetIndexDailyKLine_Unsupported(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.GetIndexDailyKLine(context.Background(), "000001", time.Now(), time.Now(), "")
	if err == nil {
		t.Fatal("预期返回错误，实际 nil")
	}
}
