package db

import (
	"log"

	"stock-ai/internal/model"

	"gorm.io/gorm/clause"
)

// ========== K线 DAO ==========

// klineUpdateCols K线 upsert 更新列（4个周期共用）
var klineUpdateCols = []string{
	"open", "high", "low", "close",
	"volume", "amount", "turnover_rate",
}

// UpsertDailyKline 日K线 upsert (INSERT ON DUPLICATE KEY UPDATE)
// 返回受影响的行数: 0=新增(INSERT), >0=更新(UPDATE), -1=错误
func UpsertDailyKline(m model.DailyKline) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(klineUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-kline] 日K upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// UpsertWeeklyKline 周K线 upsert
func UpsertWeeklyKline(m model.WeeklyKline) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(klineUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-kline] 周K upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// UpsertMonthlyKline 月K线 upsert
func UpsertMonthlyKline(m model.MonthlyKline) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(klineUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-kline] 月K upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// UpsertYearlyKline 年K线 upsert
func UpsertYearlyKline(m model.YearlyKline) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(klineUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-kline] 年K upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// FindDailyKlines 查询日K线数据（按日期范围）
func FindDailyKlines(code string, startDate, endDate int, limit int) ([]model.DailyKline, error) {
	var klines []model.DailyKline
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
	err := q.Order("trade_date ASC").Find(&klines).Error
	return klines, err
}

// FindLatestDailyKline 查询最新一条日K线
func FindLatestDailyKline(code string) (model.DailyKline, error) {
	var kline model.DailyKline
	err := GetDB().Where("stock_code = ?", code).
		Order("trade_date DESC").
		First(&kline).Error
	return kline, err
}
