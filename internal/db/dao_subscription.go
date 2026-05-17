package db

import (
	"errors"
	"fmt"

	"stock-ai/internal/model"

	"gorm.io/gorm"
)

// ============================================================================
//  Subscription 订阅配置 DAO
// ============================================================================

// CreateSubscription 创建订阅
func CreateSubscription(sub *model.Subscription) error {
	return GetDB().Create(sub).Error
}

// GetSubscriptionByID 根据 ID + UID 查询订阅
func GetSubscriptionByID(id, uid uint) (*model.Subscription, error) {
	var sub model.Subscription
	err := GetDB().Where("id = ? AND uid = ?", id, uid).First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &sub, nil
}

// ListSubscriptions 查询用户订阅列表（分页，按更新时间倒序）
func ListSubscriptions(uid uint, page, pageSize int) ([]model.Subscription, int64, error) {
	query := GetDB().Model(&model.Subscription{}).Where("uid = ?", uid)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var subs []model.Subscription
	offset := (page - 1) * pageSize
	err := query.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&subs).Error
	return subs, total, err
}

// UpdateSubscription 更新订阅（全量 Save）
func UpdateSubscription(sub *model.Subscription) error {
	return GetDB().Save(sub).Error
}

// DeleteSubscription 软删除订阅（限制用户归属）
func DeleteSubscription(id, uid uint) error {
	// 先清理关联的 subscription_bots 记录
	if err := GetDB().Where("subscription_id = ?", id).Delete(&model.SubscriptionBot{}).Error; err != nil {
		return fmt.Errorf("清理订阅关联机器人失败: %w", err)
	}
	// 再软删除订阅
	return GetDB().Where("id = ? AND uid = ?", id, uid).Delete(&model.Subscription{}).Error
}

// SetActive 切换订阅启停状态
func SetActive(id, uid uint, active bool) error {
	return GetDB().Model(&model.Subscription{}).
		Where("id = ? AND uid = ?", id, uid).
		Update("is_active", active).Error
}

// GetActiveSubscriptions 获取所有活跃订阅（Scheduler 启动时用，不按 uid 过滤）
func GetActiveSubscriptions() ([]model.Subscription, error) {
	var subs []model.Subscription
	err := GetDB().Where("is_active = ?", true).Find(&subs).Error
	return subs, err
}

// SetSubscriptionBots 全量替换订阅关联的机器人（先删后插）
func SetSubscriptionBots(subscriptionID uint, botIDs []uint) error {
	tx := GetDB().Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 删除旧的关联
	if err := tx.Where("subscription_id = ?", subscriptionID).Delete(&model.SubscriptionBot{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 插入新的关联
	if len(botIDs) > 0 {
		bots := make([]model.SubscriptionBot, len(botIDs))
		for i, botID := range botIDs {
			bots[i] = model.SubscriptionBot{
				SubscriptionID: subscriptionID,
				BotID:          botID,
			}
		}
		if err := tx.Create(&bots).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// GetSubscriptionBots 获取订阅关联的机器人列表（JOIN 查询）
func GetSubscriptionBots(subscriptionID uint) ([]model.PushBot, error) {
	var bots []model.PushBot
	err := GetDB().
		Joins("JOIN subscription_bots sb ON sb.bot_id = push_bots.id").
		Where("sb.subscription_id = ?", subscriptionID).
		Find(&bots).Error
	return bots, err
}

// CreateSubscriptionLog 创建订阅执行日志
func CreateSubscriptionLog(log *model.SubscriptionLog) error {
	return GetDB().Create(log).Error
}

// ListSubscriptionLogs 查询订阅执行日志（分页，支持状态过滤）
func ListSubscriptionLogs(subscriptionID uint, page, pageSize int, status string) ([]model.SubscriptionLog, int64, error) {
	query := GetDB().Model(&model.SubscriptionLog{}).Where("subscription_id = ?", subscriptionID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.SubscriptionLog
	offset := (page - 1) * pageSize
	err := query.Order("run_time DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// UpdateLastRunTime 更新订阅最后运行时间
func UpdateLastRunTime(id uint) error {
	return GetDB().Model(&model.Subscription{}).Where("id = ?", id).
		Update("last_run_at", gorm.Expr("NOW()")).Error
}

// CountUserSubscriptions 统计用户订阅数量（上限校验用）
func CountUserSubscriptions(uid uint) (int64, error) {
	var count int64
	err := GetDB().Model(&model.Subscription{}).Where("uid = ?", uid).Count(&count).Error
	return count, err
}

// GetSubscriptionByIDForScheduler 根据 ID 查询订阅（不限制 uid，Scheduler 用）
func GetSubscriptionByIDForScheduler(id uint) (*model.Subscription, error) {
	var sub model.Subscription
	err := GetDB().First(&sub, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &sub, nil
}
