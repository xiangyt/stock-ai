package model

// ========== 股票每日估值快照 ==========
//
// 设计要点：
//   - 联合主键：(stock_code, trade_date)
//   - trade_date 用 INT 存储，格式 YYYYMMDD
//   - 无 created/updated 字段，快照是时序数据，可重复计算覆盖
//   - 所有字段均为计算值（从 K线 + 财报 + 股本变动 推导）

// StockDailySnapshot 股票每日估值快照
type StockDailySnapshot struct {
	StockCode string  `gorm:"primaryKey;size:10;not null;column:stock_code" json:"stock_code"`
	TradeDate int     `gorm:"primaryKey;not null;column:trade_date" json:"trade_date"` // YYYYMMDD

	// 估值指标 (倍)
	PEDynamic float64 `gorm:"column:pe_dynamic" json:"pe_dynamic"`       // 市盈率(动态)
	PEStatic  float64 `gorm:"column:pe_static" json:"pe_static"`         // 市盈率(静态)
	PETTM     float64 `gorm:"column:pe_ttm" json:"pe_ttm"`               // 市盈率(TTM)
	PSTTM     float64 `gorm:"column:ps_ttm" json:"ps_ttm"`               // 市销率(TTM)
	PB        float64 `gorm:"column:pb" json:"pb"`                       // 市净率

	// 盈利能力指标 (%)
	ROE        float64 `gorm:"column:roe" json:"roe"`         // 净资产收益率，取 performance_reports.roe_w
	ROA        float64 `gorm:"column:roa" json:"roa"`         // 总资产收益率，取 performance_reports.roa
	GrossMargin float64 `gorm:"column:gross_margin" json:"gross_margin"` // 毛利率，取 performance_reports.gross_margin
	NetMargin  float64 `gorm:"column:net_margin" json:"net_margin"`     // 净利率，取 performance_reports.net_margin

	// 每股指标 (元)
	BVPS    float64 `gorm:"column:bvps" json:"bvps"`     // 每股净资产
	BasicEPS float64 `gorm:"column:basic_eps" json:"basic_eps"` // 基本每股收益，取 performance_reports.basic_eps

	// 财报当期数据 (元)
	ParentNetProfit float64 `gorm:"column:parent_net_profit" json:"parent_net_profit"`     // 归属母公司净利润（最新一期财报）
	DeductNetProfit float64 `gorm:"column:deduct_net_profit" json:"deduct_net_profit"`     // 扣非净利润（最新一期财报）
	TotalRevenue    float64 `gorm:"column:total_revenue" json:"total_revenue"`             // 营业总收入（最新一期财报）

	// 股本数据 (股)
	TotalShares int64 `gorm:"column:total_shares" json:"total_shares"`   // 总股本
	FloatShares int64 `gorm:"column:float_shares" json:"float_shares"`   // 流通A股

	// 市值数据 (元)
	TotalMarketCap     float64 `gorm:"column:total_market_cap" json:"total_market_cap"`           // 总市值
	CirculateMarketCap float64 `gorm:"column:circulate_market_cap" json:"circulate_market_cap"`   // 流通市值

	// 偿债能力指标 (%)
	DebtRatio float64 `gorm:"column:debt_ratio" json:"debt_ratio"` // 资产负债率，取 performance_reports.debt_ratio
}

func (StockDailySnapshot) TableName() string { return "stock_daily_snapshot" }
