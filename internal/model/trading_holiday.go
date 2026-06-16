package model

import "time"

// TradingHoliday A股法定节假日表
// 存储历年法定节假日日期（包含落在周末的节假日），
// 由 IsTradingDay() 在查询时结合周末判断来判定是否为交易日
type TradingHoliday struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	HolidayDate string    `gorm:"uniqueIndex;size:10;not null;comment:节假日日期 YYYY-MM-DD" json:"holiday_date"`
	HolidayName string    `gorm:"size:50;not null;comment:节假日名称" json:"holiday_name"`
	CreatedAt   time.Time `json:"created_at"`
}

func (TradingHoliday) TableName() string { return "trading_holidays" }
