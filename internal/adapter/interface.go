package adapter

import (
	"context"
	"time"
)

// DataSource 数据源接口，所有数据源适配器必须实现此接口
// 对齐 stock 项目的 DataCollector 接口
type DataSource interface {
	// 基础信息
	Name() string                          // 数据源标识名 (eastmoney/tonghuashun)
	DisplayName() string                  // 显示名称
	Type() string                          // 类型: web_crawl/api/sdk/mock

	// 连接管理
	Init(config map[string]interface{}) error // 初始化配置
	TestConnection(ctx context.Context) error // 测试连接可用性
	Close() error                            // 关闭连接/清理资源

	// 股票列表
	GetStockList(ctx context.Context, cb ProgressCallback) ([]StockBasic, error) // 获取A股股票列表（带进度回调）
	GetStockDetail(ctx context.Context, code string) (*StockBasic, error)        // 获取股票详情（含发行价、发行PE等）

	// K线数据 - 多周期支持
	GetDailyKLine(ctx context.Context, code, startDate, endDate string, cb ProgressCallback) ([]StockPriceDaily, error)   // 日K线
	GetWeeklyKLine(ctx context.Context, code, startDate, endDate string, cb ProgressCallback) ([]StockPriceDaily, error)   // 周K线
	GetMonthlyKLine(ctx context.Context, code, startDate, endDate string, cb ProgressCallback) ([]StockPriceDaily, error)  // 月K线
	GetQuarterlyKLine(ctx context.Context, code, startDate, endDate string, cb ProgressCallback) ([]StockPriceDaily, error) // 季K线
	GetYearlyKLine(ctx context.Context, code, startDate, endDate string, cb ProgressCallback) ([]StockPriceDaily, error)    // 年K线

	// 实时数据
	GetRealtimeData(ctx context.Context, codes []string, cb ProgressCallback) (map[string]StockPriceDaily, error) // 批量实时行情
	GetTodayData(ctx context.Context, code string) (*StockPriceDaily, string, error)                                   // 当日数据(含名称)
	GetThisWeekData(ctx context.Context, code string) (*StockPriceDaily, error)                                        // 本周数据
	GetThisMonthData(ctx context.Context, code string) (*StockPriceDaily, error)                                       // 本月数据
	GetThisQuarterData(ctx context.Context, code string) (*StockPriceDaily, error)                                      // 本季数据
	GetThisYearData(ctx context.Context, code string) (*StockPriceDaily, error)                                         // 本年数据

	// 财务数据
	GetPerformanceReports(ctx context.Context, code string) ([]PerformanceReport, error)              // 业绩报表列表
	GetLatestPerformanceReport(ctx context.Context, code string) (*PerformanceReport, error)         // 最新业绩报表
	GetShareholderCounts(ctx context.Context, code string) ([]ShareholderCount, error)               // 股东户数列表
	GetLatestShareholderCount(ctx context.Context, code string) (*ShareholderCount, error)          // 最新股东户数

	// 股本变动
	GetShareChanges(ctx context.Context, code string) ([]ShareChange, error)                        // 历年股本变动

	// 配额与状态
	GetQuotaInfo() QuotaInfo // 配额信息
}

// StockBasic 股票基本信息（静态不变数据）
type StockBasic struct {
	Code        string  `json:"code"`         // 股票代码 000001
	Name        string  `json:"name"`         // 股票简称 平安银行
	FullName    string  `json:"full_name"`    // 股票全称
	Exchange    string  `json:"exchange"`     // 交易所 SSE/SZSE/BSE
	ListingBoard string `json:"listing_board"` // 板块 main/chinext/star/bse
	ListDate    string  `json:"list_date"`     // 上市日期 1991-04-03
	IssuePrice  float64 `json:"issue_price"`  // 发行价
	IssuePE     float64 `json:"issue_pe"`     // 发行市盈率
	IssuePB     float64 `json:"issue_pb"`     // 发行市净率
	IssueShares float64 `json:"issue_shares"` // 发行股数(万股)
	Industry    string  `json:"industry"`      // 行业 银行
	Sector      string  `json:"sector"`        // 细分行业 银行Ⅱ
}

// StockPriceDaily 价格日线（动态数据）
// 价格/金额类字段单位: 分 (cents)，避免浮点精度问题
type StockPriceDaily struct {
	Code      string `json:"code"`
	Date      string `json:"date"`       // 2026-04-12
	Open      int64  `json:"open"`       // 开盘价(分)
	High      int64  `json:"high"`       // 最高价(分)
	Low       int64  `json:"low"`        // 最低价(分)
	Close     int64  `json:"close"`      // 收盘价(分)
	Volume    int64  `json:"volume"`     // 成交量
	Amount    int64  `json:"amount"`     // 成交额(分)
	Change    int64  `json:"change"`     // 涨跌额(分)
	ChangePct float64 `json:"change_pct"` // 涨跌幅%
	Turnover  float64 `json:"turnover"`   // 换手率%
	Pe        float64 `json:"pe"`         // 市盈率
	Pb        float64 `json:"pb"`         // 市净率
	MarketCap float64 `json:"market_cap"` // 总市值(亿)
}

// PerformanceReport 业绩报表
type PerformanceReport struct {
	Code                      string  `json:"code"`
	ReportDate                int     `json:"report_date"`                 // 报告期 YYYYMMDD
	EPS                       float64 `json:"eps"`                        // 每股收益
	WeightEPS                 float64 `json:"weight_eps"`                 // 扣非每股收益
	Revenue                   float64 `json:"revenue"`                    // 营业总收入(元)
	RevenueQoQ                float64 `json:"revenue_qoq"`                // 营收同比增长(%)
	RevenueYoY                float64 `json:"revenue_yoy"`                // 营收环比增长(%)
	NetProfit                 float64 `json:"net_profit"`                 // 净利润(元)
	NetProfitQoQ              float64 `json:"net_profit_qoq"`            // 净利润同比增长(%)
	NetProfitYoY              float64 `json:"net_profit_yoy"`             // 净利润环比增长(%)
	BVPS                      float64 `json:"bvps"`                       // 每股净资产
	GrossMargin               float64 `json:"gross_margin"`               // 销售毛利率(%)
	DividendYield             float64 `json:"dividend_yield"`             // 股息率(%)
	LatestAnnouncementDate    *time.Time `json:"latest_announcement_date"` // 公告日期
	FirstAnnouncementDate     *time.Time `json:"first_announcement_date"`  // 首次公告日期
}

// ShareholderCount 股东户数
type ShareholderCount struct {
	Code           string  `json:"code"`
	SecurityCode   string  `json:"security_code"`    // 证券代码
	SecurityName   string  `json:"security_name"`    // 证券简称
	EndDate        int     `json:"end_date"`         // 统计截止日 YYYYMMDD
	HolderNum      int64   `json:"holder_num"`       // 股东户数
	PreHolderNum   int64   `json:"pre_holder_num"`   // 上期股东户数
	HolderNumChange int64  `json:"holder_num_change"` // 变动数量
	HolderNumRatio float64 `json:"holder_num_ratio"`  // 变动比例(%)
	AvgMarketCap   float64 `json:"avg_market_cap"`   // 户均市值(万元)
	AvgHoldNum     float64 `json:"avg_hold_num"`     // 户均持股数(股)
	TotalMarketCap float64 `json:"total_market_cap"` // 总市值(亿元)
	TotalAShares   int64   `json:"total_a_shares"`   // A股总股本(万股)
	HoldNoticeDate *time.Time `json:"hold_notice_date"` // 公告日期
}

// ShareChange 历年股本变动（对应东方财富"历年股份变动"）
type ShareChange struct {
	Code           string `json:"code"`             // 股票代码
	Date           string `json:"date"`             // 变动日期 YYYY-MM-DD
	TotalShares    int64  `json:"total_shares"`     // 总股本(万股)
	FloatAShares   int64  `json:"float_a_shares"`   // 已上市流通A股(万股)
	FloatShares    int64  `json:"float_shares"`     // 流通受限股份(万股)
	ChangeReason   string `json:"change_reason"`    // 变动原因 (高管股份变动/送转/增发等)
}

// QuotaInfo 配额信息
type QuotaInfo struct {
	DailyLimit    int     `json:"daily_limit"`    // 每日限制 (-1=无限制)
	DailyUsed     int     `json:"daily_used"`     // 已使用
	RateLimit     int     `json:"rate_limit"`     // 每秒请求限制
	LastRequestAt *time.Time `json:"last_request_at"` // 上次请求时间
}

// ProgressCallback 进度回调函数
// current: 当前进度, total: 总数(0=未知), message: 进度消息
type ProgressCallback func(current, total int, message string)
