package adapter

import (
	"context"
	"time"
)

// DataSource 数据源接口，所有数据源适配器必须实现此接口
// 对齐 stock 项目的 DataCollector 接口
type DataSource interface {
	// 基础信息
	Name() string        // Name 数据源标识名 (eastmoney/tonghuashun)
	DisplayName() string // DisplayName 显示名称
	Type() string        // Type 类型: web_crawl/api/sdk/mock

	// 连接管理
	Init(config map[string]interface{}) error // Init 初始化配置
	TestConnection(ctx context.Context) error // 测试连接可用性
	Close() error                             // 关闭连接/清理资源

	// 股票列表
	GetStockList(ctx context.Context) ([]StockBasic, error) // 获取A股股票列表
	GetStockDetail(ctx context.Context, code string) (*StockBasic, error) // 获取股票详情（含发行价、发行PE等）

	// K线数据 - 多周期支持
	// adjType: 复权类型, 使用 AdjQFQ(前复权)/AdjNone(不复权)/AdjBQQ(后复权)
	GetDailyKLine(ctx context.Context, code, adjType string) ([]StockPriceDaily, error)     // 日K线
	GetWeeklyKLine(ctx context.Context, code, adjType string) ([]StockPriceDaily, error)    // 周K线
	GetMonthlyKLine(ctx context.Context, code, adjType string) ([]StockPriceDaily, error)   // 月K线
	GetQuarterlyKLine(ctx context.Context, code, adjType string) ([]StockPriceDaily, error) // 季K线
	GetYearlyKLine(ctx context.Context, code, adjType string) ([]StockPriceDaily, error)    // 年K线

	// 实时数据
	GetTodayData(ctx context.Context, code string) (*StockPriceDaily, string, error)                              // 当日数据(含名称)
	GetThisWeekData(ctx context.Context, code string) (*StockPriceDaily, error)                                   // 本周数据
	GetThisMonthData(ctx context.Context, code string) (*StockPriceDaily, error)                                  // 本月数据
	GetThisQuarterData(ctx context.Context, code string) (*StockPriceDaily, error)                                // 本季数据
	GetThisYearData(ctx context.Context, code string) (*StockPriceDaily, error)                                   // 本年数据

	// 财务数据
	GetPerformanceReports(ctx context.Context, code string) ([]PerformanceReport, error)     // 业绩报表列表
	GetLatestPerformanceReport(ctx context.Context, code string) (*PerformanceReport, error) // 最新业绩报表
	GetShareholderCounts(ctx context.Context, code string) ([]ShareholderCount, error)       // 股东户数列表
	GetLatestShareholderCount(ctx context.Context, code string) (*ShareholderCount, error)   // 最新股东户数

	// 股本变动
	GetShareChanges(ctx context.Context, code string) ([]ShareChange, error) // 历年股本变动

	// 配额与状态
	GetQuotaInfo() QuotaInfo // 配额信息
}

// StockBasic 股票基本信息（静态不变数据，来自F10基本资料接口 RPT_F10_BASIC_ORGINFO）
type StockBasic struct {
	Code          string  `json:"code"`           // 股票代码 002475
	Name          string  `json:"name"`           // 股票简称 立讯精密
	FullName      string  `json:"full_name"`      // 公司全称 立讯精密工业股份有限公司
	FullNameEn    string  `json:"full_name_en"`   // 英文名称 Luxshare Precision Industry Co., Ltd.
	FormerName    string  `json:"former_name"`    // 曾用名
	Exchange      string  `json:"exchange"`       // 交易所 SSE/SZSE/BSE
	ListingBoard  string  `json:"listing_board"`  // 板块 main/chinext/star/bse
	ListDate      string  `json:"list_date"`      // 上市日期 2010-09-15
	FoundDate     string  `json:"found_date"`     // 成立日期 2004-05-24
	SecurityType  string  `json:"security_type"`  // 证券类型 深交所主板A股
	Industry      string  `json:"industry"`       // 行业 制造业-计算机、通信和其他电子设备制造业
	Sector        string  `json:"sector"`         // 细分行业(东财分类) 电子设备-消费电子设备-消费电子设备
	Province      string  `json:"province"`       // 所在省份 广东
	Address       string  `json:"address"`        // 公司地址
	RegAddress    string  `json:"reg_address"`    // 注册地址
	RegCapital    float64 `json:"reg_capital"`    // 注册资本(万元) 728598.4811
	EmpNum        int     `json:"emp_num"`        // 员工人数 278103
	President     string  `json:"president"`      // 董事长 王来春
	LegalPerson   string  `json:"legal_person"`   // 法人代表 王来春
	Secretary     string  `json:"secretary"`      // 董秘 肖云兮
	OrgTel        string  `json:"org_tel"`        // 联系电话
	OrgEmail      string  `json:"org_email"`      // 联系邮箱
	OrgWeb        string  `json:"org_web"`        // 公司网址
	OrgProfile    string  `json:"org_profile"`    // 公司简介
	BusinessScope string  `json:"business_scope"` // 经营范围
	MainBusiness  string  `json:"main_business"`  // 主营业务
	ActualHolder  string  `json:"actual_holder"`  // 实际控制人 王来春,王来胜
	Currency      string  `json:"currency"`       // 货币单位 人民币

	// ========== IPO 发行信息 (来自 RPT_PCF10_ORG_ISSUEINFO) ==========
	IssuePrice      float64 `json:"issue_price"`       // 发行价(元) 28.8
	IssuePE         float64 `json:"issue_pe"`          // 发行市盈率(倍) 72
	ParValue        float64 `json:"par_value"`         // 每股面值(元) 1
	TotalIssueNum   int64   `json:"total_issue_num"`   // 发行数量(股) 43800000
	OnlineIssueDate string  `json:"online_issue_date"` // 网上申购日期 2010-09-01
	IssueWay        string  `json:"issue_way"`         // 发行方式 网下询价配售
	Sponsor         string  `json:"sponsor"`           // 保荐机构 中信证券股份有限公司
	Underwriter     string  `json:"underwriter"`       // 主承销商 中信证券股份有限公司
}

// StockPriceDaily 价格日线（动态数据）
// 价格/金额类字段单位: 分 (cents)，避免浮点精度问题
type StockPriceDaily struct {
	Code      string  `json:"code"`
	Date      string  `json:"date"`       // 2026-04-12
	Open      int64   `json:"open"`       // 开盘价(分)
	High      int64   `json:"high"`       // 最高价(分)
	Low       int64   `json:"low"`        // 最低价(分)
	Close     int64   `json:"close"`      // 收盘价(分)
	Volume    int64   `json:"volume"`     // 成交量
	Amount    int64   `json:"amount"`     // 成交额(分)
	Change    int64   `json:"change"`     // 涨跌额(分)
	ChangePct float64 `json:"change_pct"` // 涨跌幅%
	Turnover  float64 `json:"turnover"`   // 换手率%
	Pe        float64 `json:"pe"`         // 市盈率
	Pb        float64 `json:"pb"`         // 市净率
	MarketCap float64 `json:"market_cap"` // 总市值(亿)
}

// PerformanceReport 业绩报表
type PerformanceReport struct {
	Code                   string     `json:"code"`
	ReportDate             int        `json:"report_date"`              // 报告期 YYYYMMDD
	EPS                    float64    `json:"eps"`                      // 每股收益
	WeightEPS              float64    `json:"weight_eps"`               // 扣非每股收益
	Revenue                float64    `json:"revenue"`                  // 营业总收入(元)
	RevenueQoQ             float64    `json:"revenue_qoq"`              // 营收同比增长(%)
	RevenueYoY             float64    `json:"revenue_yoy"`              // 营收环比增长(%)
	NetProfit              float64    `json:"net_profit"`               // 净利润(元)
	NetProfitQoQ           float64    `json:"net_profit_qoq"`           // 净利润同比增长(%)
	NetProfitYoY           float64    `json:"net_profit_yoy"`           // 净利润环比增长(%)
	BVPS                   float64    `json:"bvps"`                     // 每股净资产
	GrossMargin            float64    `json:"gross_margin"`             // 销售毛利率(%)
	DividendYield          float64    `json:"dividend_yield"`           // 股息率(%)
	LatestAnnouncementDate *time.Time `json:"latest_announcement_date"` // 公告日期
	FirstAnnouncementDate  *time.Time `json:"first_announcement_date"`  // 首次公告日期
}

// ShareholderCount 股东户数
type ShareholderCount struct {
	Code            string     `json:"code"`
	SecurityCode    string     `json:"security_code"`     // 证券代码
	SecurityName    string     `json:"security_name"`     // 证券简称
	EndDate         int        `json:"end_date"`          // 统计截止日 YYYYMMDD
	HolderNum       int64      `json:"holder_num"`        // 股东户数
	PreHolderNum    int64      `json:"pre_holder_num"`    // 上期股东户数
	HolderNumChange int64      `json:"holder_num_change"` // 变动数量
	HolderNumRatio  float64    `json:"holder_num_ratio"`  // 变动比例(%)
	AvgMarketCap    float64    `json:"avg_market_cap"`    // 户均市值(万元)
	AvgHoldNum      float64    `json:"avg_hold_num"`      // 户均持股数(股)
	TotalMarketCap  float64    `json:"total_market_cap"`  // 总市值(亿元)
	TotalAShares    int64      `json:"total_a_shares"`    // A股总股本(万股)
	HoldNoticeDate  *time.Time `json:"hold_notice_date"`  // 公告日期
}

// ShareChange 历年股本变动（对应东方财富"历年股份变动"）
type ShareChange struct {
	Code            string `json:"code"`             // 股票代码
	Date            string `json:"date"`             // 变动日期 YYYY-MM-DD
	TotalShares     int64  `json:"total_shares"`     // 总股本(股)
	LimitedShares   int64  `json:"limited_shares"`   // 流通受限股份(股)
	UnlimitedShares int64  `json:"unlimited_shares"` // 已流通股份(股)
	FloatAShares    int64  `json:"float_a_shares"`   // 已上市流通A股(股)
	ChangeReason    string `json:"change_reason"`    // 变动原因
}

// QuotaInfo 配额信息
type QuotaInfo struct {
	DailyLimit    int        `json:"daily_limit"`     // 每日限制 (-1=无限制)
	DailyUsed     int        `json:"daily_used"`      // 已使用
	RateLimit     int        `json:"rate_limit"`      // 每秒请求限制
	LastRequestAt *time.Time `json:"last_request_at"` // 上次请求时间
}

// ProgressCallback 进度回调函数
// current: 当前进度, total: 总数(0=未知), message: 进度消息
type ProgressCallback func(current, total int, message string)

// AdjType 复权类型
const (
	AdjQFQ = "1" // 前复权 (forward adjustment)
	AdjNone = "0" // 不复权 (no adjustment)
	AdjBQQ = "2" // 后复权 (backward adjustment)
)
