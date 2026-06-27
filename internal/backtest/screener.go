package backtest

import "context"

// ============================================================================
//  Screener — 选股执行器接口（可插拔）
//
//  目前唯一实现：复用 internal/indicator 包的 Engine。
//  未来可以有其他实现（如纯内存版、缓存版）。
// ============================================================================

// Screener 选股执行器，对股票池执行信号评估。
type Screener interface {
	// Screen 执行选股，返回每只股票的评估结果。
	Screen(ctx context.Context, stocks []*StockSnapshot, configs []SignalConfig) (*ScreenSummary, error)
}

// ScreenSummary 选股结果汇总。
type ScreenSummary struct {
	Total    int            `json:"total"`
	Passed   int            `json:"passed"`
	Rejected int            `json:"rejected"`
	Results  []ScreenResult `json:"results"`
}

// ScreenResult 单只股票的选股评估结果。
type ScreenResult struct {
	Code   string `json:"code"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}
