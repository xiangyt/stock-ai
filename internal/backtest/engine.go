package backtest

import (
	"stock-ai/internal/adapter"
	"stock-ai/internal/model"
)

// Engine 回测引擎（同步版，由 Service 的 goroutine 调用）
type Engine struct{ KLineAdapter adapter.DataSource }

// NewEngine 创建回测引擎
func NewEngine(a adapter.DataSource) *Engine { return &Engine{KLineAdapter: a} }

// BacktestRequest 回测请求参数
type BacktestRequest struct {
	StockPool      []string
	StartDate      string
	EndDate        string
	InitialCapital float64
	ExitRules      model.ExitRules
	PositionRules  model.PositionRules
}

// BacktestResult 回测结果
type BacktestResult struct {
	Run       *BacktestRun
	Trades    []BacktestTrade
	Snapshots []DailySnapshot
}
