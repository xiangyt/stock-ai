package backtest

import (
	"context"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/model"
)

// ============================================================================
//  defaultScreener — Screener 的默认实现（复用 indicator.Engine）
// ============================================================================

type defaultScreener struct {
	engine      *indicator.Engine
	concurrency int
}

// NewScreener 创建选股执行器。
func NewScreener(indicators []indicator.Indicator, concurrency int) Screener {
	return &defaultScreener{
		engine:      indicator.NewEngine(indicators),
		concurrency: concurrency,
	}
}

// Screen 实现 Screener，对股票池执行信号评估（复用 indicator.Engine）。
func (s *defaultScreener) Screen(
	ctx context.Context,
	stocks []*StockSnapshot, configs []SignalConfig,
) (*ScreenSummary, error) {
	if len(stocks) == 0 || len(configs) == 0 {
		return &ScreenSummary{}, nil
	}

	// 1. StockSnapshot → indicator.StockSource
	sources := make([]indicator.StockSource, 0, len(stocks))
	for _, stock := range stocks {
		sources = append(sources, newSnapshotSource(stock))
	}

	// 2. SignalConfig → indicator.SignalConfig
	icfgs := convertSignalConfigs(configs)

	// 3. 调用旧 engine.Execute
	results := s.engine.Execute(sources, icfgs, s.concurrency)

	// 4. 转换结果
	summary := &ScreenSummary{
		Total:   len(stocks),
		Results: make([]ScreenResult, 0, len(results)),
	}
	for _, r := range results {
		sr := ScreenResult{
			Code:   r.Code,
			Passed: r.Result == indicator.ResultPassed,
			Reason: r.Message,
		}
		if sr.Passed {
			summary.Passed++
		} else {
			summary.Rejected++
		}
		summary.Results = append(summary.Results, sr)
	}
	return summary, nil
}

// ============================================================================
//  snapshotSource — StockSnapshot → indicator.StockSource 适配器
// ============================================================================

var _ indicator.StockSource = (*snapshotSource)(nil)

type snapshotSource struct {
	code    string
	name    string
	dailyKL []*model.DailyKline
}

func newSnapshotSource(snap *StockSnapshot) *snapshotSource {
	return &snapshotSource{
		code:    snap.Code,
		name:    snap.Name,
		dailyKL: snap.KLine,
	}
}

// GetCode 实现 StockSource，返回股票代码。
func (s *snapshotSource) GetCode() string { return s.code }

// GetName 实现 StockSource，返回股票名称。
func (s *snapshotSource) GetName() string { return s.name }

// GetDailyKline 实现 TechnicalSource，返回内存中的日线数据。
func (s *snapshotSource) GetDailyKline() ([]*model.DailyKline, error) { return s.dailyKL, nil }

// GetWeeklyKline 实现 TechnicalSource（快照源不提供周线数据）。
func (s *snapshotSource) GetWeeklyKline() ([]*model.WeeklyKline, error) { return nil, nil }

// GetMonthlyKline 实现 TechnicalSource（快照源不提供月线数据）。
func (s *snapshotSource) GetMonthlyKline() ([]*model.MonthlyKline, error) { return nil, nil }

// GetYearlyKline 实现 TechnicalSource（快照源不提供年线数据）。
func (s *snapshotSource) GetYearlyKline() ([]*model.YearlyKline, error) { return nil, nil }

// GetDailySnapshot 实现 MarketSource（快照源不提供实时行情快照）。
func (s *snapshotSource) GetDailySnapshot() (*model.StockDailySnapshot, error) { return nil, nil }

// GetDetail 实现 FundamentalSource（快照源不提供公司详情）。
func (s *snapshotSource) GetDetail() (*model.Stock, error) { return nil, nil }

// GetPerformanceReport 实现 FinancialSource（快照源不提供财报数据）。
func (s *snapshotSource) GetPerformanceReport() ([]*model.PerformanceReport, error) { return nil, nil }

// GetShareholderCount 实现 FinancialSource（快照源不提供股东数据）。
func (s *snapshotSource) GetShareholderCount() (*model.ShareholderCount, error) { return nil, nil }
