package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsTradingSessionAt(t *testing.T) {
	tests := []struct {
		name string
		time string
		want bool
	}{
		{"开盘前", "2026-01-05 09:29", false},
		{"上午盘起点", "2026-01-05 09:30", true},
		{"上午盘中间", "2026-01-05 10:30", true},
		{"上午盘终点", "2026-01-05 11:30", true},
		{"午休", "2026-01-05 12:00", false},
		{"下午盘起点", "2026-01-05 13:00", true},
		{"下午盘中间", "2026-01-05 14:00", true},
		{"下午盘终点", "2026-01-05 15:00", true},
		{"收盘后", "2026-01-05 15:01", false},
		{"凌晨", "2026-01-05 02:00", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := time.ParseInLocation("2006-01-02 15:04", tt.time, time.Local)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, isTradingSessionAt(got))
		})
	}
}

func TestIsTradingSessionAt_IgnoresWeekend(t *testing.T) {
	// 周六交易时段内也返回 true：本函数只判时刻，不判交易日
	saturday, err := time.ParseInLocation("2006-01-02 15:04", "2026-01-03 10:30", time.Local)
	assert.NoError(t, err)
	assert.Equal(t, time.Saturday, saturday.Weekday())
	assert.True(t, isTradingSessionAt(saturday))
}

func TestIsTradingDayForDate_FallbackWithoutProvider(t *testing.T) {
	// 未注册 provider 时的降级模式：仅排除周末
	got, err := IsTradingDayForDate("2026-01-03") // 周六
	assert.NoError(t, err)
	assert.False(t, got)

	got, err = IsTradingDayForDate("2026-01-04") // 周日
	assert.NoError(t, err)
	assert.False(t, got)

	got, err = IsTradingDayForDate("2026-01-05") // 周一
	assert.NoError(t, err)
	assert.True(t, got)

	_, err = IsTradingDayForDate("2026/01/05")
	assert.Error(t, err)
}
