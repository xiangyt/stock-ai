package db

import (
	"log"

	"stock-ai/internal/model"

	"gorm.io/gorm/clause"
)

// ========== 股票 DAO ==========

// UpsertStock 股票 upsert (INSERT ON DUPLICATE KEY UPDATE)
// 返回受影响的行数: 0=新增(INSERT), >0=更新(UPDATE), -1=错误
func UpsertStock(stock model.Stock) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "full_name", "english_name",
			"exchange", "exchange_name",
			"listing_board", "board_name",
			"list_date", "delist_date",
			"issue_price", "issue_pe", "issue_shares",
			"industry", "industry_code", "sector",
			"updated",
		}),
	}).Create(&stock)
	if result.Error != nil {
		log.Printf("[dao-stock] upsert失败 [%s]: %v", stock.Code, result.Error)
		return -1
	}
	return result.RowsAffected
}

// LoadAllStockCodes 从数据库加载全量股票代码列表
func LoadAllStockCodes() []model.Stock {
	var stocks []model.Stock
	GetDB().Find(&stocks)
	return stocks
}

// FindStockByCode 根据代码查询单只股票
func FindStockByCode(code string) (model.Stock, error) {
	var stock model.Stock
	err := GetDB().Where("code = ?", code).First(&stock).Error
	return stock, err
}

// CountStocks 统计股票总数
func CountStocks() int64 {
	var count int64
	GetDB().Model(&model.Stock{}).Count(&count)
	return count
}

// ListStocks 分页查询股票列表
func ListStocks(offset, limit int) ([]model.Stock, error) {
	var stocks []model.Stock
	err := GetDB().Offset(offset).Limit(limit).Find(&stocks).Error
	return stocks, err
}
