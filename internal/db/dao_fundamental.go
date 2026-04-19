package db

import (
	"log"

	"stock-ai/internal/model"

	"gorm.io/gorm/clause"
)

// ========== 基本面/财务面 DAO ==========

// UpsertPerformanceReport 单条财报 upsert (INSERT ON DUPLICATE KEY UPDATE)
func UpsertPerformanceReport(m model.PerformanceReport) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "stock_code"}, {Name: "report_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"report_type", "report_name", "currency",
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
		log.Printf("[dao-fundamental] 财报upsert失败 [%s/%s]: %v", m.StockCode, m.ReportDate, result.Error)
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
		log.Printf("[dao-fundamental] 股东户数upsert失败 [%s/%s]: %v", m.StockCode, m.EndDate, result.Error)
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
func FindPerformanceReports(code string, limit int) ([]model.PerformanceReport, error) {
	var reports []model.PerformanceReport
	q := GetDB().Where("stock_code = ?", code).Order("report_date DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&reports).Error
	return reports, err
}

// FindLatestShareholderCount 查询最新股东户数
func FindLatestShareholderCount(code string) (model.ShareholderCount, error) {
	var sc model.ShareholderCount
	err := GetDB().Where("stock_code = ?", code).
		Order("end_date DESC").
		First(&sc).Error
	return sc, err
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
