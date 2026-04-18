package eastmoney

import (
	"context"
	"fmt"

	"stock-ai/internal/adapter"
)

// ========== 财务数据（财报） ==========

// GetPerformanceReports 获取业绩报表
func (a *Adapter) GetPerformanceReports(ctx context.Context, code string) ([]adapter.PerformanceReport, error) {
	// TODO: 移植 stock 项目中东方财富的业绩报表逻辑
	return nil, fmt.Errorf("not implemented")
}

// GetLatestPerformanceReport 获取最新业绩报表
func (a *Adapter) GetLatestPerformanceReport(ctx context.Context, code string) (*adapter.PerformanceReport, error) {
	reports, err := a.GetPerformanceReports(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("no reports for %s", code)
	}
	latest := reports[0]
	for i := 1; i < len(reports); i++ {
		if reports[i].ReportDate > latest.ReportDate {
			latest = reports[i]
		}
	}
	return &latest, nil
}
