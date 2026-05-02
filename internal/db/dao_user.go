package db

import (
	"stock-ai/internal/model"
	"time"
)

// CreateUser 创建用户
func CreateUser(u *model.User) error {
	return GetDB().Create(u).Error
}

// GetUserByUsername 根据用户名查询
func GetUserByUsername(username string) (*model.User, error) {
	var u model.User
	err := GetDB().Where("username = ? AND status = 1", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByID 根据 ID 查询
func GetUserByID(id uint) (*model.User, error) {
	var u model.User
	err := GetDB().Where("id = ? AND status = 1", id).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateUserLastLogin 更新最后登录时间
func UpdateUserLastLogin(id uint) error {
	now := time.Now()
	return GetDB().Model(&model.User{}).Where("id = ?", id).
		Update("last_login_at", now).Error
}

// UpdateUserProfile 更新用户资料（昵称/头像）
func UpdateUserProfile(id uint, nickname, avatar string) error {
	updates := map[string]interface{}{}
	if nickname != "" {
		updates["nickname"] = nickname
	}
	if avatar != "" {
		updates["avatar"] = avatar
	}
	if len(updates) == 0 {
		return nil
	}
	return GetDB().Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

// UpdatePassword 更新密码
func UpdatePassword(id uint, password string) error {
	return GetDB().Model(&model.User{}).Where("id = ?", id).
		Update("password", password).Error
}

// UsernameExists 检查用户名是否已存在
func UsernameExists(username string) (bool, error) {
	var count int64
	err := GetDB().Model(&model.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// ListAllUsers 查询所有用户（管理后台用，含已禁用的）
func ListAllUsers() ([]model.User, error) {
	var users []model.User
	err := GetDB().Order("id ASC").Find(&users).Error
	return users, err
}

// UpdateUserStatus 更新用户状态（启用/禁用）
func UpdateUserStatus(id uint, status int) error {
	return GetDB().Model(&model.User{}).Where("id = ?", id).
		Update("status", status).Error
}
