package model

import (
	"time"

	"gorm.io/gorm"
)

// ============================================================================
//  策略订阅模块 — 数据模型
// ============================================================================

// MonitorScope 监控范围
type MonitorScope string

const (
	ScopeAll    MonitorScope = "all"    // 全部A股
	ScopeHeld   MonitorScope = "held"   // 用户持仓
	ScopeCustom MonitorScope = "custom" // 自选股票
)

// PresetType 预设频率类型
type PresetType string

const (
	PresetEvery15min PresetType = "every_15min"
	PresetEvery30min PresetType = "every_30min"
	PresetEveryHour  PresetType = "every_hour"
	PresetDailyOpen  PresetType = "daily_open"
	PresetDailyClose PresetType = "daily_close"
	PresetDailyTwice PresetType = "daily_twice"
	PresetNoon       PresetType = "noon"        // 中午12点
	PresetCloseAlert PresetType = "close_alert" // 14:45 收盘预警
	PresetCustom     PresetType = "custom"      // 自定义 cron
)

// LogStatus 执行日志状态
type LogStatus string

const (
	LogStatusSuccess LogStatus = "success"
	LogStatusPartial LogStatus = "partial"
	LogStatusFailed  LogStatus = "failed"
)

// Subscription 订阅配置表
type Subscription struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	UID              uint           `gorm:"index;not null" json:"uid"`
	Name             string         `gorm:"size:100;not null" json:"name"`
	StrategyID       uint           `gorm:"index;not null" json:"strategy_id"`
	Scope            MonitorScope   `gorm:"size:20;not null;default:all" json:"scope"`
	CustomStocks     string         `gorm:"type:text" json:"custom_stocks"`               // JSON: []string（6位代码）
	PresetType       PresetType     `gorm:"size:20;not null;default:every_30min" json:"preset_type"`
	CronExpr         string         `gorm:"size:100" json:"cron_expr"`                    // 自定义cron表达式
	TradingHoursOnly bool           `gorm:"default:true" json:"trading_hours_only"`
	IsActive         bool           `gorm:"default:true;index" json:"is_active"`
	Template         string         `gorm:"type:text" json:"template"`                     // 用户自定义模板，NULL 用默认
	LastRunAt        *time.Time     `json:"last_run_at"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Subscription) TableName() string { return "strategy_subscriptions" }

// SubscriptionBot 订阅-机器人关联表（M2M）
type SubscriptionBot struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	SubscriptionID uint     `gorm:"uniqueIndex:idx_sub_bot;not null" json:"subscription_id"`
	BotID         uint      `gorm:"uniqueIndex:idx_sub_bot;not null" json:"bot_id"`
	CreatedAt     time.Time `json:"created_at"`
}

func (SubscriptionBot) TableName() string { return "subscription_bots" }

// SubscriptionLog 订阅执行日志
type SubscriptionLog struct {
	ID             uint         `gorm:"primarykey" json:"id"`
	SubscriptionID uint         `gorm:"index;not null" json:"subscription_id"`
	RunTime        time.Time    `gorm:"not null" json:"run_time"`
	FinishedAt     *time.Time   `json:"finished_at"`
	Scope          MonitorScope `gorm:"size:20" json:"scope"`
	TotalScanned   int          `gorm:"default:0" json:"total_scanned"`
	MatchCount     int          `gorm:"default:0" json:"match_count"`
	MatchStocks    string       `gorm:"type:text" json:"match_stocks"`   // JSON: []MatchStock
	DurationMs     int          `gorm:"default:0" json:"duration_ms"`
	Status         LogStatus    `gorm:"size:20;not null" json:"status"`
	ErrorMsg       string       `gorm:"type:text" json:"error_msg"`
	PushStatus     string       `gorm:"type:text" json:"push_status"`    // JSON: map[uint]string
	CreatedAt      time.Time    `json:"created_at"`
}

func (SubscriptionLog) TableName() string { return "subscription_logs" }

// MatchStock 匹配股票（JSON 序列化用）
type MatchStock struct {
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}
