package model

import (
	"encoding/json"
	"time"
)

// MonitorScope / ScopeHeld / ScopeCustom 已在 subscription.go 中定义，此处复用

// ============================================================================
//  MonitorConfig 盯盘监控配置
// ============================================================================

// MonitorConfig 盯盘监控配置
type MonitorConfig struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	UID       uint           `gorm:"index;not null" json:"uid"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	Scope     MonitorScope   `gorm:"size:20;not null;default:held" json:"scope"`
	Stocks    string         `gorm:"type:text" json:"stocks"` // JSON: ["000001","600036"]
	Rules     string         `gorm:"type:text;not null" json:"rules"`
	Cooldown  string         `gorm:"type:text;not null" json:"cooldown"`
	Template  string         `gorm:"type:text" json:"template"`
	IsActive  bool           `gorm:"default:true;index" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt *time.Time     `gorm:"index" json:"-"`
}

func (MonitorConfig) TableName() string { return "monitor_configs" }

// ============================================================================
//  MonitorConfigBot 监控配置-机器人关联
// ============================================================================

// MonitorConfigBot 监控配置关联机器人
type MonitorConfigBot struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	ConfigID  uint      `gorm:"uniqueIndex:idx_config_bot" json:"config_id"`
	BotID     uint      `gorm:"uniqueIndex:idx_config_bot" json:"bot_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (MonitorConfigBot) TableName() string { return "monitor_config_bots" }

// ============================================================================
//  告警规则类型（动态，每种类型有独立参数）
// ============================================================================

// RuleType 告警规则类型
type RuleType string

const (
	RuleTypeDailyChange  RuleType = "daily_change"  // 当日涨幅（6档）
	RuleTypeRapidMove    RuleType = "rapid_move"     // 急拉急跌
	RuleTypeVolumeRatio  RuleType = "volume_ratio"   // 量比异动
	RuleTypeSealBoard    RuleType = "seal_board"     // 封单数量

	// 兼容旧版
	_legacyShortTermVolatility = "short_term_volatility"

	// 兼容旧版（已废弃，保留枚举值便于解析错误提示）
	_legacyLimitUp      = "limit_up"
	_legacyLimitDown    = "limit_down"
	_legacySurgeUp      = "surge_up"
	_legacySurgeDown    = "surge_down"
	_legacyHighTurnover = "high_turnover"
)

// MonitorRule 单条告警规则（type + params）
type MonitorRule struct {
	Type   RuleType        `json:"type"`
	Params json.RawMessage `json:"params"` // 类型特定的参数 JSON
}

// ============================================================================
//  各规则类型的参数结构体
// ============================================================================

// DailyChangeParams 当日涨幅参数
type DailyChangeParams struct {
	SurgeBigEnabled   bool    `json:"surge_big_enabled"`   // 大涨告警开关
	SurgeBig          float64 `json:"surge_big"`           // 大涨阈值(%)
	SurgeSmallEnabled bool    `json:"surge_small_enabled"` // 小涨告警开关
	SurgeSmall        float64 `json:"surge_small"`         // 小涨阈值(%)
	LimitUpEnabled    bool    `json:"limit_up_enabled"`    // 涨停告警开关
	LimitUp           float64 `json:"limit_up"`            // 涨停阈值(%)
	LimitDownEnabled  bool    `json:"limit_down_enabled"`  // 跌停告警开关
	LimitDown         float64 `json:"limit_down"`          // 跌停阈值(%)
	DropSmallEnabled  bool    `json:"drop_small_enabled"`  // 小跌告警开关
	DropSmall         float64 `json:"drop_small"`          // 小跌阈值(%)
	DropBigEnabled    bool    `json:"drop_big_enabled"`    // 大跌告警开关
	DropBig           float64 `json:"drop_big"`            // 大跌阈值(%)
}

// DefaultDailyChangeParams 默认当日涨幅参数（全部启用）
func DefaultDailyChangeParams() DailyChangeParams {
	return DailyChangeParams{
		SurgeBigEnabled:   true,
		SurgeBig:          9.0,
		SurgeSmallEnabled: true,
		SurgeSmall:        5.0,
		LimitUpEnabled:    true,
		LimitUp:           9.8,
		LimitDownEnabled:  true,
		LimitDown:         -9.8,
		DropSmallEnabled:  true,
		DropSmall:         -5.0,
		DropBigEnabled:    true,
		DropBig:           -9.0,
	}
}

// RapidMoveParams 急拉急跌参数
type RapidMoveParams struct {
	Minutes      int     `json:"minutes"`       // 时间窗口(分钟)
	AmplitudePct float64 `json:"amplitude_pct"` // 涨跌幅阈值(%)
	UpEnabled    bool    `json:"up_enabled"`    // 上涨方向启用
	DownEnabled  bool    `json:"down_enabled"`  // 下跌方向启用
}

// DefaultRapidMoveParams 默认急拉急跌参数
func DefaultRapidMoveParams() RapidMoveParams {
	return RapidMoveParams{Minutes: 5, AmplitudePct: 3.0, UpEnabled: true, DownEnabled: true}
}

// VolumeRatioParams 量比异动参数
type VolumeRatioParams struct {
	MinRatio float64 `json:"min_ratio"` // 量比阈值（当日量/5日均量）
}

// DefaultVolumeRatioParams 默认量比参数
func DefaultVolumeRatioParams() VolumeRatioParams {
	return VolumeRatioParams{MinRatio: 3.0}
}

// SealBoardParams 涨跌停封单数量监控参数
//
// 涨停时监控买一(bid)，跌停时监控卖一(ask)，共用同一阈值
type SealBoardParams struct {
	MinLots int `json:"min_lots"` // 封单小于此手数时告警（涨停看买一，跌停看卖一）
}

// DefaultSealBoardParams 默认封单参数
func DefaultSealBoardParams() SealBoardParams {
	return SealBoardParams{MinLots: 1000}
}

// ============================================================================
//  规则类型中文标签
// ============================================================================

// RuleTypeLabel 规则类型中文标签
var RuleTypeLabel = map[RuleType]string{
	RuleTypeDailyChange: "当日涨幅监控",
	RuleTypeRapidMove:   "急拉急跌监控",
	RuleTypeVolumeRatio: "量比异动监控",
	RuleTypeSealBoard:   "涨跌停封单监控",
}

// DailyChangeSubLabel 当日涨幅子标签
var DailyChangeSubLabel = map[string]string{
	"surge_big":   "大涨",
	"surge_small": "小涨",
	"limit_up":    "涨停",
	"limit_down":  "跌停",
	"drop_small":  "小跌",
	"drop_big":    "大跌",
}

// ============================================================================
//  冷却配置
// ============================================================================

// MonitorCooldown 冷却策略
type MonitorCooldown struct {
	IntervalMinutes int `json:"interval_minutes"` // 同股同类型告警最小间隔(分钟)
	DailyMax        int `json:"daily_max"`        // 每天同股同类型最多推送次数
}

// ============================================================================
//  Parse 方法
// ============================================================================

// ParseRule 解析 Rule JSON
func (c *MonitorConfig) ParseRule() (*MonitorRule, error) {
	if c.Rules == "" {
		return nil, nil
	}
	var rule MonitorRule
	if err := json.Unmarshal([]byte(c.Rules), &rule); err != nil {
		return nil, err
	}
	if rule.Type == "" {
		return nil, nil
	}
	return &rule, nil
}

// ParseCooldown 解析 Cooldown JSON
func (c *MonitorConfig) ParseCooldown() (*MonitorCooldown, error) {
	var cd MonitorCooldown
	if err := json.Unmarshal([]byte(c.Cooldown), &cd); err != nil {
		return nil, err
	}
	return &cd, nil
}

// ParseStocks 解析 Stocks JSON
func (c *MonitorConfig) ParseStocks() ([]string, error) {
	var stocks []string
	if c.Stocks == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(c.Stocks), &stocks); err != nil {
		return nil, err
	}
	return stocks, nil
}

// ============================================================================
//  MonitorConfigDetail 响应 DTO（含关联机器人名称）
// ============================================================================

// MonitorConfigDetail 监控配置详情（含关联机器人）
type MonitorConfigDetail struct {
	ID        uint              `json:"id"`
	UID       uint              `json:"uid"`
	Name      string            `json:"name"`
	Scope     MonitorScope      `json:"scope"`
	Stocks    []string          `json:"stocks"`
	Rule      MonitorRule      `json:"rule"`
	Cooldown  *MonitorCooldown  `json:"cooldown"`
	Template  string            `json:"template"`
	IsActive  bool              `json:"is_active"`
	Bots      []PushBot         `json:"bots"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
