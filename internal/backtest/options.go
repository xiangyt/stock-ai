package backtest

// ============================================================================
//  EngineOption — go-micro 风格 Options 模式
// ============================================================================

// EngineOption Engine 配置选项。
type EngineOption func(*defaultEngine)

// WithScreener 注入选股执行器。
func WithScreener(s Screener) EngineOption {
	return func(e *defaultEngine) { e.screener = s }
}

// WithPositionManager 注入仓位分配器。
func WithPositionManager(p PositionManager) EngineOption {
	return func(e *defaultEngine) { e.positionManager = p }
}

// WithExitExecutor 注入卖股执行器。
func WithExitExecutor(x ExitExecutor) EngineOption {
	return func(e *defaultEngine) { e.exitExecutor = x }
}

// WithFeeCalculator 注入费用计算器。
func WithFeeCalculator(f FeeCalculator) EngineOption {
	return func(e *defaultEngine) { e.feeCalculator = f }
}

// WithTradeRecorder 注入交易记录管理器。
func WithTradeRecorder(r TradeRecorder) EngineOption {
	return func(e *defaultEngine) { e.tradeRecorder = r }
}

// WithDataProvider 注入数据提供者。
func WithDataProvider(p DataProvider) EngineOption {
	return func(e *defaultEngine) { e.dataProvider = p }
}

// WithConcurrency 设置选股并发度。
func WithConcurrency(n int) EngineOption {
	return func(e *defaultEngine) { e.concurrency = n }
}
