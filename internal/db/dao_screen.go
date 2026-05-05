package db

import (
	"stock-ai/internal/model"
)

// ========== 选股引擎 — 数据查询 DAO ==========
// 供 ScreenService 调用，每个函数对应 StockData 的一个字段。

// GetPerformanceReports 按公告时间(notice_date) DESC 取最近 limit 条财报
func GetPerformanceReports(code string, noticeDate, limit int) ([]*model.PerformanceReport, error) {
	var list []*model.PerformanceReport
	db := GetDB().Where("stock_code = ?", code)
	if noticeDate > 0 {
		db = db.Where("notice_date <= ?", noticeDate)
	}
	if err := db.Order("notice_date DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
