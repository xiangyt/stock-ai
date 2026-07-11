package db

import (
	"errors"
	"log"

	"stock-ai/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ========== 基本面/财务面 DAO ==========

// UpsertPerformanceReport 单条财报 upsert (INSERT ON DUPLICATE KEY UPDATE)
func UpsertPerformanceReport(m model.PerformanceReport) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "stock_code"}, {Name: "report_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"report_type", "report_name", "currency",
			"notice_date",
			"basic_eps", "deducted_eps", "diluted_eps", "bvps",
			"equity_reserve", "undistributed_profit", "ocfps",
			"total_revenue", "gross_profit", "parent_net_profit", "deduct_net_profit",
			"revenue_yoy", "parent_net_profit_yoy", "deduct_net_profit_yoy",
			"roe_w", "roe_dw", "roa", "gross_margin", "net_margin",
			"current_ratio", "quick_ratio", "debt_ratio",
			"ocf_to_revenue",
		}),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-fundamental] 财报upsert失败 [%s/%d]: %v", m.StockCode, m.ReportDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// UpsertShareholderCount 单条股东户数 upsert (INSERT ON DUPLICATE KEY UPDATE)
func UpsertShareholderCount(m model.ShareholderCount) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "stock_code"}, {Name: "end_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"security_name", "holder_num", "holder_num_change_pct",
			"avg_free_shares", "avg_free_shares_change_pct",
			"hold_focus", "price", "avg_hold_amount",
			"hold_ratio_total", "free_hold_ratio_total",
		}),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-fundamental] 股东户数upsert失败 [%s/%d]: %v", m.StockCode, m.EndDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// UpsertShareChange 单条股本变动 upsert (INSERT ON DUPLICATE KEY UPDATE)
func UpsertShareChange(m model.ShareChange) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "stock_code"}, {Name: "change_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"change_reason", "total_shares", "limited_shares",
			"unlimited_shares", "float_a_shares",
		}),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-fundamental] 股本变动upsert失败 [%s/%d]: %v", m.StockCode, m.ChangeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// FindPerformanceReports 查询股票财报列表（按报告日期降序）
func FindPerformanceReports(code string, endDate, limit int) ([]model.PerformanceReport, error) {
	var reports []model.PerformanceReport
	q := GetDB().Where("stock_code = ?", code).Order("report_date DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&reports).Error
	return reports, err
}

// FindLatestShareholderCount 查询最新股东户数
func FindLatestShareholderCount(code string, endDate int) (*model.ShareholderCount, error) {
	var sc model.ShareholderCount
	db := GetDB().Where("stock_code = ?", code)
	if endDate > 0 {
		db = db.Where("end_date <= ?", endDate)
	}
	err := db.Order("end_date DESC").First(&sc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &sc, err
}

// FindShareChanges 查询股本变动记录
func FindShareChanges(code string, limit int) ([]model.ShareChange, error) {
	var changes []model.ShareChange
	q := GetDB().Where("stock_code = ?", code).Order("change_date DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&changes).Error
	return changes, err
}

// UpsertDividendHistory 单条分红历史 upsert (INSERT ON DUPLICATE KEY UPDATE)
func UpsertDividendHistory(m model.DividendHistory) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "stock_code"}, {Name: "notice_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"security_name", "plan_profile", "assign_progress",
			"equity_record_date", "ex_dividend_date", "pay_cash_date",
			"is_unassign", "report_period", "assign_object", "new_profile",
			"gm_decision_notice_date", "annual_report_date",
			"total_dividend", "total_dividend_a", "report_time",
		}),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-fundamental] 分红upsert失败 [%s/%d]: %v", m.StockCode, m.NoticeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// FindDividendHistory 查询分红历史
func FindDividendHistory(code string, limit int) ([]model.DividendHistory, error) {
	var dividends []model.DividendHistory
	q := GetDB().Where("stock_code = ?", code).Order("notice_date DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&dividends).Error
	return dividends, err
}

// FindLatestDividend 查询指定股票最新一条除权除息记录（按 ex_dividend_date 降序）。
// 用于判断最近一次除权除息日是否在同一个周期内（日/周/月/年），
// 以触发 dividend 模式全量刷新 OHLCV。
func FindLatestDividend(code string) (model.DividendHistory, error) {
	var dividend model.DividendHistory
	err := GetDB().
		Where("stock_code = ? AND is_unassign = ?", code, false).
		Order("ex_dividend_date DESC").
		First(&dividend).Error
	return dividend, err
}

// ========== 名称变更 DAO ==========

// UpsertNameChange 单条名称变更 upsert (INSERT ON DUPLICATE KEY UPDATE)
func UpsertNameChange(m model.NameChange) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "stock_code"}, {Name: "change_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"security_name", "change_reason",
		}),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-fundamental] 名称变更upsert失败 [%s/%d]: %v", m.StockCode, m.ChangeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// FindNameChanges 查询名称变更记录
func FindNameChanges(code string, limit int) ([]model.NameChange, error) {
	var changes []model.NameChange
	q := GetDB().Where("stock_code = ?", code).Order("change_date DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&changes).Error
	return changes, err
}

// FindLatestNameChange 查询指定股票最近一次名称变更记录（按 change_date DESC）
// 若数据库中无记录，返回 gorm.ErrRecordNotFound
func FindLatestNameChange(code string) (model.NameChange, error) {
	var nc model.NameChange
	err := GetDB().
		Where("stock_code = ?", code).
		Order("change_date DESC").
		First(&nc).Error
	return nc, err
}
