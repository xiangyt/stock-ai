package db

import (
	"stock-ai/internal/model"
)

// ============================================================================
//  Position 持仓表 DAO
// ============================================================================

// CreatePosition 创建持仓记录
func CreatePosition(p *model.Position) error {
	return GetDB().Create(p).Error
}

// GetPositionByID 根据 ID 查询持仓
func GetPositionByID(id uint) (*model.Position, error) {
	var p model.Position
	err := GetDB().First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPositionByIDAndUID 根据 ID + 用户ID 查询持仓（权限校验）
func GetPositionByIDAndUID(id, uid uint) (*model.Position, error) {
	var p model.Position
	err := GetDB().Where("id = ? AND uid = ?", id, uid).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPositions 查询用户持仓列表（支持状态过滤+分页）
// statusFilter: 空字符串=全部, "holding"=持有中, "closed"=已清仓
func ListPositions(uid uint, statusFilter string, page, pageSize int) ([]model.Position, int64, error) {
	query := GetDB().Model(&model.Position{}).Where("uid = ?", uid)
	if statusFilter != "" && statusFilter != "all" {
		query = query.Where("status = ?", statusFilter)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var positions []model.Position
	offset := (page - 1) * pageSize
	err := query.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&positions).Error

	return positions, total, err
}

// UpdatePosition 更新持仓（全量 Save）
func UpdatePosition(p *model.Position) error {
	return GetDB().Save(p).Error
}

// DeletePositionByID 按 ID 软删除持仓（限制用户归属）
func DeletePositionByID(id, uid uint) error {
	return GetDB().Where("id = ? AND uid = ?", id, uid).Delete(&model.Position{}).Error
}

// GetPositionByStockCodeAndUID 根据股票代码+用户查询当前持仓（建仓前检查是否已存在）
func GetPositionByStockCodeAndUID(stockCode string, uid uint) (*model.Position, error) {
	var p model.Position
	err := GetDB().Where("stock_code = ? AND uid = ? AND status = ?", stockCode, uid, model.PositionHolding).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ============================================================================
//  PositionTrade 交易记录 DAO
// ============================================================================

// CreateTrade 创建交易记录
func CreateTrade(t *model.PositionTrade) error {
	return GetDB().Create(t).Error
}

// ListTradesByPositionID 查询某持仓的交易记录（按创建时间倒序）
func ListTradesByPositionID(positionID uint) ([]model.PositionTrade, error) {
	var trades []model.PositionTrade
	err := GetDB().Where("position_id = ?", positionID).
		Order("created_at ASC").
		Find(&trades).Error
	return trades, err
}

// CountTradesByPositionID 统计某持仓的交易笔数
func CountTradesByPositionID(positionID uint) (int64, error) {
	var count int64
	err := GetDB().Model(&model.PositionTrade{}).
		Where("position_id = ?", positionID).
		Count(&count).Error
	return count, err
}
