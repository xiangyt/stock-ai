package model

import (
	"time"

	"gorm.io/gorm"
)

// ========== 股票基本信息 (静态数据 - 很少变动) ==========

// StockExchange 交易所枚举
const (
	ExchangeSSE  = "SSE"  // 上海证券交易所 (上交所)
	ExchangeSZSE = "SZSE" // 深圳证券交易所 (深交所)
	ExchangeBSE  = "BSE"  // 北京证券交易所 (北交所)
)

// StockBoard 板块枚举
const (
	BoardMain    = "main"    // 主板
	BoardSME     = "sme"     // 中小板(已合并到主板)
	BoardChiNext = "chinext" // 创业板
	BoardStar    = "star"    // 科创板
	BoardBSE     = "bse"     // 北交所
)

// Stock 股票基本信息表
// 存储股票的固定属性，如代码、名称、交易所、上市信息等
// 这些信息很少变化，只在IPO或更名时更新
type Stock struct {
	ID          uint   `gorm:"primarykey" json:"id"`
	Code        string `gorm:"uniqueIndex;size:20;not null;comment:股票代码" json:"code"`
	Name        string `gorm:"size:50;not null;comment:股票简称" json:"name"` // 股票简称
	FullName    string `gorm:"size:100;comment:股票全称" json:"full_name"`    // 股票全称
	EnglishName string `gorm:"size:200;comment:英文名称" json:"english_name"` // 英文名称

	// 交易所与板块
	Exchange     string `gorm:"size:10;index;not null;comment:交易所" json:"exchange"` // SSE/SZSE/BSE
	ExchangeName string `gorm:"size:50;comment:交易所中文名" json:"exchange_name"`        // 上海/深圳/北京
	ListingBoard string `gorm:"size:20;index;comment:上市板块" json:"listing_board"`    // main/chinext/star/bse
	BoardName    string `gorm:"size:50;comment:板块名称" json:"board_name"`             // 主板/创业板/科创板/北交所

	// 上市信息 (固定不变)
	ListDate    string  `gorm:"size:10;index;comment:上市日期 YYYY-MM-DD" json:"list_date"` // 上市日期
	DelistDate  string  `gorm:"size:10;comment:退市日期" json:"delist_date"`                // 退市日期 (空=在市)
	IssuePrice  float64 `gorm:"comment:发行价" json:"issue_price"`                         // 发行价 (元)
	IssuePE     float64 `gorm:"comment:发行市盈率" json:"issue_pe"`                          // 发行市盈率 (倍)
	IssuePB     float64 `gorm:"comment:发行市净率" json:"issue_pb"`                          // 发行市净率 (倍)
	IssueShares int64   `gorm:"comment:发行股数(万股)" json:"issue_shares"`                   // 发行数量 (股)

	// 行业分类 (相对稳定)
	Industry     string `gorm:"size:100;index;comment:所属行业" json:"industry"` // 所属行业 (申万一级)
	IndustryCode string `gorm:"size:20;comment:行业代码" json:"industry_code"`   // 行业代码
	Sector       string `gorm:"size:100;comment:细分行业" json:"sector"`         // 细分行业

	// 元数据
	Created   time.Time      `json:"created"`
	Updated   time.Time      `json:"updated"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 表名
func (Stock) TableName() string { return "stocks" }

// BeforeCreate 创建前钩子：自动设置 Created/Updated 为当前时间
func (s *Stock) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if s.Created.IsZero() {
		s.Created = now
	}
	if s.Updated.IsZero() {
		s.Updated = now
	}
	return nil
}

// BeforeUpdate 更新前钩子：自动更新 Updated 字段
func (s *Stock) BeforeUpdate(tx *gorm.DB) error {
	s.Updated = time.Now()
	return nil
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

// ========== A股市场特殊字段说明 ==========
//
// Stock 表设计要点:
// 1. Code 使用纯数字代码 (000001, 600519)，不带交易所前缀
// 2. Exchange 字段区分交易所: SSE(沪)/SZSE(深)/BSE(京)
// 3. ListingBoard 区分板块: main(主板)/chinext(创业板)/star(科创板)/bse(北交所)
// 4. Issue* 字段存储IPO时的固定数据，一旦确定不再变更
// 5. 股本变动数据(总股本/流通A股/受限股份)存储在 share_changes 表中
