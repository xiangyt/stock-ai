package db

import (
	"log"

	"stock-ai/internal/model"

	"gorm.io/gorm/clause"
)

// ========== 快照 Upsert ==========

var snapshotUpdateCols = []string{
	"pe_dynamic", "pe_static", "pe_ttm", "ps_ttm", "pb",
	"roe", "roa", "gross_margin", "net_margin",
	"bvps", "basic_eps",
	"parent_net_profit", "deduct_net_profit", "total_revenue",
	"total_shares", "float_shares",
	"total_market_cap", "circulate_market_cap",
	"debt_ratio",
}

// UpsertSnapshot upsert 一条快照记录 (stock_code + trade_date 联合唯一)
//
// MySQL ON DUPLICATE KEY UPDATE 的 RowsAffected 语义：
//   - 1: 新增（INSERT）
//   - 2: 更新且数据有变化（UPDATE）
//   - 0: 数据未变化，无需更新（仍算成功）
//
// 返回值: true=成功（含无需更新的情况）, false=出错
func UpsertSnapshot(m model.StockDailySnapshot) bool {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(snapshotUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-snapshot] upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return false
	}
	return true
}

// BatchUpsertSnapshots 批量 upsert 快照记录
// 返回成功条数（含数据未变化视为成功）和首个错误
func BatchUpsertSnapshots(snapshots []model.StockDailySnapshot) (int64, error) {
	if len(snapshots) == 0 {
		return 0, nil
	}
	var successCount int64
	var firstErr error
	for _, s := range snapshots {
		if UpsertSnapshot(s) {
			successCount++
		} else if firstErr == nil {
			firstErr = GetDB().Error
		}
	}
	return successCount, firstErr
}

// ========== 快照查询 ==========

// FindSnapshotsByStock 查询指定股票的快照（按日期范围）
func FindSnapshotsByStock(code string, startDate, endDate int, limit int) ([]model.StockDailySnapshot, error) {
	var snaps []model.StockDailySnapshot
	q := GetDB().Where("stock_code = ?", code)
	if startDate > 0 {
		q = q.Where("trade_date >= ?", startDate)
	}
	if endDate > 0 {
		q = q.Where("trade_date <= ?", endDate)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Order("trade_date ASC").Find(&snaps).Error
	return snaps, err
}

// FindSnapshotByStockAndDate 查询指定股票指定日期的单条快照
func FindSnapshotByStockAndDate(code string, tradeDate int) (model.StockDailySnapshot, error) {
	var snap model.StockDailySnapshot
	err := GetDB().
		Where("stock_code = ? AND trade_date = ?", code, tradeDate).
		First(&snap).Error
	return snap, err
}

// FindSnapshotsByDate 查询指定日期的所有股票快照
func FindSnapshotsByDate(tradeDate int, offset, limit int) ([]model.StockDailySnapshot, error) {
	var snaps []model.StockDailySnapshot
	q := GetDB().Where("trade_date = ?", tradeDate)
	if offset > 0 {
		q = q.Offset(offset)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Order("stock_code ASC").Find(&snaps).Error
	return snaps, err
}

// CountSnapshotsByStock 统计指定股票的快照数量
func CountSnapshotsByStock(code string) (int64, error) {
	var count int64
	err := GetDB().Model(&model.StockDailySnapshot{}).
		Where("stock_code = ?", code).
		Count(&count).Error
	return count, err
}

// CountSnapshotsByDate 统计指定日期的快照数量
func CountSnapshotsByDate(tradeDate int) (int64, error) {
	var count int64
	err := GetDB().Model(&model.StockDailySnapshot{}).
		Where("trade_date = ?", tradeDate).
		Count(&count).Error
	return count, err
}

// DeleteSnapshotsByStock 删除指定股票的所有快照（重新计算前清理）
func DeleteSnapshotsByStock(code string) (int64, error) {
	result := GetDB().
		Where("stock_code = ?", code).
		Delete(&model.StockDailySnapshot{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// DeleteSnapshotByStockAndDate 删除指定股票指定日期的快照
func DeleteSnapshotByStockAndDate(code string, tradeDate int) (int64, error) {
	result := GetDB().
		Where("stock_code = ? AND trade_date = ?", code, tradeDate).
		Delete(&model.StockDailySnapshot{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
