package holiday

import (
	"log"
	"sync"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// Provider 实现 utils.HolidayProvider，从数据库加载节假日数据
type Provider struct {
	mu       sync.RWMutex
	holidays map[string]bool // date string -> true（仅法定节假日，不含周末）
	loaded   bool
}

var defaultProvider = &Provider{
	holidays: make(map[string]bool),
}

// GetProvider 获取节假日数据提供者单例
func GetProvider() *Provider {
	return defaultProvider
}

// Load 从数据库加载节假日数据并注册到 utils
func (p *Provider) Load() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loaded {
		return nil
	}

	var holidays []model.TradingHoliday
	if err := db.GetDB().Find(&holidays).Error; err != nil {
		return err
	}

	for _, h := range holidays {
		p.holidays[h.HolidayDate] = true
	}
	p.loaded = true

	// 注册到 utils 包
	utils.RegisterHolidayProvider(p)

	log.Printf("✅ 节假日数据已加载: %d 条", len(holidays))
	return nil
}

// Reload 重新从数据库加载节假日（用于数据更新后）
func (p *Provider) Reload() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.holidays = make(map[string]bool)
	p.loaded = false

	var holidays []model.TradingHoliday
	if err := db.GetDB().Find(&holidays).Error; err != nil {
		return err
	}

	for _, h := range holidays {
		p.holidays[h.HolidayDate] = true
	}
	p.loaded = true

	utils.RegisterHolidayProvider(p)
	log.Printf("✅ 节假日数据已重新加载: %d 条", len(holidays))
	return nil
}

// IsHoliday 判断指定日期是否为法定节假日（不含周末判断）
func (p *Provider) IsHoliday(dateStr string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.holidays[dateStr]
}

// IsTradingDay 判断指定日期是否为 A 股交易日
func (p *Provider) IsTradingDay(t time.Time) bool {
	weekday := t.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	return !p.IsHoliday(t.Format("2006-01-02"))
}

// IsTradingHours 判断当前时间是否在 A 股交易时段（9:30-11:30, 13:00-15:00）
// 非交易日直接返回 false
func (p *Provider) IsTradingHours(t time.Time) bool {
	if !p.IsTradingDay(t) {
		return false
	}
	totalMinutes := t.Hour()*60 + t.Minute()
	// 上午盘 9:30-11:30
	if totalMinutes >= 9*60+30 && totalMinutes <= 11*60+30 {
		return true
	}
	// 下午盘 13:00-15:00
	return totalMinutes >= 13*60 && totalMinutes <= 15*60
}

// GetTradingDays 返回 [startDate, endDate] 区间内所有 A 股交易日
func (p *Provider) GetTradingDays(startDate, endDate string) ([]string, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	var days []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		wd := d.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			continue
		}
		dateStr := d.Format("2006-01-02")
		if p.holidays[dateStr] {
			continue
		}
		days = append(days, dateStr)
	}
	return days, nil
}

// AddTradingDays 从给定日期往后加 N 个交易日，返回新日期字符串
func (p *Provider) AddTradingDays(dateStr string, n int) (string, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	added := 0
	for added < n {
		t = t.AddDate(0, 0, 1)
		wd := t.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			continue
		}
		if p.holidays[t.Format("2006-01-02")] {
			continue
		}
		added++
	}
	return t.Format("2006-01-02"), nil
}
