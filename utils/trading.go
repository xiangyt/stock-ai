package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
//  交易日与交易时段判断
// ============================================================================

// HolidayProvider 节假日数据提供者接口
// 由 internal/holiday 包实现，main.go 中注册
type HolidayProvider interface {
	IsTradingDay(t time.Time) bool
	IsTradingHours(t time.Time) bool
	GetTradingDays(startDate, endDate string) ([]string, error)
	AddTradingDays(dateStr string, n int) (string, error)
}

var holidayProvider HolidayProvider

// RegisterHolidayProvider 注册节假日数据提供者
func RegisterHolidayProvider(p HolidayProvider) {
	holidayProvider = p
}

// IsTradingDay 判断今天是否交易日（排除周末 + 法定节假日）
func IsTradingDay() bool {
	if holidayProvider != nil {
		return holidayProvider.IsTradingDay(time.Now())
	}
	// 降级模式：仅排除周末
	wd := time.Now().Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

// IsTradingDayForDate 判断指定日期是否交易日
func IsTradingDayForDate(dateStr string) (bool, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false, fmt.Errorf("日期格式错误: %s", dateStr)
	}
	if holidayProvider != nil {
		return holidayProvider.IsTradingDay(t), nil
	}
	// 降级模式：仅排除周末
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday, nil
}

// IsTradingHours 判断当前时间是否在交易时段（9:30-11:30, 13:00-15:00）
func IsTradingHours() bool {
	if holidayProvider != nil {
		return holidayProvider.IsTradingHours(time.Now())
	}
	// 降级模式：基础交易时段检查
	totalMinutes := time.Now().Hour()*60 + time.Now().Minute()
	return (totalMinutes >= 9*60+30 && totalMinutes <= 11*60+30) ||
		(totalMinutes >= 13*60 && totalMinutes <= 15*60)
}

// IsPriceUpdateTime 判断当前是否在「可更新现价」时段：交易日 10:00~19:00
// 用于持仓管理获取现价的时机判断：
//   - 在时段内 → 从行情接口获取实时价
//   - 不在时段内 → 从数据库日K线取最新收盘价
func IsPriceUpdateTime() bool {
	// 1. 必须是交易日
	if !IsTradingDay() {
		return false
	}
	// 2. 时间在 10:00~19:00
	now := time.Now()
	h, m := now.Hour(), now.Minute()
	totalMinutes := h*60 + m
	return totalMinutes >= 10*60 && totalMinutes <= 19*60
}

// GetTradingDays 返回 [startDate, endDate] 区间内所有 A 股交易日
func GetTradingDays(startDate, endDate string) ([]string, error) {
	if holidayProvider != nil {
		return holidayProvider.GetTradingDays(startDate, endDate)
	}
	return nil, fmt.Errorf("节假日数据未初始化")
}

// AddTradingDays 从给定日期往后加 N 个交易日，返回新日期字符串
func AddTradingDays(dateStr string, n int) (string, error) {
	if holidayProvider != nil {
		return holidayProvider.AddTradingDays(dateStr, n)
	}
	return "", fmt.Errorf("节假日数据未初始化")
}

// ParseDateToTradeDate 将 "2026-04-12" 或 "20260412" 格式转为 20260412 整数
func ParseDateToTradeDate(dateStr string) (int, error) {
	clean := strings.ReplaceAll(dateStr, "-", "")
	if len(clean) != 8 {
		return 0, fmt.Errorf("日期格式错误: %s", dateStr)
	}
	return strconv.Atoi(clean)
}

// TodayTradeDate 返回今天的 YYYYMMDD 整数
func TodayTradeDate() int {
	now := time.Now()
	return now.Year()*10000 + int(now.Month())*100 + now.Day()
}
