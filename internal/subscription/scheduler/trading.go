package scheduler

import (
	"time"
)

// ============================================================================
//  交易日与交易时段判断
// ============================================================================

// IsTradingDay 判断今天是否交易日（排除周末 + 法定节假日）
func IsTradingDay() bool {
	now := time.Now()
	weekday := now.Weekday()

	// 排除周末
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	// 排除法定节假日
	holidays := getHolidays()
	todayStr := now.Format("2006-01-02")
	for _, h := range holidays {
		if h == todayStr {
			return false
		}
	}

	return true
}

// IsTradingHours 判断当前时间是否在交易时段（9:15-11:30, 13:00-15:00）
func IsTradingHours() bool {
	now := time.Now()
	totalMinutes := now.Hour()*60 + now.Minute()

	// 上午盘 9:15-11:30
	if totalMinutes >= 9*60+15 && totalMinutes <= 11*60+30 {
		return true
	}
	// 下午盘 13:00-15:00
	if totalMinutes >= 13*60 && totalMinutes <= 15*60 {
		return true
	}
	return false
}

// getHolidays A 股法定节假日列表
func getHolidays() []string {
	return []string{
		// 2025 年
		// 元旦
		"2025-01-01",
		// 春节
		"2025-01-28", "2025-01-29", "2025-01-30", "2025-01-31",
		"2025-02-03", "2025-02-04",
		// 清明节
		"2025-04-04",
		// 劳动节
		"2025-05-01", "2025-05-02", "2025-05-05",
		// 端午节
		"2025-05-31", "2025-06-02",
		// 中秋节 + 国庆节
		"2025-10-01", "2025-10-02", "2025-10-03", "2025-10-06",
		"2025-10-07", "2025-10-08",

		// 2026 年
		// 元旦
		"2026-01-01", "2026-01-02",
		// 春节
		"2026-02-16", "2026-02-17", "2026-02-18",
		// 清明节
		"2026-04-05",
		// 劳动节
		"2026-05-01", "2026-05-02", "2026-05-03", "2026-05-04", "2026-05-05",
		// 端午节
		"2026-06-19",
		// 中秋节
		"2026-09-25",
		// 国庆节
		"2026-10-01", "2026-10-02", "2026-10-05", "2026-10-06", "2026-10-07",
	}
}
