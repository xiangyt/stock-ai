package backtest

// ============================================================================
//  EngineFactory — 回测引擎工厂
//
//  每次 Initiate 调用通过工厂创建独立的 Engine 实例，
//  避免 positionManager / tradeRecorder 等有状态组件在并发回测时互相干扰。
// ============================================================================

// EngineFactory 回测引擎工厂，持有可复用的无状态组件。
type EngineFactory struct {
	screener      Screener
	feeCalculator FeeCalculator
	dataProvider  DataProvider
}

// NewEngineFactory 创建引擎工厂。
func NewEngineFactory(screener Screener, feeCalculator FeeCalculator) *EngineFactory {
	return &EngineFactory{
		screener:      screener,
		feeCalculator: feeCalculator,
	}
}

// SetDataProvider 设置数据提供者（可选，在 Wire 注入后补充设置）。
func (f *EngineFactory) SetDataProvider(p DataProvider) {
	f.dataProvider = p
}

// NewEngine 为一次回测运行创建全新的引擎实例。
//
// 每次调用返回独立的 defaultEngine，其 positionManager 和 tradeRecorder
// 互不共享，保证并发安全。
func (f *EngineFactory) NewEngine() (Engine, error) {
	opts := []EngineOption{
		WithScreener(f.screener),
		WithFeeCalculator(f.feeCalculator),
		WithTradeRecorder(NewDBTradeRecorder()),
	}
	if f.dataProvider != nil {
		opts = append(opts, WithDataProvider(f.dataProvider))
	}
	return NewEngine(opts...)
}
