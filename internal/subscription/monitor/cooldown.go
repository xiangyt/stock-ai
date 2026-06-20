package monitor

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================================
//  CooldownController — 冷却控制器
//
//  每个告警 key = "configID:code:alertType"，独立冷却。
//  双规则：间隔冷却（分） + 日上限（次）。
// ============================================================================

// CooldownController 冷却控制器
type CooldownController struct {
	mu         sync.Mutex
	lastAlert  map[string]time.Time // key: "1:000001:surge_up" → 最后告警时间
	dailyCount map[string]int       // key: "1:000001:surge_up:2026-06-19" → 当日次数
	today      string               // 用于检测跨天
}

// NewCooldownController 创建冷却控制器
func NewCooldownController() *CooldownController {
	return &CooldownController{
		lastAlert:  make(map[string]time.Time),
		dailyCount: make(map[string]int),
		today:      time.Now().Format("2006-01-02"),
	}
}

// ShouldAlert 检查是否允许推送告警
//
// intervalMinutes: 同股同类型告警最小间隔（分钟）
// dailyMax: 每天同股同类型最多推送次数
func (c *CooldownController) ShouldAlert(configID uint, code, alertType string, intervalMinutes, dailyMax int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	today := now.Format("2006-01-02")

	// 跨天重置日计数
	if c.today != today {
		c.dailyCount = make(map[string]int)
		c.today = today
	}

	typeKey := fmt.Sprintf("%d:%s:%s", configID, code, alertType)
	dayKey := fmt.Sprintf("%s:%s", typeKey, today)

	// 间隔检查
	if last, ok := c.lastAlert[typeKey]; ok {
		if now.Sub(last) < time.Duration(intervalMinutes)*time.Minute {
			return false
		}
	}

	// 日上限检查
	if c.dailyCount[dayKey] >= dailyMax {
		return false
	}

	c.lastAlert[typeKey] = now
	c.dailyCount[dayKey]++
	return true
}

// ResetDaily 重置日计数器（0:00 调用）
func (c *CooldownController) ResetDaily() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dailyCount = make(map[string]int)
	c.today = time.Now().Format("2006-01-02")
}
