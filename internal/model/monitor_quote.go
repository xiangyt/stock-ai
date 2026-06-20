package model

import "stock-ai/internal/subscription/quotecache"

// QuoteData 行情快照（Monitor 所需的简化视图）。
// 类型别名指向 quotecache.QuoteData，零拷贝零转换。
type QuoteData = quotecache.QuoteData

// MinuteBar 分钟级价格（Monitor 所需的最简视图）。
type MinuteBar = quotecache.MinuteBarInfo

// QuoteEvent 行情更新事件。
type QuoteEvent = quotecache.QuoteEvent

// QuoteSubscriber 行情订阅函数签名。
// 接收股票代码列表 → 返回事件 channel + 取消订阅函数。
type QuoteSubscriber = quotecache.QuoteSubscriber
