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

	// 机构持仓
	GetInstitutionalHoldings(ctx context.Context, code string) ([]InstitutionalHolding, error) // 机构持仓历史

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

// PerformanceReport 业绩报表（东财 RPT_F10_FINANCE_MAINFINADATA）
type PerformanceReport struct {
	// ========== 基本信息 ==========
	Code                   string     `json:"code"`                    // 股票代码 002404
	SecurityName           string     `json:"security_name"`          // 证券简称 嘉欣丝绸
	ReportDate             string     `json:"report_date"`            // 报告期 2025-12-31（截取前10位）
	ReportType             string     `json:"report_type"`            // 报告类型 年报/季报等
	ReportDateName         string     `json:"report_date_name"`       // 报告期名称 2025年报
	Currency               string     `json:"currency"`               // 货币单位 CNY
	NoticeDate             string     `json:"notice_date"`            // 公告日期
	UpdateDate             string     `json:"update_date"`            // 更新日期
	IsBZ                   string     `json:"is_bz"`                  // 是否本期 IS_BZ

	// ========== 每股指标 ==========
	BasicEPS                float64 `json:"basic_eps"`                 // 基本每股收益(元)
	DeductedEPS             float64 `json:"deducted_eps"`              // 扣非每股收益(元)
	DilutedEPS              float64 `json:"diluted_eps"`               // 摊薄每股收益(元)
	BVPS                    float64 `json:"bvps"`                      // 每股净资产(元)
	EquityReservePerShare   float64 `json:"equity_reserve_per_share"`  // 每股公积金
	UndistributedProfitPS   float64 `json:"undistributed_profit_ps"`   // 每股未分配利润
	OCFPS                   float64 `json:"ocfps"`                     // 每股经营现金流(元)

	// ========== 成长能力指标 ==========
	TotalRevenue        float64 `json:"total_revenue"`          // 营业总收入(元)
	GrossProfit         float64 `json:"gross_profit"`           // 毛利润(元)
	ParentNetProfit     float64 `json:"parent_net_profit"`      // 归属净利润(元)
	DeductNetProfit     float64 `json:"deduct_net_profit"`      // 扣非净利润(元)
	RevenueYoY          float64 `json:"revenue_yoy"`            // 营业总收入同比增长(%)
	ParentNetProfitYoY  float64 `json:"parent_net_profit_yoy"`  // 归属净利润同比增长(%)
	DeductNetProfitYoY  float64 `json:"deduct_net_profit_yoy"`  // 扣非净利润同比增长(%)
	RevenueRollQoQ      float64 `json:"revenue_roll_qoq"`       // 营业总收入滚动环比增长(%)
	NetProfitRollQoQ    float64 `json:"net_profit_roll_qoq"`    // 归属净利润滚动环比增长(%)
	DeductNPTRollQoQ    float64 `json:"deduct_npt_roll_qoq"`    // 扣非净利润滚动环比增长(%)
	RevenueQoQ          float64 `json:"revenue_qoq"`            // 营业总收入环比增长(%)
	NetProfitQoQ        float64 `json:"net_profit_qoq"`         // 归属净利润环比增长(%)
	DeductNPTQoQ        float64 `json:"deduct_npt_qoq"`         // 扣非净利润环比增长(%)

	// ========== 盈利能力指标 ==========
	ROEW           float64 `json:"roe_w"`             // 净资产收益率-加权(%)
	ROEDW           float64 `json:"roe_dw"`            // 净资产收益率-扣非加权(%)
	ROA             float64 `json:"roa"`               // 总资产收益率(%)
	NetMargin       float64 `json:"net_margin"`        // 销售净利率(%)
	GrossMargin     float64 `json:"gross_margin"`       // 销售毛利率(%)
	NetProfitMargin float64 `json:"net_profit_margin"`  // 净利率(%)
	ROIC            float64 `json:"roic"`               // 投资资本回报率(%)
	TaxRate         float64 `json:"tax_rate"`           // 实际税率(%)

	// ========== 收益质量指标 ==========
	ARToRevenue       float64 `json:"ar_to_revenue"`        // 应收账款/营业收入
	SaleOCFToRevenue  float64 `json:"sale_ocf_to_revenue"`  // 销售净现金流/营业收入
	OCFToRevenue      float64 `json:"ocf_to_revenue"`       // 经营净现金流/营业收入

	// ========== 财务风险指标 ==========
	CurrentRatio       float64 `json:"current_ratio"`        // 流动比率
	QuickRatio         float64 `json:"quick_ratio"`          // 速动比率
	CashFlowRatio      float64 `json:"cash_flow_ratio"`      // 现金流比率
	DebtRatio          float64 `json:"debt_ratio"`           // 资产负债率(%)
	EquityMultiplier   float64 `json:"equity_multiplier"`    // 权益乘数
	DebtEquityRatio    float64 `json:"debt_equity_ratio"`    // 产权比率
	Liability          float64 `json:"liability"`            // 总负债(元)

	// ========== 营运能力指标 ==========
	TotalAssetTurnoverDays  float64 `json:"total_asset_turnover_days"`  // 总资产周转天数(天)
	InvTurnoverDays        float64 `json:"inv_turnover_days"`          // 存货周转天数(天)
	ARTurnoverDays         float64 `json:"ar_turnover_days"`           // 应收账款周转天数(天)
	PayableTurnoverDays    float64 `json:"payable_turnover_days"`      // 应付账款周转天数(天)
	PrepaidTurnoverDays    float64 `json:"prepaid_turnover_days"`      // 预付款项周转天数(天)
	FixedAssetTurnover     float64 `json:"fixed_asset_turnover"`       // 固定资产周转率
	CurrentAssetTurnover   float64 `json:"current_asset_turnover"`     // 流动资产周转率
	OperateCycle           float64 `json:"operate_cycle"`              // 营运周期(天)
	GuardSpeedRatio        float64 `json:"guard_speed_ratio"`          // 速动比率(修正)
	CashRatio              float64 `json:"cash_ratio"`                 // 现金比率
	InterestCoverageRatio  float64 `json:"interest_coverage_ratio"`    // 利息保障倍数
	LiquidationRatio       float64 `json:"liquidation_ratio"`          // 清算价值比率
	InterestDebtRatio      float64 `json:"interest_debt_ratio"`        // 有息负债率(%)
	FCLiabilities          float64 `json:"fc_liabilities"`             // 金融负债占比(%)

	// ========== 偿债能力指标 ==========
	CurrentAssetRatio   float64 `json:"current_asset_ratio"`   // 流动资产/总资产(%)
	NonCurrentAssetRatio float64 `json:"non_current_asset_ratio"` // 非流动资产/总资产(%)

	// ========== 现金流指标 ==========
	FCFFForward float64 `json:"fcff_forward"` // 企业自由现金流(预测)
	FCFFBack    float64 `json:"fcff_back"`    // 企业自由现金流(回溯)

	// ========== 其他 ==========
	StaffNum            int     `json:"staff_num"`             // 员工人数
	AvgTOI              float64 `json:"avg_toi"`              // 人均创收
	AvgNetProfit        float64 `json:"avg_net_profit"`       // 人均创利
	PrepaidAccountsRatio float64 `json:"prepaid_accounts_ratio"` // 预付款项/营业成本(%)
	AccountsPayableTR   float64 `json:"accounts_payable_tr"`  // 应付账款周转率
	FixedAssetTR        float64 `json:"fixed_asset_tr"`       // 固定资产周转率
}

// ShareholderCount 股东户数（东财 RPT_F10_EH_HOLDERNUM）
type ShareholderCount struct {
	Code                 string     `json:"code"`                    // 股票代码
	SecurityCode         string     `json:"security_code"`          // 证券代码
	SecurityName         string     `json:"security_name"`          // 证券简称
	EndDate              string     `json:"end_date"`               // 统计截止日 YYYY-MM-DD

	// ========== 核心指标 ==========
	HolderNum            int64      `json:"holder_num"`             // 股东人数(户)
	HolderNumChangePct   float64    `json:"holder_num_change_pct"`  // 较上期变化(%)
	AvgFreeShares        int64      `json:"avg_free_shares"`        // 人均流通股(股)
	AvgFreeSharesChangePct float64  `json:"avg_free_shares_change_pct"` // 较上期变化(%)

	// ========== 筹码集中度 ==========
	HoldFocus            string     `json:"hold_focus"`             // 筹码集中度（较分散/较集中等）
	Price                float64    `json:"price"`                  // 股价(元)（对应报告期末）
	AvgHoldAmount        float64    `json:"avg_hold_amount"`        // 人均持股市值(元)
	HoldRatioTotal       float64    `json:"hold_ratio_total"`       // 十大股东持股合计(%)
	FreeHoldRatioTotal   float64    `json:"free_hold_ratio_total"`  // 十大流通股东持股合计(%)
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

// QuotaInfo 配额信息（同时作为限速器配置和运行时状态）
type QuotaInfo struct {
	DailyLimit    int        `json:"daily_limit"`     // 每日限制 (-1=无限制)
	DailyUsed     int        `json:"daily_used"`      // 已使用（运行时累计）
	RateLimit     int        `json:"rate_limit"`      // 每秒请求限制（=令牌桶速率）
	Burst         int        `json:"burst"`           // 突发容量（=令牌桶大小，默认=RateLimit）
	LastRequestAt *time.Time `json:"last_request_at"` // 上次请求时间（运行时）
}

// LimiterConfig 从 QuotaInfo 提取限速器参数
func (q *QuotaInfo) LimiterConfig() (float64, int) {
	if q.RateLimit <= 0 {
		return 10, 10 // 默认 10rps
	}
	r := float64(q.RateLimit)
	b := q.Burst
	if b <= 0 {
		b = q.RateLimit
	}
	return r, b
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

// InstitutionalHolding 机构持仓（东财 RPT_F10_MAIN_ORGHOLDDETAILS，ORG_TYPE=00 合计）
type InstitutionalHolding struct {
	Code                string  `json:"code"`                  // 股票代码
	ReportDate          string  `json:"report_date"`           // 报告期 YYYY-MM-DD

	// ========== 核心指标 ==========
	InstitutionCount    int     `json:"institution_count"`     // 机构总数(家)
	TotalFreeShares     int64   `json:"total_free_shares"`     // 合计持股(股) - 流通股本
	TotalMarketCap      float64 `json:"total_market_cap"`       // 合计市值(元)
	FreeShareRatio      float64 `json:"free_share_ratio"`      // 占流通股比(%)
	TotalShareRatio     float64 `json:"total_share_ratio"`     // 占总股本比例(%)
	ClosePrice          float64 `json:"close_price"`           // 报告期末收盘价(元)

	// ========== 变动指标 ==========
	FreeShareChangePct  float64 `json:"free_share_change_pct"` // 较上期变化(%)
	HoldingChangeRatio  float64 `json:"holding_change_ratio"`  // 持股变动幅度(%)
	FreeShareChangeNum  int64   `json:"free_share_change_num"` // 持仓变动数量(股)
}
