package model

import "time"

// UserSoftwareConfig 用户软件配置（如东方财富/同花顺 cookie）
type UserSoftwareConfig struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	UserID       uint      `gorm:"not null;index;column:user_id" json:"user_id"`
	SoftwareName string    `gorm:"size:50;not null;column:software_name" json:"software_name"`
	DisplayName  string    `gorm:"size:100;column:display_name" json:"display_name"`
	Cookie       string    `gorm:"type:text;column:cookie" json:"cookie"`
	Extra        string    `gorm:"type:json;column:extra" json:"extra"`
	Enabled      bool      `gorm:"not null;default:1;column:enabled" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (UserSoftwareConfig) TableName() string { return "user_software_configs" }

// SoftwareConfigItem 返回给前端的配置项（简化，不含 user_id）
type SoftwareConfigItem struct {
	SoftwareName string `json:"software_name"`
	DisplayName  string `json:"display_name"`
	Cookie       string `json:"cookie"`
	Extra        string `json:"extra"`
	Enabled      bool   `json:"enabled"`
	UpdatedAt    string `json:"updated_at"`
}
