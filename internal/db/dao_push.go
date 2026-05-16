package db

import (
	"errors"
	"stock-ai/internal/model"
)

// ErrRecordNotFound 记录不存在或无权操作
var ErrRecordNotFound = errors.New("记录不存在")

// CreatePushConfig 创建推送配置
func CreatePushConfig(cfg *model.PushConfig) error {
	return GetDB().Create(cfg).Error
}

// ListPushConfigs 查询用户的推送配置列表
func ListPushConfigs(userID uint) ([]model.PushConfig, error) {
	var list []model.PushConfig
	err := GetDB().Where("user_id = ?", userID).Order("id DESC").Find(&list).Error
	return list, err
}

// GetPushConfigByID 根据 ID 查询（带归属校验）
func GetPushConfigByID(id, userID uint) (*model.PushConfig, error) {
	var cfg model.PushConfig
	err := GetDB().Where("id = ? AND user_id = ?", id, userID).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdatePushConfig 更新推送配置（带归属校验）
func UpdatePushConfig(id, userID uint, updates map[string]interface{}) error {
	result := GetDB().Model(&model.PushConfig{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates)
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return result.Error
}

// DeletePushConfig 删除推送配置（带归属校验）
func DeletePushConfig(id, userID uint) error {
	result := GetDB().Where("id = ? AND user_id = ?", id, userID).Delete(&model.PushConfig{})
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return result.Error
}

// UpdatePushStatus 更新推送状态（启用/禁用，带归属校验）
func UpdatePushStatus(id, userID uint, status int) error {
	result := GetDB().Model(&model.PushConfig{}).Where("id = ? AND user_id = ?", id, userID).Update("status", status)
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return result.Error
}
