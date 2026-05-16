package model

import "time"

// PushConfig 推送配置表
type PushConfig struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Name       string    `gorm:"size:100;not null" json:"name"`
	Channel    string    `gorm:"size:20;not null" json:"channel"`    // qq | wecom | dingtalk | feishu
	WebhookURL string    `gorm:"size:500" json:"webhook_url"`
	Token      string    `gorm:"size:255" json:"token"`
	Secret     string    `gorm:"size:255" json:"secret"`            // 钉钉加签密钥
	Status     int       `gorm:"default:1" json:"status"`           // 0=禁用 1=启用
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PushConfig) TableName() string {
	return "push_configs"
}
