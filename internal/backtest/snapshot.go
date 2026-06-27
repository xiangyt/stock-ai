package backtest

import "stock-ai/internal/model"

// StockSnapshot 选股时刻的股票数据快照（值对象）。
type StockSnapshot struct {
	Code      string             // 股票代码
	Name      string             // 股票名称
	Price     float64            // 最新价（元）
	TradeDate int                // 交易日期 YYYYMMDD（用于懒加载历史K线）
	KLine     []*model.DailyKline // K线数据，[0]=最新，Close单位为分，可选预填充
}
