package db

import (
	"stock-ai/internal/model"
)

// CreateStrategy 创建策略
func CreateStrategy(s *model.Strategy) error {
	return GetDB().Create(s).Error
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
func ListStrategies(uid uint, keyword string, page, pageSize int) ([]model.Strategy, int64, error) {
	query := GetDB().Model(&model.Strategy{}).Where("uid = ?", uid)

	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var strategies []model.Strategy
	offset := (page - 1) * pageSize
	err := query.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&strategies).Error

	return strategies, total, err
}

// UpdateStrategy 更新策略
func UpdateStrategy(s *model.Strategy) error {
	return GetDB().Save(s).Error
}

// DeleteStrategyByID 按 ID 软删除策略（限制用户归属）
func DeleteStrategyByID(id, uid uint) error {
	return GetDB().Where("id = ? AND uid = ?", id, uid).Delete(&model.Strategy{}).Error
}

// DeleteStrategyByIDs 批量软删除策略（限制用户归属）
func DeleteStrategyByIDs(ids []uint, uid uint) error {
	return GetDB().Where("id IN ? AND uid = ?", ids, uid).Delete(&model.Strategy{}).Error
}

// RenameStrategy 重命名策略
func RenameStrategy(id uint, newName string) error {
	return GetDB().Model(&model.Strategy{}).Where("id = ?", id).
		Update("name", newName).Error
}
