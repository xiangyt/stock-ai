package quotecache

// ============================================================================
//  分时行情数据结构
//
//  本期仅定义结构体，不填充数据。后续需扩展 adapter 数据源获取分时数据。
// ============================================================================

// MinuteBar 分钟级行情数据
type MinuteBar struct {
	Time   string `json:"time"`   // 时间 "09:35"
	Price  int64  `json:"price"`  // 当前价（分）
	Volume int64  `json:"volume"` // 累计成交量（股）
	// TODO: 后续扩展
	// AvgPrice int64 `json:"avg_price"` // 均价（分）
	// Amount   int64 `json:"amount"`    // 累计成交额（分）
}

// MinuteData 当日分时行情
// 包含当日所有分钟 bar 和昨日收盘价，用于支持分时图绘制、均价线计算等
type MinuteData struct {
	Bars     []MinuteBar `json:"bars"`      // 当日分时数据，按时间升序
	PreClose int64       `json:"pre_close"` // 昨日收盘价（分）
}
