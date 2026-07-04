package db

import (
	"gorm.io/gorm"

	"stock-ai/internal/model"
)

// CreateStrategy 创建策略
func CreateStrategy(s *model.Strategy) error {
	return GetDB().Create(s).Error
}

// CopyStrategy 复制策略（读取原策略，以新用户身份创建副本）
func CopyStrategy(originalID, newUID uint) (*model.Strategy, error) {
	var original model.Strategy
	if err := GetDB().First(&original, originalID).Error; err != nil {
		return nil, err
	}

	copy := model.Strategy{
		UID:         newUID,
		Name:        original.Name + " - 副本",
		Description:  original.Description,
		LogicalOp:    original.LogicalOp,
		Conditions:   original.Conditions,
		ExitRules:    original.ExitRules,
		PositionRules: original.PositionRules,
		BacktestCount: 0,
		IsPublic:     false,
		StarCount:    0,
	}
	// JSON 类型列不允许空字符串，原策略为空时设为 "{}"
	if copy.ExitRules == "" {
		copy.ExitRules = "{}"
	}
	if copy.PositionRules == "" {
		copy.PositionRules = "{}"
	}

	if err := GetDB().Create(&copy).Error; err != nil {
		return nil, err
	}
	return &copy, nil
}

// GetStrategyByUID 根据 UID 查询策略（UID 为用户ID）
func GetStrategyByUID(uid uint) (*model.Strategy, error) {
	var s model.Strategy
	err := GetDB().Where("uid = ?", uid).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetStrategyByID 根据 ID 查询策略
func GetStrategyByID(id uint) (*model.Strategy, error) {
	var s model.Strategy
	err := GetDB().First(&s, id).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListStrategies 查询策略列表（支持关键词搜索+用户过滤，按更新时间倒序）
// 管理员：查看所有策略
// 普通用户：查看自己的策略 + 公开策略（uid = ? OR is_public = true）
// 通过关联子查询一次性填充订阅数量，避免 N+1 问题
func ListStrategies(uid uint, isAdmin bool, keyword string, page, pageSize int) ([]model.Strategy, int64, error) {
	base := GetDB().Model(&model.Strategy{})

	if !isAdmin {
		base = base.Where("uid = ? OR is_public = ?", uid, true)
	}

	if keyword != "" {
		base = base.Where("name LIKE ?", "%"+keyword+"%")
	}

	// Count 使用独立 session，避免影响后续 Select
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 子查询：策略的订阅数量
	subQuery := `COALESCE(
		(SELECT COUNT(*) FROM strategy_subscriptions
		 WHERE strategy_subscriptions.strategy_id = strategies.id
		   AND strategy_subscriptions.deleted_at IS NULL), 0
	) AS subscription_count`

	var strategies []model.Strategy
	offset := (page - 1) * pageSize
	err := base.
		Select("strategies.*, "+subQuery).
		Order("updated_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&strategies).Error

	return strategies, total, err
}

// UpdateStrategy 更新策略
func UpdateStrategy(s *model.Strategy) error {
	return GetDB().Save(s).Error
}

// DeleteStrategyByID 按 ID 软删除策略
func DeleteStrategyByID(id, uid uint) error {
	return GetDB().Where("id = ?", id).Delete(&model.Strategy{}).Error
}

// DeleteStrategyByIDs 批量软删除策略
func DeleteStrategyByIDs(ids []uint, uid uint) error {
	return GetDB().Where("id IN ?", ids).Delete(&model.Strategy{}).Error
}

// RenameStrategy 重命名策略
func RenameStrategy(id uint, newName string) error {
	return GetDB().Model(&model.Strategy{}).Where("id = ?", id).
		Update("name", newName).Error
}

// SetStrategyPublic 设置策略的公开状态
func SetStrategyPublic(id uint, isPublic bool) error {
	return GetDB().Model(&model.Strategy{}).Where("id = ?", id).
		Update("is_public", isPublic).Error
}
