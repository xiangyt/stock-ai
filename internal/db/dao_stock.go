package db

import (
	"log"
	"time"

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

// LoadAllStockCodes 从数据库加载全量股票代码列表（排除已退市和B股：delist_date 为空字符串）
// B股代码规则：深B 200xxx，沪B 900xxx
func LoadAllStockCodes() []model.Stock {
	var stocks []model.Stock
	GetDB().
		Where("delist_date = ?", "").
		Where("code NOT LIKE '200%' AND code NOT LIKE '900%'").
		Find(&stocks)
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

// UpdateStockName 更新股票简称，同时根据名称变更更新退市日期
func UpdateStockName(code, name, delistDate string) error {
	updates := map[string]interface{}{
		"name":    name,
		"updated": time.Now(),
	}
	if delistDate != "" {
		updates["delist_date"] = delistDate
	}
	return GetDB().Model(&model.Stock{}).
		Where("code = ?", code).
		Updates(updates).Error
}

// LoadStockCodesByTradeDate 加载在指定交易日有日K数据的股票列表
// 通过 INNER JOIN daily_kline 过滤，确保选股范围内每只股票在当天都有交易数据
// 排除在交易日之前已退市的股票（退市日期 > 交易日 的仍保留）
// LoadStockCodesByTradeDate 按交易日加载当天有交易数据的股票列表。
// 使用 EXISTS 子查询替代 JOIN，利用 daily_kline 的主键 (stock_code, trade_date) 做 index-only scan。
func LoadStockCodesByTradeDate(tradeDate int) ([]model.Stock, error) {
	var stocks []model.Stock
	err := GetDB().
		Where("delist_date = ? OR delist_date > ?", "", FormatTradeDate(tradeDate)).
		Where("EXISTS (SELECT 1 FROM daily_kline WHERE stock_code = stocks.code AND trade_date = ?)", tradeDate).
		Find(&stocks).Error
	return stocks, err
}
