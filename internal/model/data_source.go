package model

import (
	"time"
)

// ========== 数据源配置相关 ==========

// DataSourceConfig 数据源配置表
// 支持多个数据源，每个数据源可独立配置
type DataSourceConfig struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	Name         string     `gorm:"uniqueIndex;size:50;not null" json:"name"` // 数据源标识名: tushare / eastmoney / akshare
	DisplayName  string     `gorm:"size:100" json:"display_name"`             // 显示名称: Tushare Pro / 东方财富 / AKShare
	Type         string     `gorm:"size:20;not null" json:"type"`             // 类型: api / sdk / web_crawl
	Status       string     `gorm:"size:20;default:active" json:"status"`     // 状态: active / disabled / error
	Priority     int        `gorm:"default:0" json:"priority"`                // 优先级，数字越小优先级越高
	Config       string     `gorm:"type:text" json:"config"`                  // JSON格式配置 (API Key、URL等)
	RateLimit    int        `gorm:"default:60" json:"rate_limit"`             // 每分钟请求限制
	DailyQuota   int        `gorm:"default:0" json:"daily_quota"`             // 每日调用配额(0=无限制)
	UsedQuota    int        `gorm:"default:0" json:"used_quota"`              // 已使用配额
	QuotaResetAt *time.Time `json:"quota_reset_at"`                           // 配额重置时间
	Description  string     `gorm:"size:500" json:"description"`              // 描述
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName 表名
func (DataSourceConfig) TableName() string { return "data_source_configs" }

// IsActive 判断是否可用
func (d *DataSourceConfig) IsActive() bool {
	return d.Status == "active"
}

// IsQuotaAvailable 检查配额是否充足
func (d *DataSourceConfig) IsQuotaAvailable() bool {
	if d.DailyQuota <= 0 {
		return true // 无限制
	}
	return d.UsedQuota < d.DailyQuota
}
