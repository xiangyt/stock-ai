package backtest

import (
	"stock-ai/internal/db"
)

// DAO 回测数据持久层
type DAO struct{}

// NewDAO 创建 DAO 实例
func NewDAO() *DAO { return &DAO{} }

// =========================== BacktestRun ===========================

// CreateRun 创建回测运行记录
func (d *DAO) CreateRun(run *BacktestRun) error {
	return db.GetDB().Create(run).Error
}

// UpdateRun 更新回测运行记录（按 ID，只更新非零值字段）
func (d *DAO) UpdateRun(run *BacktestRun) error {
	return db.GetDB().Model(run).Updates(run).Error
}

// GetRun 按 ID 获取回测运行记录
func (d *DAO) GetRun(id uint64) (*BacktestRun, error) {
	var run BacktestRun
	err := db.GetDB().First(&run, id).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// GetRunsByStrategy 按策略 ID 获取回测历史（最新在前）
func (d *DAO) GetRunsByStrategy(strategyID uint64, limit int) ([]BacktestRun, error) {
	var runs []BacktestRun
	err := db.GetDB().
		Where("strategy_id = ?", strategyID).
		Order("id DESC").
		Limit(limit).
		Find(&runs).Error
	return runs, err
}

// GetRunningRuns 获取所有运行中的回测
func (d *DAO) GetRunningRuns() ([]BacktestRun, error) {
	var runs []BacktestRun
	err := db.GetDB().Where("status = ?", "running").Find(&runs).Error
	return runs, err
}

// DeleteRun 删除回测记录及其关联数据
func (d *DAO) DeleteRun(id uint64) error {
	tx := db.GetDB().Begin()
	if err := tx.Where("run_id = ?", id).Delete(&BacktestTrade{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("run_id = ?", id).Delete(&DailySnapshot{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&BacktestRun{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// =========================== BacktestTrade ===========================

// CreateTrades 批量创建交易记录
func (d *DAO) CreateTrades(trades []BacktestTrade) error {
	if len(trades) == 0 {
		return nil
	}
	// 分批插入，每批 500 条
	batchSize := 500
	for i := 0; i < len(trades); i += batchSize {
		end := i + batchSize
		if end > len(trades) {
			end = len(trades)
		}
		if err := db.GetDB().Create(trades[i:end]).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetTradesByRun 按回测 ID 获取交易记录（分页）
func (d *DAO) GetTradesByRun(runID uint64, offset, limit int) ([]BacktestTrade, int64, error) {
	var total int64
	if err := db.GetDB().Model(&BacktestTrade{}).Where("run_id = ?", runID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var trades []BacktestTrade
	err := db.GetDB().
		Where("run_id = ?", runID).
		Order("trade_date ASC, id ASC").
		Offset(offset).
		Limit(limit).
		Find(&trades).Error
	return trades, total, err
}

// GetTradesByRunAll 获取回测的全部交易记录（不分页）
func (d *DAO) GetTradesByRunAll(runID uint64) ([]BacktestTrade, error) {
	var trades []BacktestTrade
	err := db.GetDB().
		Where("run_id = ?", runID).
		Order("trade_date ASC, id ASC").
		Find(&trades).Error
	return trades, err
}

// =========================== DailySnapshot ===========================

// CreateSnapshot 插入单条每日快照
func (d *DAO) CreateSnapshot(snapshot *DailySnapshot) error {
	return db.GetDB().Create(snapshot).Error
}

// CreateSnapshots 批量创建每日快照
func (d *DAO) CreateSnapshots(snapshots []DailySnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	batchSize := 500
	for i := 0; i < len(snapshots); i += batchSize {
		end := i + batchSize
		if end > len(snapshots) {
			end = len(snapshots)
		}
		if err := db.GetDB().Create(snapshots[i:end]).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetSnapshotsByRun 获取回测的每日快照（按日期升序）
func (d *DAO) GetSnapshotsByRun(runID uint64) ([]DailySnapshot, error) {
	var snapshots []DailySnapshot
	err := db.GetDB().
		Where("run_id = ?", runID).
		Order("snap_date ASC").
		Find(&snapshots).Error
	return snapshots, err
}
