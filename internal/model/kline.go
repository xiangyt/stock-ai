package model

// ========== K线数据（多周期） ==========
//
// 设计要点：
//   - trade_date 用 INT 存储，格式 YYYYMMDD
//   - 价格统一用 INT 存储单位：分
//   - 成交量用 BIGINT 单位：股；成交额用 BIGINT 单位：分
//   - 联合主键：(stock_code, trade_date)
//   - 无 created/updated 字段，K线是时序快照

// ---------- 日K ----------

// DailyKline 日K线表
type DailyKline struct {
	StockCode    string  `gorm:"primaryKey;size:10;not null" json:"stock_code"`
	TradeDate    int     `gorm:"primaryKey;not null" json:"trade_date"`         // YYYYMMDD
	Open         int     `json:"open"`
	High         int     `json:"high"`
	Low          int     `json:"low"`
	Close        int     `json:"close"`
	Volume       int64   `json:"volume"`
	Amount       int64   `json:"amount"`
	TurnoverRate float64 `gorm:"type:decimal(8,4)" json:"turnover_rate"`
}

func (DailyKline) TableName() string { return "daily_kline" }

// ---------- 周K ----------

// WeeklyKline 周K线表
type WeeklyKline struct {
	StockCode    string  `gorm:"primaryKey;size:10;not null" json:"stock_code"`
	TradeDate    int     `gorm:"primaryKey;not null" json:"trade_date"`         // 周最后交易日 YYYYMMDD
	Open         int     `json:"open"`
	High         int     `json:"high"`
	Low          int     `json:"low"`
	Close        int     `json:"close"`
	Volume       int64   `json:"volume"`
	Amount       int64   `json:"amount"`
	TurnoverRate float64 `gorm:"type:decimal(8,4)" json:"turnover_rate"`
}

func (WeeklyKline) TableName() string { return "weekly_kline" }

// ---------- 月K ----------

// MonthlyKline 月K线表
type MonthlyKline struct {
	StockCode    string  `gorm:"primaryKey;size:10;not null" json:"stock_code"`
	TradeDate    int     `gorm:"primaryKey;not null" json:"trade_date"`         // 月最后交易日 YYYYMMDD
	Open         int     `json:"open"`
	High         int     `json:"high"`
	Low          int     `json:"low"`
	Close        int     `json:"close"`
	Volume       int64   `json:"volume"`
	Amount       int64   `json:"amount"`
	TurnoverRate float64 `gorm:"type:decimal(8,4)" json:"turnover_rate"`
}

func (MonthlyKline) TableName() string { return "monthly_kline" }

// ---------- 年K ----------

// YearlyKline 年K线表
type YearlyKline struct {
	StockCode    string  `gorm:"primaryKey;size:10;not null" json:"stock_code"`
	TradeDate    int     `gorm:"primaryKey;not null" json:"trade_date"`         // 年最后一个交易日 YYYYMMDD
	Open         int     `json:"open"`
	High         int     `json:"high"`
	Low          int     `json:"low"`
	Close        int     `json:"close"`
	Volume       int64   `json:"volume"`
	Amount       int64   `json:"amount"`
	TurnoverRate float64 `gorm:"type:decimal(8,4)" json:"turnover_rate"`
}

func (YearlyKline) TableName() string { return "yearly_kline" }
