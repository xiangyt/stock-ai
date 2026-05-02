package model

import "time"

// User 用户表
type User struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	Username     string    `gorm:"size:50;uniqueIndex;not null" json:"username"`       // 登录用户名
	Password     string    `gorm:"size:32;not null;column:password" json:"-"`               // MD5 密码
	Nickname     string    `gorm:"size:50" json:"nickname"`                           // 昵称/显示名
	Avatar       string    `gorm:"size:255" json:"avatar"`                           // 头像 URL
	Role         string    `gorm:"size:20;default:user" json:"role"`                 // 角色: user/admin
	Status       int       `gorm:"default:1" json:"status"`                          // 0=禁用 1=正常
	LastLoginAt  *time.Time `json:"last_login_at"`                                   // 最后登录时间
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string { return "users" }

// UserInfo 返回给前端的用户信息（不含敏感字段）
type UserInfo struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}
