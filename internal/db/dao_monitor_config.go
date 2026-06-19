package db

import (
	"errors"

	"stock-ai/internal/model"

	"gorm.io/gorm"
)

// ============================================================================
//  MonitorConfig 监控配置 DAO
// ============================================================================

// CreateMonitorConfig 创建监控配置
func CreateMonitorConfig(cfg *model.MonitorConfig) error {
	return GetDB().Create(cfg).Error
}

// GetMonitorConfigByID 根据 ID + UID 查询监控配置
func GetMonitorConfigByID(id, uid uint) (*model.MonitorConfig, error) {
	var cfg model.MonitorConfig
	err := GetDB().Where("id = ? AND uid = ?", id, uid).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &cfg, nil
}

// ListMonitorConfigs 查询用户监控配置列表（分页，按更新时间倒序）
func ListMonitorConfigs(uid uint, page, pageSize int) ([]model.MonitorConfig, int64, error) {
	query := GetDB().Model(&model.MonitorConfig{}).Where("uid = ?", uid)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var cfgs []model.MonitorConfig
	offset := (page - 1) * pageSize
	err := query.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&cfgs).Error
	return cfgs, total, err
}

// UpdateMonitorConfig 更新监控配置（全量 Save）
func UpdateMonitorConfig(cfg *model.MonitorConfig) error {
	return GetDB().Save(cfg).Error
}

// DeleteMonitorConfig 软删除监控配置（限制用户归属）
func DeleteMonitorConfig(id, uid uint) error {
	// 先清理关联机器人
	if err := GetDB().Where("config_id = ?", id).Delete(&model.MonitorConfigBot{}).Error; err != nil {
		return err
	}
	// 再软删除配置
	return GetDB().Where("id = ? AND uid = ?", id, uid).Delete(&model.MonitorConfig{}).Error
}

// SetMonitorConfigActive 切换监控配置启停状态
func SetMonitorConfigActive(id, uid uint, active bool) error {
	return GetDB().Model(&model.MonitorConfig{}).
		Where("id = ? AND uid = ?", id, uid).
		Update("is_active", active).Error
}

// GetAllActiveMonitorConfigs 获取所有活跃的监控配置（Monitor 启动时用，不按 uid 过滤）
func GetAllActiveMonitorConfigs() ([]model.MonitorConfig, error) {
	var cfgs []model.MonitorConfig
	err := GetDB().Where("is_active = ?", true).Find(&cfgs).Error
	return cfgs, err
}

// GetMonitorConfigByIDForMonitor 按 ID 查询监控配置（不校验 UID，Monitor 用）
func GetMonitorConfigByIDForMonitor(id uint) (*model.MonitorConfig, error) {
	var cfg model.MonitorConfig
	err := GetDB().Where("id = ?", id).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &cfg, nil
}

// ============================================================================
//  MonitorConfigBot 关联 DAO
// ============================================================================

// SetMonitorConfigBots 全量替换监控配置关联的机器人（事务内先删后插）
func SetMonitorConfigBots(configID uint, botIDs []uint) error {
	tx := GetDB().Begin()

	// 1. 先删除旧关联
	if err := tx.Where("config_id = ?", configID).Delete(&model.MonitorConfigBot{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 2. 再插入新关联
	for _, botID := range botIDs {
		mb := &model.MonitorConfigBot{
			ConfigID: configID,
			BotID:    botID,
		}
		if err := tx.Create(mb).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// GetMonitorConfigBots 获取监控配置关联的机器人列表（JOIN push_bots）
func GetMonitorConfigBots(configID uint) ([]model.PushBot, error) {
	var bots []model.PushBot
	err := GetDB().Table("push_bots").
		Joins("INNER JOIN monitor_config_bots ON push_bots.id = monitor_config_bots.bot_id").
		Where("monitor_config_bots.config_id = ?", configID).
		Find(&bots).Error
	return bots, err
}

// CountUserMonitorConfigs 统计用户监控配置数（用于上限校验）
func CountUserMonitorConfigs(uid uint) (int64, error) {
	var count int64
	err := GetDB().Model(&model.MonitorConfig{}).Where("uid = ?", uid).Count(&count).Error
	return count, err
}
