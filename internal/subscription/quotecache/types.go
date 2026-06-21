package quotecache

// ============================================================================
//  Priority 分级刷新
// ============================================================================

// Priority 股票刷新优先级。
type Priority int

const (
	PriorityLow    Priority = iota // 低优先级：10:30 / 12:00 / 14:00 定时刷新
	PriorityNormal                 // 普通优先级：每 5 分钟刷新
	PriorityHigh                   // 高优先级：每 1 分钟刷新（持仓股/Monitor 监控股）
)

// TTL returns the cache TTL for this priority.
func (p Priority) TTL() int {
	switch p {
	case PriorityHigh:
		return 60   // seconds
	case PriorityNormal:
		return 300  // seconds
	default:
		return 900  // seconds
	}
}

// ============================================================================
//  Data types — 所有类型本地定义，不依赖 adapter/model
// ============================================================================

// MinuteBar 分时 bar。
type MinuteBar struct {
	Time   string `json:"time"`   // "09:35"
	Price  int64  `json:"price"`  // 当前价（分）
	Volume int64  `json:"volume"` // 累计成交量（股）
	Amount int64  `json:"amount"` // 累计成交额（分）
}

// MinuteData 当日分时行情（quotecache 存储的唯一原始数据）。
type MinuteData struct {
	Bars     []MinuteBar `json:"bars"`      // 分时 bar，按时间升序
	PreClose int64       `json:"pre_close"` // 昨日收盘价（分）
	Date     string      `json:"date"`      // 交易日期 "2026-06-20"

	// 以下由 collectorAdapter 从 adapter.IntradayData 同步，供上层使用
	Name           string  `json:"name"`             // 股票名称
	Open           int64   `json:"open"`             // 今开（分）
	Current        int64   `json:"current"`          // 当前价（分）
	High           int64   `json:"high"`             // 最高（分）
	Low            int64   `json:"low"`              // 最低（分）
	Volume         int64   `json:"volume"`           // 成交量（股）
	Amount         int64   `json:"amount"`           // 成交额（分）
	Change         int64   `json:"change"`           // 涨跌额（分）
	ChangePct      float64 `json:"change_pct"`       // 涨跌幅 %
	Turnover       float64 `json:"turnover"`         // 换手率 %
	Pe             float64 `json:"pe"`               // 市盈率(TTM)
	Pb             float64 `json:"pb"`               // 市净率
	MarketCap      float64 `json:"market_cap"`       // 总市值（亿）
	FloatMarketCap float64 `json:"float_market_cap"` // 流通市值（亿）
	Amplitude      float64 `json:"amplitude"`        // 振幅 %
	Depth          *MarketDepth `json:"depth"`       // 五档买卖盘口
}

// MarketDepth 五档买卖盘口（本包自定，与 adapter.MarketDepth 结构一致）。
// 价格单位: 分, 数量单位: 股
type MarketDepth struct {
	Ask1Price  int64 `json:"ask1_price"`
	Ask1Volume int64 `json:"ask1_volume"` // 股
	Ask2Price  int64 `json:"ask2_price"`
	Ask2Volume int64 `json:"ask2_volume"`
	Ask3Price  int64 `json:"ask3_price"`
	Ask3Volume int64 `json:"ask3_volume"`
	Ask4Price  int64 `json:"ask4_price"`
	Ask4Volume int64 `json:"ask4_volume"`
	Ask5Price  int64 `json:"ask5_price"`
	Ask5Volume int64 `json:"ask5_volume"`
	Bid1Price  int64 `json:"bid1_price"`
	Bid1Volume int64 `json:"bid1_volume"` // 股
	Bid2Price  int64 `json:"bid2_price"`
	Bid2Volume int64 `json:"bid2_volume"`
	Bid3Price  int64 `json:"bid3_price"`
	Bid3Volume int64 `json:"bid3_volume"`
	Bid4Price  int64 `json:"bid4_price"`
	Bid4Volume int64 `json:"bid4_volume"`
	Bid5Price  int64 `json:"bid5_price"`
	Bid5Volume int64 `json:"bid5_volume"`
}

// QuoteData 推送给订阅者的行情快照。
type QuoteData struct {
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	Date      string          `json:"date"`       // 交易日期 "2026-06-20"
	Price     float64         `json:"price"`      // 当前价（元）
	PreClose  float64         `json:"pre_close"`  // 昨收价（元），急拉急跌计算基准
	ChangePct float64         `json:"change_pct"` // 涨跌幅 %
	Volume    int64           `json:"volume"`     // 成交量(股)
	Turnover  float64         `json:"turnover"`   // 换手率 %
	Minutes   []MinuteBarInfo `json:"minutes"`    // 分时价格（Monitor 急拉急跌用）

	// 扩展行情（从 adapter 直传，供 Monitor 过滤和展示）
	MarketCap      float64      `json:"market_cap"`       // 总市值（亿）
	FloatMarketCap float64      `json:"float_market_cap"` // 流通市值（亿）
	Pe             float64      `json:"pe"`               // 市盈率(TTM)
	Pb             float64      `json:"pb"`               // 市净率
	Amplitude      float64      `json:"amplitude"`        // 振幅 %
	Depth          *MarketDepth `json:"depth"`            // 五档买卖盘口
}

// MinuteBarInfo 分时价格精简视图。
type MinuteBarInfo struct {
	Time  string  `json:"time"`
	Price float64 `json:"price"` // 元
}

// QuoteEvent 行情更新事件。
type QuoteEvent struct {
	Code string
	Data *QuoteData
}

// QuoteSubscriber 行情订阅函数签名。
// 接收代码列表，返回事件 channel 和取消函数。
type QuoteSubscriber func(codes []string) (<-chan QuoteEvent, func())

// ============================================================================
//  Stats
// ============================================================================

// Stats 缓存统计。
type Stats struct {
	Size        int   `json:"size"`         // 缓存条目数
	HighCount   int   `json:"high_count"`   // 高优先级数量
	NormalCount int   `json:"normal_count"` // 普通优先级数量
	LowCount    int   `json:"low_count"`    // 低优先级数量
	HitCount    int64 `json:"hit_count"`    // 命中次数
	MissCount   int64 `json:"miss_count"`   // 未命中次数
}

// ============================================================================
//  CachedQuoteData — 缓存单元
// ============================================================================

// CachedQuoteData 缓存的行情数据。
//
// 唯一原始数据：Intraday（分时 bar）。
// 日线 OHLCV 由 adapter 直接提供，通过 Intraday.Open/Current/High/Low 等字段访问。
// 周/月/年暂不处理（TODO）。
//
// Meta 字段供上层通过 OnQuoteReady hook 注入扩展数据（如 Snapshot），
// quotecache 本身不关心其内容。
type CachedQuoteData struct {
	Code     string         // 股票代码
	Name     string         // 股票名称
	Intraday *MinuteData    // 分时行情（唯一原始数据）
	Meta     map[string]any // 扩展数据，由上层 hook 注入（如 snapshot, pe, pb 等）
}
