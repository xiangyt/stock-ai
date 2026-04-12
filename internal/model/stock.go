package model

import (
	"time"

	"gorm.io/gorm"
)

// ========== 股票基本信息 (静态数据 - 很少变动) ==========

// StockExchange 交易所枚举
const (
	ExchangeSSE = "SSE"  // 上海证券交易所 (上交所)
	ExchangeSZSE = "SZSE" // 深圳证券交易所 (深交所)
	ExchangeBSE  = "BSE"  // 北京证券交易所 (北交所)
)

// StockBoard 板块枚举
const (
	BoardMain     = "main"      // 主板
	BoardSME      = "sme"       // 中小板(已合并到主板)
	BoardChiNext  = "chinext"   // 创业板
	BoardStar     = "star"      // 科创板
	BoardBSE      = "bse"       // 北交所
)

// Stock 股票基本信息表
// 存储股票的固定属性，如代码、名称、交易所、上市信息等
// 这些信息很少变化，只在IPO或更名时更新
type Stock struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	Code            string         `gorm:"uniqueIndex;size:20;not null;comment:股票代码" json:"code"`
	Name            string         `gorm:"size:50;not null;comment:股票简称" json:"name"`                // 股票简称
	FullName        string         `gorm:"size:100;comment:股票全称" json:"full_name"`                   // 股票全称
	EnglishName     string         `gorm:"size:200;comment:英文名称" json:"english_name"`                 // 英文名称
	
	// 交易所与板块
	Exchange        string         `gorm:"size:10;index;not null;comment:交易所" json:"exchange"`          // SSE/SZSE/BSE
	ExchangeName    string         `gorm:"size:50;comment:交易所中文名" json:"exchange_name"`             // 上海/深圳/北京
	ListingBoard    string         `gorm:"size:20;index;comment:上市板块" json:"listing_board"`           // main/chinext/star/bse
	BoardName       string         `gorm:"size:50;comment:板块名称" json:"board_name"`                    // 主板/创业板/科创板/北交所
	
	// 上市信息 (固定不变)
	ListDate        string         `gorm:"size:10;index;comment:上市日期 YYYY-MM-DD" json:"list_date"`   // 上市日期
	DelistDate      string         `gorm:"size:10;comment:退市日期" json:"delist_date"`                 // 退市日期 (空=在市)
	IssuePrice      float64        `comment:发行价" json:"issue_price"`                                 // 发行价 (元)
	IssuePE        float64        `comment:发行市盈率" json:"issue_pe"`                                // 发行市盈率 (倍)
	IssuePB        float64        `comment:发行市净率" json:"issue_pb"`                                 // 发行市净率 (倍)
	IssueShares     int64          `comment:发行股数(万股)" json:"issue_shares"`                         // 发行数量 (万股)
	
	// 行业分类 (相对稳定)
	Industry        string         `gorm:"size:100;index;comment:所属行业" json:"industry"`              // 所属行业 (申万一级)
	IndustryCode    string         `gorm:"size:20;comment:行业代码" json:"industry_code"`                // 行业代码
	Sector          string         `gorm:"size:100;comment:细分行业" json:"sector"`                     // 细分行业
	Concept         string         `gorm:"type:text;comment:概念标签,逗号分隔" json:"concept"`           // 概念标签
	
	// 公司基本信息
	CompanyCode     string         `gorm:"size:30;index;comment:公司代码" json:"company_code"`           // 公司统一社会信用代码
	RegisteredCapital float64      `comment:注册资本(万元)" json:"registered_capital"`                  // 注册资本 (万元)
	EstablishDate   string         `gorm:"size:10;comment:成立日期" json:"establish_date"`             // 成立日期
	Website         string         `gorm:"size:255;comment:公司官网" json:"website"`                    // 官网地址
	Address         string         `gorm:"size:500;comment:注册地址" json:"address"`                    // 注册地址
	Chairman        string         `gorm="size:50;comment:董事长" json:"chairman"`                     // 董事长姓名
	
	// 状态标记
	Status          string         `gorm:"size:20;default:normal;index;comment:状态" json:"status"`    // normal/delisted/suspended
	UpdateTime      string         `gorm:"size:19;comment:最后更新时间" json:"update_time"`            // 最后更新时间
	
	// 元数据
	DataSources     string         `gorm:"type:text;comment:数据来源,JSON数组" json:"data_sources"`     // 来源记录
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 表名
func (Stock) TableName() string { return "stocks" }

// IsListed 是否在市交易
func (s *Stock) IsListed() bool {
	return s.Status == "normal"
}

// GetExchangeDisplay 获取交易所显示名
func (s *Stock) GetExchangeDisplay() string {
	switch s.Exchange {
	case ExchangeSSE:
		return "上海证券交易所"
	case ExchangeSZSE:
		return "深圳证券交易所"
	case ExchangeBSE:
		return "北京证券交易所"
	default:
		return s.ExchangeName
	}
}

// GetBoardDisplay 获取板块显示名
func (s *Stock) GetBoardDisplay() string {
	switch s.ListingBoard {
	case BoardMain:
		return "主板"
	case BoardChiNext:
		return "创业板"
	case BoardStar:
		return "科创板"
	case BoardBSE:
		return "北交所"
	default:
		return s.BoardName
	}
}

// GetCodeWithExchange 获取带交易所前缀的代码
func (s *Stock) GetCodeWithExchange() string {
	prefix := ""
	switch s.Exchange {
	case ExchangeSSE:
		prefix = "sh"
	case ExchangeSZSE:
		prefix = "sz"
	case ExchangeBSE:
		prefix = "bj"
	}
	return prefix + s.Code
}


// ========== 股票价格数据 (动态数据 - 每日变动) ==========

// StockPrice 股票日线行情表
// 存储每日的开高低收、成交量等技术面数据
type StockPrice struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	StockCode     string    `gorm:"uniqueIndex:idx_code_date;size:20;index;not null" json:"stock_code"`
	Date          string    `gorm:"uniqueIndex:idx_code_date;size:10;not null" json:"date"` // YYYY-MM-DD
	
	// OHLCV 数据
	Open          float64   `json:"open"`    // 开盘价
	Close         float64   `json:"close"`   // 收盘价
	High          float64   `json:"high"`    // 最高价
	Low           float64   `json:"low"`     // 最低价
	Volume        int64     `json:"volume"`   // 成交量 (手)
	Amount        float64   `json:"amount"`   // 成交额 (元)
	TurnoverRate  float64   `json:"turnover_rate"` // 换手率 (%)
	
	// 涨跌信息
	PreClose      float64   `json:"pre_close"`      // 昨收价
	Change        float64   `json:"change"`          // 涨跌额
	ChangePct     float64   `json:"change_pct"`      // 涨跌幅 (%)
	Amplitude     float64   `json:"amplitude"`       // 振幅 (%)
	
	// 市值数据 (当日)
	TotalMarketCap  float64 `json:"total_market_cap"` // 总市值 (亿元)
	CirculateMarketCap float64 `json:"circulate_market_cap"` // 流通市值 (亿元)
	
	// 技术指标 (计算值)
	MA5           float64   `json:"ma5"`       // 5日均线
	MA10          float64   `json:"ma10"`      // 10日均线
	MA20          float64   `json:"ma20"`      // 20日均线
	MA60          float64   `json:"ma60"`      // 60日均线
	MACD          float64   `json:"macd"`      // MACD值
	MACDSignal    float64   `json:"macd_signal"` // MACD信号线
	MACDHist      float64   `json:"macd_hist"`   // MACD柱状图
	KDJ_K         float64   `json:"kdj_k"`     // KDJ-K值
	KDJ_D         float64   `json:"kdj_d"`     // KDJ-D值
	KDJ_J         float64   `json:"kdj_j"`     // KDJ-J值
	RSI6          float64   `json:"rsi6"`      // RSI-6
	RSI12         float64   `json:"rsi12"`     // RSI-12
	BOLLUpper     float64   `json:"boll_upper"` // 布林带上轨
	BOLLMid       float64   `json:"boll_mid"`   // 布林带中轨
	BOLLLower     float64   `json:"boll_lower"` // 布林带下轨
	
	// 数据来源
	SourceName    string    `gorm:"size:50" json:"source_name"` // 数据源名称
	CreatedAt     time.Time `json:"created_at"`
}

func (StockPrice) TableName() string { return "stock_prices" }

// IsUp 当日是否上涨
func (p *StockPrice) IsUp() bool { return p.Change > 0 }
func (p *StockPrice) IsDown() bool { return p.Change < 0 }
func (p *StockPrice) IsLimitUp() bool { return p.ChangePct >= 9.85 } // 涨停
func (p *StockPrice) IsLimitDown() bool { return p.ChangePct <= -9.85 } // 跌停


// ========== 热门题材 (运营数据) ==========

// HotTopic 热门题材/概念表
type HotTopic struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:100;not null" json:"name"`          // 题材名称
	Tag         string    `gorm:"size:50" json:"tag"`                                 // 标签:热门/政策/事件/技术
	Description string    `gorm:"size:500" json:"description"`                        // 描述
	Keywords    string    `gorm:"type:text" json:"keywords"`                          // 关键词,逗号分隔
	RelatedCodes string   `gorm:"type:text" json:"related_codes"`                     // 关联股票代码,逗号分隔
	Heat        int       `json:"heat"`                                              // 热度指数 (0-100)
	Trend       string    `gorm:"size:20" json:"trend"`                               // 趋势: up/down/stable
	Rank        int       `json:"rank"`                                               // 排名
	ActiveFrom  string    `gorm:"size:10" json:"active_from"`                         // 活跃开始时间
	ActiveTo    string    `gorm:"size:10" json:"active_to"`                           // 活跃结束时间
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (HotTopic) TableName() string { return "hot_topics" }


// ========== 用户筛选条件 (业务数据) ==========

// FilterCondition 用户保存的筛选条件
type FilterCondition struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `json:"user_id"`                                            // 用户ID (预留)
	Name        string    `gorm:"size:100" json:"name"`                               // 条件名称
	Description string    `gorm:"size:500" json:"description"`                         // 描述
	Conditions  string    `gorm:"type:text;not null" json:"conditions"`               // JSON格式的条件数组
	IsPublic    bool      `gorm:"default:false" json:"is_public"`                    // 是否公开
	UseCount    int       `gorm:"default:0" json:"use_count"`                        // 使用次数
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (FilterCondition) TableName() string { return "filter_conditions" }


// ========== A股市场特殊字段说明 ==========
//
// Stock 表设计要点:
// 1. Code 使用纯数字代码 (000001, 600519)，不带交易所前缀
// 2. Exchange 字段区分交易所: SSE(沪)/SZSE(深)/BSE(京)
// 3. ListingBoard 区分板块: main(主板)/chinext(创业板)/star(科创板)/bse(北交所)
// 4. Issue* 字段存储IPO时的固定数据，一旦确定不再变更
// 5. 股本变动数据(总股本/流通A股/受限股份)存储在 share_changes 表中
//
// StockPrice 表设计要点:
// 1. StockCode + Date 作为联合唯一索引
// 2. Date 格式为 YYYY-MM-DD
// 3. MA/MACD/KDJ/RSI/BOLL 为预计算的技术指标值
// 4. SourceName 记录数据来自哪个数据源
