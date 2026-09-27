package db

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"stock-ai/internal/model"
)

// condUsingStrategy "用户已订阅且订阅处于启用状态" 的 SQL 条件，
// 占位符依次为订阅用户ID、is_active。
const condUsingStrategy = `EXISTS (
	SELECT 1 FROM strategy_subscriptions
	WHERE strategy_subscriptions.strategy_id = strategies.id
	  AND strategy_subscriptions.uid = ?
	  AND strategy_subscriptions.is_active = ?
	  AND strategy_subscriptions.deleted_at IS NULL)`

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
func ListStrategies(ctx context.Context, uid uint, isAdmin bool, keyword string, page, pageSize int) ([]model.Strategy, int64, error) {
	base := GetDB().WithContext(ctx).Model(&model.Strategy{})

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

// ListSubscribedStrategyIDs 查询用户已订阅的策略ID列表（可能重复，按订阅记录顺序）。
//
// activeOnly 为 true 时仅返回启用中（is_active）订阅对应的策略ID。
func ListSubscribedStrategyIDs(uid uint, activeOnly bool) ([]uint, error) {
	q := GetDB().Model(&model.Subscription{}).Where("uid = ?", uid)
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}

	var ids []uint
	if err := q.Pluck("strategy_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list subscribed strategy ids(uid=%d, activeOnly=%v): %w", uid, activeOnly, err)
	}
	return ids, nil
}

// ListUserStrategies 查询用户可见的策略列表，按更新时间倒序。
//
// 可见范围：管理员可见全部策略；普通用户可见自己创建的（uid）与公开的（is_public）策略。
// usingOnly 为 true 时，仅返回该用户已订阅且订阅处于启用状态的策略。
func ListUserStrategies(ctx context.Context, uid uint, isAdmin, usingOnly bool) ([]model.Strategy, error) {
	q := GetDB().WithContext(ctx).Model(&model.Strategy{})
	switch {
	case usingOnly:
		q = q.Where(condUsingStrategy, uid, true)
	case !isAdmin:
		q = q.Where("strategies.uid = ? OR strategies.is_public = ?", uid, true)
	}

	var strategies []model.Strategy
	if err := q.Order("strategies.updated_at DESC").Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("list user strategies(uid=%d, usingOnly=%v): %w", uid, usingOnly, err)
	}
	return strategies, nil
}

// GetVisibleStrategyByID 按ID查询用户可见的策略。
//
// 可见范围：管理员可见全部策略；普通用户可见自己创建的（uid）与公开的（is_public）策略。
// 策略不存在或不可见时返回 ErrRecordNotFound。
func GetVisibleStrategyByID(ctx context.Context, uid uint, isAdmin bool, id uint) (*model.Strategy, error) {
	q := GetDB().WithContext(ctx).Model(&model.Strategy{}).Where("strategies.id = ?", id)
	if !isAdmin {
		q = q.Where("strategies.uid = ? OR strategies.is_public = ?", uid, true)
	}

	var strategy model.Strategy
	if err := q.First(&strategy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("get visible strategy(uid=%d, id=%d): %w", uid, id, err)
	}
	return &strategy, nil
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
