package tencentstock

import (
	"context"
	"time"

	"stock-ai/internal/adapter"
)

// GetDailyKLine 获取日K线（暂不支持）
func (a *Adapter) GetDailyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetWeeklyKLine 获取周K线（暂不支持）
func (a *Adapter) GetWeeklyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetMonthlyKLine 获取月K线（暂不支持）
func (a *Adapter) GetMonthlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetQuarterlyKLine 获取季K线（暂不支持）
func (a *Adapter) GetQuarterlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetYearlyKLine 获取年K线（暂不支持）
func (a *Adapter) GetYearlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetIndexDailyKLine 获取指数日K线（暂不支持）
func (a *Adapter) GetIndexDailyKLine(ctx context.Context, code string, startTime, endTime time.Time, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}
