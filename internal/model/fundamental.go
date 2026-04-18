package model

// ========== 基本面/财务面数据（F10） ==========
//
// 设计要点：
//   - 联合主键：(stock_code, report_date)
//   - report_date 用 string 存储 YYYY-MM-DD
//   - 无 created/updated 字段，财报是时序快照

// ---------- 财报(业绩报表) ----------

// PerformanceReport 财报表（东财 RPT_F10_FINANCE_MAINFINADATA）
type PerformanceReport struct {
	StockCode   string  `gorm:"primaryKey;size:10;not null" json:"stock_code"`
	ReportDate  string  `gorm:"primaryKey;size:10;not null" json:"report_date"` // YYYY-MM-DD
	ReportType  string  `gorm:"size:20" json:"report_type"`                     // 年报/一季报/中报/三季报
	ReportName  string  `gorm:"size:50" json:"report_name"`                     // 2025年报
	Currency    string  `gorm:"size:10" json:"currency"`                        // CNY

	// 每股指标
	BasicEPS       float64 `json:"basic_eps"`                 // 基本每股收益(元)
	DeductedEPS    float64 `json:"deducted_eps"`              // 扣非每股收益(元)
	DilutedEPS     float64 `json:"diluted_eps"`               // 摊薄每股收益(元)
	BVPS           float64 `json:"bvps"`                      // 每股净资产(元)
	EquityReserve  float64 `json:"equity_reserve"`             // 每股公积金
	UndistProfit   float64 `json:"undistributed_profit"`      // 每股未分配利润(元)
	OCFPS          float64 `json:"ocfps"`                     // 每股经营现金流(元)

	// 成长能力
	TotalRevenue        float64 `json:"total_revenue"`          // 营业总收入(元)
	GrossProfit         float64 `json:"gross_profit"`           // 毛利润(元)
	ParentNetProfit     float64 `json:"parent_net_profit"`      // 归属净利润(元)
	DeductNetProfit     float64 `json:"deduct_net_profit"`      // 扣非净利润(元)
	RevenueYoY          float64 `json:"revenue_yoy"`            // 营收同比(%)
	ParentNetProfitYoY  float64 `json:"parent_net_profit_yoy"`  // 归母净利同比(%)
	DeductNetProfitYoY  float64 `json:"deduct_net_profit_yoy"`  // 扣非净利同比(%)

	// 盈利能力
	ROEW           float64 `json:"roe_w"`            // 净资产收益率-加权(%)
	ROEDW          float64 `json:"roe_dw"`           // 净资产收益率-扣非加权(%)
	ROA            float64 `json:"roa"`              // 总资产收益率(%)
	GrossMargin    float64 `json:"gross_margin"`     // 销售毛利率(%)
	NetMargin      float64 `json:"net_margin"`       // 销售净利率(%)

	// 偿债能力
	CurrentRatio   float64 `json:"current_ratio"`     // 流动比率(倍)
	QuickRatio     float64 `json:"quick_ratio"`       // 速动比率(倍)
	DebtRatio      float64 `json:"debt_ratio"`        // 资产负债率(%)

	// 现金流
	OCFToRevenue   float64 `json:"ocf_to_revenue"`    // 经营净现金流/营业收入
}

func (PerformanceReport) TableName() string { return "performance_reports" }

// ---------- 股东户数 ----------

// ShareholderCount 股东户数表（东财 RPT_F10_EH_HOLDERNUM）
type ShareholderCount struct {
	StockCode           string  `gorm:"primaryKey;size:10;not null" json:"stock_code"`
	EndDate             string  `gorm:"primaryKey;size:10;not null" json:"end_date"`         // 统计截止日 YYYY-MM-DD
	SecurityName        string  `gorm:"size:50" json:"security_name"`

	HolderNum           int64   `json:"holder_num"`                       // 股东人数(户)
	HolderNumChangePct  float64 `json:"holder_num_change_pct"`            // 较上期变化(%)
	AvgFreeShares       int64   `json:"avg_free_shares"`                  // 人均流通股(股)
	AvgFreeSharesChgPct float64 `json:"avg_free_shares_change_pct"`       // 较上期变化(%)
	HoldFocus           string  `gorm:"size:20" json:"hold_focus"`          // 筹码集中度
	Price               float64 `json:"price"`                             // 股价(元)(报告期末)
	AvgHoldAmount       float64 `json:"avg_hold_amount"`                   // 人均持股市值(元)
	HoldRatioTotal      float64 `json:"hold_ratio_total"`                  // 十大股东持股合计(%)
	FreeHoldRatioTotal  float64 `json:"free_hold_ratio_total"`             // 十大流通股东持股合计(%)
}

func (ShareholderCount) TableName() string { return "shareholder_counts" }

// ---------- 股本变动 ----------

// ShareChange 股本变动表（对应东方财富"历年股份变动"）
type ShareChange struct {
	StockCode       string `gorm:"primaryKey;size:10;not null" json:"stock_code"`
	Date            string `gorm:"primaryKey;size:10;not null" json:"date"`               // 变动日期 YYYY-MM-DD
	ChangeReason    string `gorm:"size:200" json:"change_reason"`                         // 变动原因
	TotalShares     int64  `json:"total_shares"`                                          // 总股本(股)
	LimitedShares   int64  `json:"limited_shares"`                                        // 流通受限股份(股)
	UnlimitedShares int64  `json:"unlimited_shares"`                                      // 已流通股份(股)
	FloatAShares    int64  `json:"float_a_shares"`                                       // 已上市流通A股(股)
}

func (ShareChange) TableName() string { return "share_changes" }
