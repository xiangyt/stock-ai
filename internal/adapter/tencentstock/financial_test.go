package tencentstock

import (
	"context"
	"testing"
)

func TestGetPerformanceReports(t *testing.T) {
	a := newTestAdapter(t)
	reports, err := a.GetPerformanceReports(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetPerformanceReports 失败: %v", err)
	}
	if len(reports) == 0 {
		t.Fatal("财报列表为空")
	}
	t.Logf("财报列表: 共 %d 条，最新报告期=%s", len(reports), reports[0].ReportDate)
}

func TestGetLatestPerformanceReport(t *testing.T) {
	a := newTestAdapter(t)
	report, err := a.GetLatestPerformanceReport(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetLatestPerformanceReport 失败: %v", err)
	}
	if report == nil {
		t.Fatal("最新财报为空")
	}
	t.Logf("最新财报: 报告期=%s 类型=%s 营收=%.2f亿",
		report.ReportDate, report.ReportType, report.TotalRevenue/1e8)
}
