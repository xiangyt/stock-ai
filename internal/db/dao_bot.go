package db

import (
	"errors"
	"stock-ai/internal/model"
)

// ErrRecordNotFound 记录不存在或无权操作
var ErrRecordNotFound = errors.New("记录不存在")

// CreatePushBot 创建推送配置
func CreatePushBot(cfg *model.PushBot) error {
	return GetDB().Create(cfg).Error
}

// ListPushBots 查询用户的推送配置列表
func ListPushBots(userID uint) ([]model.PushBot, error) {
	var list []model.PushBot
	err := GetDB().Where("user_id = ?", userID).Order("id DESC").Find(&list).Error
	return list, err
}

// GetPushBotByID 根据 ID 查询（带归属校验）
func GetPushBotByID(id, userID uint) (*model.PushBot, error) {
	var cfg model.PushBot
	err := GetDB().Where("id = ? AND user_id = ?", id, userID).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdatePushBot 更新推送配置（带归属校验）
func UpdatePushBot(id, userID uint, updates map[string]interface{}) error {
	result := GetDB().Model(&model.PushBot{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates)
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return result.Error
}

// DeletePushBot 删除推送配置（带归属校验）
func DeletePushBot(id, userID uint) error {
	result := GetDB().Where("id = ? AND user_id = ?", id, userID).Delete(&model.PushBot{})
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return result.Error
}

// UpdatePushStatus 更新推送状态（启用/禁用，带归属校验）
func UpdatePushStatus(id, userID uint, status int) error {
	result := GetDB().Model(&model.PushBot{}).Where("id = ? AND user_id = ?", id, userID).Update("status", status)
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return result.Error
}
