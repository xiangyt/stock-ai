package backtest

import "context"

// ============================================================================
//  DataProvider — 数据提供者接口（可插拔）
//
//  负责为回测引擎提供股票历史数据。
//  默认实现：DB 查询 daily_kline 表。
//  测试时可用内存版实现替代。
// ============================================================================

// DataProvider 数据提供者接口。
type DataProvider interface {
	// FetchSnapshots 批量获取指定日期的股票数据快照。
	FetchSnapshots(ctx context.Context, codes []string, tradeDate int) ([]*StockSnapshot, error)
}
