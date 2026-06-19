package model

// QuoteData 行情快照（Monitor 所需的简化视图）
// 从 quotecache.CachedQuoteData 扁平化提取，解耦 monitor 与 quotecache
type QuoteData struct {
	Code      string             `json:"code"`       // 股票代码
	Name      string             `json:"name"`        // 股票名称
	Price     float64            `json:"price"`       // 当前价(元)
	ChangePct float64            `json:"change_pct"`  // 涨跌幅%
	Volume    int64              `json:"volume"`      // 成交量
	Turnover  float64            `json:"turnover"`    // 换手率%
	Minutes   []MinuteBar        `json:"minutes"`     // 当日分时价格（用于急拉急跌计算）
	Snapshot  *StockDailySnapshot `json:"snapshot"`   // 完整快照(市值/PE/PB/ROE)
}

// MinuteBar 分钟级价格（Monitor 所需的最简视图）
type MinuteBar struct {
	Time  string  `json:"time"`  // "09:35"
	Price float64 `json:"price"` // 当前价(元)
}

// QuoteEvent 行情更新事件
// quotecache 刷新某只股票后通过 channel 推送给 Monitor
type QuoteEvent struct {
	Code string
	Data *QuoteData
}

// QuoteSubscriber 行情订阅函数签名
// 接收股票代码列表 → 返回事件 channel + 取消订阅函数
// Monitor 注入此函数，不直接依赖 quotecache
type QuoteSubscriber func(codes []string) (<-chan QuoteEvent, func())
