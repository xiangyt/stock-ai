package db

import (
	"fmt"
	"log"

	"stock-ai/internal/model"

	"gorm.io/gorm/clause"
)

// ========== K线周期常量 ==========

// KLinePeriod K线周期类型
type KLinePeriod string

const (
	KLinePeriodDaily   KLinePeriod = "daily"
	KLinePeriodWeekly  KLinePeriod = "weekly"
	KLinePeriodMonthly KLinePeriod = "monthly"
	KLinePeriodYearly  KLinePeriod = "yearly"
)

// klineTableName 周期→表名映射
func klineTableName(p KLinePeriod) string {
	switch p {
	case KLinePeriodDaily:
		return model.DailyKline{}.TableName()
	case KLinePeriodWeekly:
		return model.WeeklyKline{}.TableName()
	case KLinePeriodMonthly:
		return model.MonthlyKline{}.TableName()
	case KLinePeriodYearly:
		return model.YearlyKline{}.TableName()
	default:
		return string(p)
	}
}

// KLineLabel 周期→中文标签（导出，供 service/cmd 层使用）
func KLineLabel(p KLinePeriod) string {
	switch p {
	case KLinePeriodDaily:
		return "日K"
	case KLinePeriodWeekly:
		return "周K"
	case KLinePeriodMonthly:
		return "月K"
	case KLinePeriodYearly:
		return "年K"
	default:
		return string(p)
	}
}

// ========== 删除（用于 daily 模式同周期更新日期） ==========

// DeleteKlineByDate 按股票代码和交易日期删除一条 K 线记录
func DeleteKlineByDate(period KLinePeriod, code string, tradeDate int) error {
	table := klineTableName(period)
	result := GetDB().
		Table(table).
		Where("stock_code = ? AND trade_date = ?", code, tradeDate).
		Delete(nil)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("[dao-kline] 已删除 %s 记录 [%s/%d]", KLineLabel(period), code, tradeDate)
	}
	return nil
}

// ========== Upsert（4周期，保持原有接口不变） ==========

var klineUpdateCols = []string{
	"open", "high", "low", "close",
	"volume", "amount", "turnover_rate",
}

// klineOHLCVUpdateCols 仅更新 OHLCV 五字段，预留 amount/turnover_rate 不变。
// 用于除权除息日全量刷新历史价格，成交量，但保留原始成交额。
var klineOHLCVUpdateCols = []string{
	"open", "high", "low", "close", "volume",
}

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

// ========== Upsert OHLCV-only（除权除息模式专用：仅更新价格+成交量） ==========

func UpsertDailyKlineOHLCV(m model.DailyKline) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(klineOHLCVUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-kline] 日K(OHLCV) upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

func UpsertWeeklyKlineOHLCV(m model.WeeklyKline) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(klineOHLCVUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-kline] 周K(OHLCV) upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

func UpsertMonthlyKlineOHLCV(m model.MonthlyKline) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(klineOHLCVUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-kline] 月K(OHLCV) upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

func UpsertYearlyKlineOHLCV(m model.YearlyKline) int64 {
	result := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(klineOHLCVUpdateCols),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("[dao-kline] 年K(OHLCV) upsert失败 [%s/%d]: %v", m.StockCode, m.TradeDate, result.Error)
		return -1
	}
	return result.RowsAffected
}

// ========== 查询（通用多周期） ==========

// FindLatestKlineAny 查询指定周期最新一条记录（不要求 amount>0）
// 用于 init 模式判断增量起点 / daily 模式判断是否需要更新
// 无记录返回 gorm.ErrRecordNotFound
func FindLatestKlineAny(period KLinePeriod, code string) (tradeDate int, err error) {
	table := klineTableName(period)
	row := struct {
		TradeDate int `gorm:"column:trade_date"`
	}{}
	err = GetDB().
		Table(table).
		Select("trade_date").
		Where("stock_code = ?", code).
		Order("trade_date DESC").
		Limit(1).
		First(&row).Error
	return row.TradeDate, err
}

// FindLatestKlineWithAmount 查询指定周期成交金额不为0且交易日期最大的那条记录
// 用于 fill 模式判断是否还有缺金额的数据需要补全
// 无记录返回 gorm.ErrRecordNotFound
func FindLatestKlineWithAmount(period KLinePeriod, code string) (int, error) {
	table := klineTableName(period)
	row := struct {
		TradeDate int `gorm:"column:trade_date"`
	}{}
	err := GetDB().
		Table(table).
		Select("trade_date").
		Where("stock_code = ? AND amount > 0", code).
		Order("trade_date DESC").
		Limit(1).
		First(&row).Error
	return row.TradeDate, err
}

// CountZeroAmountKlines 统计指定周期 amount=0 的记录数
// 用于 fill 模式快速判断该股票是否还需要补数据
func CountZeroAmountKlines(period KLinePeriod, code string) (int64, error) {
	table := klineTableName(period)
	var count int64
	err := GetDB().
		Table(table).
		Where("stock_code = ? AND volume > 0 AND amount = 0", code).
		Count(&count).Error
	return count, err
}

// FindLatestNKlinesAny 查询指定周期最新 N 条记录的 trade_date（降序）
// 用于 daily 模式：取 DB 尾部窗口，和全量采集数据做对齐找锚定日期
// 返回值按 trade_date DESC 排序（最新的在前）
func FindLatestNKlinesAny(period KLinePeriod, code string, n int) ([]int, error) {
	table := klineTableName(period)
	if n <= 0 {
		n = 10
	}
	type dateRow struct {
		TradeDate int `gorm:"column:trade_date"`
	}
	var rows []dateRow
	err := GetDB().
		Table(table).
		Select("trade_date").
		Where("stock_code = ?", code).
		Order("trade_date DESC").
		Limit(n).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	dates := make([]int, len(rows))
	for i, r := range rows {
		dates[i] = r.TradeDate
	}
	return dates, nil
}

// ========== 查询（按周期，供 ScreenService 使用，直接调用 GetDB()） ==========

// FindDailyKlines 按 trade_date DESC 取最近 limit 条日K
func FindDailyKlines(code string, tradeDate, limit int) ([]*model.DailyKline, error) {
	var list []*model.DailyKline
	if limit <= 0 {
		limit = 250
	}
	db := GetDB().Where("stock_code = ?", code)
	if tradeDate > 0 {
		db = db.Where("trade_date <= ?", tradeDate)
	}
	err := db.Order("trade_date DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// FindWeeklyKlines 按 trade_date DESC 取最近 limit 条周K
func FindWeeklyKlines(code string, tradeDate, limit int) ([]*model.WeeklyKline, error) {
	var list []*model.WeeklyKline
	if limit <= 0 {
		limit = 250
	}
	db := GetDB().Where("stock_code = ?", code)
	if tradeDate > 0 {
		db = db.Where("trade_date <= ?", tradeDate)
	}
	err := db.Order("trade_date DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// FindMonthlyKlines 按 trade_date DESC 取最近 limit 条月K
func FindMonthlyKlines(code string, tradeDate, limit int) ([]*model.MonthlyKline, error) {
	var list []*model.MonthlyKline
	if limit <= 0 {
		limit = 250
	}
	db := GetDB().Where("stock_code = ?", code)
	if tradeDate > 0 {
		db = db.Where("trade_date <= ?", tradeDate)
	}
	err := db.Order("trade_date DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// FindYearlyKlines 按 trade_date DESC 取最近 limit 条年K
func FindYearlyKlines(code string, tradeDate, limit int) ([]*model.YearlyKline, error) {
	var list []*model.YearlyKline
	if limit <= 0 {
		limit = 250
	}
	db := GetDB().Where("stock_code = ?", code)
	if tradeDate > 0 {
		db = db.Where("trade_date <= ?", tradeDate)
	}
	err := db.Order("trade_date DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// DeleteKlinesAfterDate 删除指定股票某周期中 trade_date > anchorDate 的所有记录
// 用于 daily 模式：全量对齐后，截断脏数据/过期数据，再重新写入全量数据
// 返回删除的行数
func DeleteKlinesAfterDate(period KLinePeriod, code string, anchorDate int) (int64, error) {
	table := klineTableName(period)
	result := GetDB().
		Table(table).
		Where("stock_code = ? AND trade_date > ?", code, anchorDate).
		Delete(nil)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("[dao-kline] 截断 %s [%s]: 删除 trade_date>%d 共 %d 条",
			KLineLabel(period), code, anchorDate, result.RowsAffected)
	}
	return result.RowsAffected, nil
}

// IsSamePeriod 判断 dateA 和 dateB 是否属于同一个周期单位
// 例如：daily → 总是 false; weekly → 同一年同一周; monthly → 同年同月; yearly → 同年
func IsSamePeriod(period KLinePeriod, dateA, dateB int) bool {
	if dateA == 0 || dateB == 0 {
		return false
	}
	switch period {
	case KLinePeriodDaily:
		return dateA == dateB
	case KLinePeriodWeekly:
		// 周K：同一年 + ISO 周数相同 → 需要更新而非新增
		yearA, weekA := isoWeek(dateA)
		yearB, weekB := isoWeek(dateB)
		return yearA == yearB && weekA == weekB
	case KLinePeriodMonthly:
		// 月K：同年同月
		return dateA/100 == dateB/100
	case KLinePeriodYearly:
		// 年K：同年
		return dateA/10000 == dateB/10000
	default:
		return false
	}
}

// ========== 内部辅助函数 ==========

// isoWeek 从 YYYYMMDD 格式的日期计算 ISO 年份和周数
func isoWeek(date int) (year, week int) {
	d := date / 10000
	m := (date / 100) % 10
	day := date % 100

	// 简化的 ISO 周数计算：以 1月4日所在周为第1周的基准
	// 不引入 time 包依赖，用公式近似
	doy := dayOfYear(d, m, day)
	// 1月1日的星期几（2026-01-01 是周四=4），这里用固定基准
	// 更精确的做法是用 Zeller 公式或查表，但 Go 的标准做法是 time 包
	// 这里用简化版本：Jan 1 的 doy=1, Jan 4 的 doy=4
	jan1Weekday := weekdayOfJan1(d)
	week = (doy - jan1Weekday + 7) / 7
	if jan1Weekday > 4 { // 如果 Jan 1 是周五/六/日，则属于上一年
		week++
	}
	if week < 1 {
		week = 1
	} else if week > 52 {
		week = 52 // 简化处理
	}

	// ISO 年份可能和日历年不同（跨年边界时）
	year = d
	if week >= 52 && doy < 4 {
		year--
	} else if week == 1 && doy > 363 {
		year++
	}

	return
}

// dayOfYear 计算某天是该年的第几天
func dayOfYear(y, m, d int) int {
	daysPerMonth := []int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if isLeapYear(y) {
		daysPerMonth[2] = 29
	}
	doy := d
	for i := 1; i < m; i++ {
		doy += daysPerMonth[i]
	}
	return doy
}

// isLeapYear 判断闰年
func isLeapYear(y int) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}

// weekdayOfJan1 计算1月1日是星期几（0=周日, 1=周一, ..., 6=周六）
// 使用 Zeller 公式（格里高利历）
func weekdayOfJan1(year int) int {
	// Zeller 公式: h = (q + ⌊13(m+1)/5⌋ + K + ⌊K/4⌋ + ⌊J/4⌋ - 2J) mod 7
	// 对于 1月1日: q=1, m=13(视为上一年的13月), year'=year-1
	y := year - 1
	k := y % 100
	j := y / 100
	h := (1 + 26*14/5 + k + k/4 + j/4 + 5*j) % 7
	return h // 0=周六, 1=周日, ..., 5=周四, 6=周五
}

// FormatTradeDate 将 trade_date int 转为字符串 "YYYY-MM-DD"（供 service 层使用）
func FormatTradeDate(tradeDate int) string {
	return fmt.Sprintf("%04d-%02d-%02d",
		tradeDate/10000,
		(tradeDate%10000)/100,
		tradeDate%100)
}
