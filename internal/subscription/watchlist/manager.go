// Package watchlist manages the centralized watchlist of stock codes that need
// high-priority quote cache refresh. All modules that affect which stocks a user
// cares about (portfolio, monitor config, strategy subscription) must sync their
// changes through this package. The manager then decides whether to set
// quotecache priorities.
//
//	Portfolio   ──→ Manager.OnPositionOpened/OnPositionClosed
//	Monitor     ──→ Manager.OnMonitorChanged
//	Subscription ──→ Manager.OnSubscriptionChanged
//	Daily task  ──→ Manager.ReloadAll
//
// Internal data model:
//   - Per-user maps track which stocks each user holds / monitors / subscribes.
//   - A refCount map aggregates how many users care about each stock across all
//     three categories. A stock gets PriorityHigh iff refCount > 0.
package watchlist

import (
	"encoding/json"
	"log"
	"sync"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/subscription/quotecache"
)

// Manager 用户关注股票代码管理器（单例）。
type Manager struct {
	cache quotecache.QuoteCache // 行情缓存（nil 时所有操作静默跳过）

	mu sync.RWMutex

	// 按用户 × 来源记录关注的股票代码
	userPositions     map[uint]map[string]bool // uid → 持仓股代码集合
	userMonitors      map[uint]map[string]bool // uid → 盯盘监控股票代码集合
	userSubscriptions map[uint]map[string]bool // uid → 策略订阅股票代码集合

	// 跨来源聚合引用计数：stockCode → 有多少用户关注
	refCount map[string]int
}

// NewManager 创建 Manager 实例。
func NewManager(cache quotecache.QuoteCache) *Manager {
	return &Manager{
		cache:             cache,
		userPositions:     make(map[uint]map[string]bool),
		userMonitors:      make(map[uint]map[string]bool),
		userSubscriptions: make(map[uint]map[string]bool),
		refCount:          make(map[string]int),
	}
}

// ============================================================================
//  持仓变化（增量更新单个股票）
// ============================================================================

// OnPositionOpened 建仓/加仓后调用。直接升为高优先级。
func (m *Manager) OnPositionOpened(uid uint, stockCode string) {
	if m.cache == nil || stockCode == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	set := m.ensureUserSet(m.userPositions, uid)
	if set[stockCode] {
		return // already tracked
	}
	set[stockCode] = true
	m.incRefLocked(stockCode)
}

// OnPositionClosed 清仓/删仓后调用。无人关注时降为低优先级。
func (m *Manager) OnPositionClosed(uid uint, stockCode string) {
	if m.cache == nil || stockCode == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	set := m.userPositions[uid]
	if set == nil || !set[stockCode] {
		return // not tracked
	}
	delete(set, stockCode)
	m.decRefLocked(stockCode)
}

// ============================================================================
//  Monitor / Subscription 变化（按用户增量更新）
// ============================================================================

// OnMonitorChanged 某用户的盯盘配置变化后调用。
// 重新从 DB 加载该用户的监控股票代码，计算增量差异并更新引用计数。
func (m *Manager) OnMonitorChanged(uid uint) {
	if m.cache == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.copySet(m.userMonitors[uid])
	new := m.loadUserMonitorCodes(uid)
	m.userMonitors[uid] = new
	m.diffUpdate(old, new)
}

// OnSubscriptionChanged 某用户的策略订阅变化后调用。
func (m *Manager) OnSubscriptionChanged(uid uint) {
	if m.cache == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.copySet(m.userSubscriptions[uid])
	new := m.loadUserSubscriptionCodes(uid)
	m.userSubscriptions[uid] = new
	m.diffUpdate(old, new)
}

// ============================================================================
//  每日全量重载（供数据采集任务调用）
// ============================================================================

// ReloadAll 从 DB 全量重建所有用户数据，重新计算所有优先级。
// 每日 8:00 由数据采集任务调用。
func (m *Manager) ReloadAll() {
	if m.cache == nil {
		log.Println("[watchlist] QuoteCache 未注入，跳过 ReloadAll")
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 保存当前已知用户 ID（清空前）
	posUsers := m.allTrackedUserIDs()

	m.cache.ResetPriorities()

	m.userPositions = make(map[uint]map[string]bool)
	m.userMonitors = make(map[uint]map[string]bool)
	m.userSubscriptions = make(map[uint]map[string]bool)
	m.refCount = make(map[string]int)

	// 加载全平台活跃 MonitorConfig 的股票代码
	monitorCfgs, err := db.GetAllActiveMonitorConfigs()
	if err != nil {
		log.Printf("[watchlist] ReloadAll: 加载 MonitorConfig 失败: %v", err)
	} else {
		for i := range monitorCfgs {
			cfg := &monitorCfgs[i]
			codes := resolveScopeCodes(cfg.Scope, cfg.UID, cfg.Stocks)
			set := m.ensureUserSet(m.userMonitors, cfg.UID)
			for _, c := range codes {
				set[c] = true
				m.incRefLocked(c)
			}
		}
	}

	// 加载全平台活跃 Subscription 的股票代码
	subs, err := db.GetActiveSubscriptions()
	if err != nil {
		log.Printf("[watchlist] ReloadAll: 加载 Subscription 失败: %v", err)
	} else {
		for i := range subs {
			sub := &subs[i]
			codes := resolveScopeCodes(sub.Scope, sub.UID, sub.CustomStocks)
			set := m.ensureUserSet(m.userSubscriptions, sub.UID)
			for _, c := range codes {
				set[c] = true
				m.incRefLocked(c)
			}
		}
	}

	// 持仓股：从已知用户 + Monitor/Subscription 的用户中加载
	userIDs := make(map[uint]bool)
	for uid := range posUsers {
		userIDs[uid] = true
	}
	for uid := range m.userMonitors {
		userIDs[uid] = true
	}
	for uid := range m.userSubscriptions {
		userIDs[uid] = true
	}

	for uid := range userIDs {
		codes, err := db.GetAllActiveStockCodes(uid)
		if err != nil {
			log.Printf("[watchlist] ReloadAll: 获取用户 %d 持仓失败: %v", uid, err)
			continue
		}
		set := m.ensureUserSet(m.userPositions, uid)
		for _, c := range codes {
			set[c] = true
			m.incRefLocked(c)
		}
	}

	// 批量设置高优先级
	highCodes := make([]string, 0, len(m.refCount))
	for code, cnt := range m.refCount {
		if cnt > 0 {
			highCodes = append(highCodes, code)
		}
	}
	m.cache.SetPriority(highCodes, quotecache.PriorityHigh)
	log.Printf("[watchlist] ReloadAll: %d users, %d high-priority codes", len(userIDs), len(highCodes))
}

// ============================================================================
//  内部：引用计数操作（调用方必须持有 mu 锁）
// ============================================================================

func (m *Manager) incRefLocked(code string) {
	if code == "" {
		return
	}
	old := m.refCount[code]
	m.refCount[code] = old + 1
	if old == 0 {
		m.cache.SetPriority([]string{code}, quotecache.PriorityHigh)
	}
}

func (m *Manager) decRefLocked(code string) {
	if code == "" {
		return
	}
	old := m.refCount[code]
	if old <= 1 {
		delete(m.refCount, code)
		m.cache.SetPriority([]string{code}, quotecache.PriorityLow)
	} else {
		m.refCount[code] = old - 1
	}
}

// diffUpdate 计算 old/new 两个代码集合的差异，增量更新引用计数。
func (m *Manager) diffUpdate(old, new map[string]bool) {
	// 移除的 → 减引用
	for code := range old {
		if !new[code] {
			m.decRefLocked(code)
		}
	}
	// 新增的 → 加引用
	for code := range new {
		if !old[code] {
			m.incRefLocked(code)
		}
	}
}

// ============================================================================
//  内部：辅助方法
// ============================================================================

func (m *Manager) ensureUserSet(dict map[uint]map[string]bool, uid uint) map[string]bool {
	set := dict[uid]
	if set == nil {
		set = make(map[string]bool)
		dict[uid] = set
	}
	return set
}

func (m *Manager) copySet(src map[string]bool) map[string]bool {
	if src == nil {
		return make(map[string]bool)
	}
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// allTrackedUserIDs 返回当前所有被追踪的用户 ID。
func (m *Manager) allTrackedUserIDs() map[uint]bool {
	ids := make(map[uint]bool)
	for uid := range m.userPositions {
		ids[uid] = true
	}
	for uid := range m.userMonitors {
		ids[uid] = true
	}
	for uid := range m.userSubscriptions {
		ids[uid] = true
	}
	return ids
}

// loadUserMonitorCodes 从 DB 加载单个用户的所有活跃 MonitorConfig 对应的股票代码集合。
func (m *Manager) loadUserMonitorCodes(uid uint) map[string]bool {
	// 全量加载所有活跃配置，过滤 uid
	cfgs, err := db.GetAllActiveMonitorConfigs()
	if err != nil {
		log.Printf("[watchlist] loadUserMonitorCodes uid=%d 失败: %v", uid, err)
		return make(map[string]bool)
	}
	set := make(map[string]bool)
	for i := range cfgs {
		cfg := &cfgs[i]
		if cfg.UID != uid {
			continue
		}
		for _, c := range resolveScopeCodes(cfg.Scope, cfg.UID, cfg.Stocks) {
			set[c] = true
		}
	}
	return set
}

// loadUserSubscriptionCodes 从 DB 加载单个用户的所有活跃 Subscription 对应的股票代码集合。
func (m *Manager) loadUserSubscriptionCodes(uid uint) map[string]bool {
	subs, err := db.GetActiveSubscriptions()
	if err != nil {
		log.Printf("[watchlist] loadUserSubscriptionCodes uid=%d 失败: %v", uid, err)
		return make(map[string]bool)
	}
	set := make(map[string]bool)
	for i := range subs {
		sub := &subs[i]
		if sub.UID != uid {
			continue
		}
		for _, c := range resolveScopeCodes(sub.Scope, sub.UID, sub.CustomStocks) {
			set[c] = true
		}
	}
	return set
}

// ============================================================================
//  工具函数
// ============================================================================

// resolveScopeCodes 按 scope 解析股票代码。
func resolveScopeCodes(scope model.MonitorScope, uid uint, stocksJSON string) []string {
	switch scope {
	case model.ScopeHeld:
		codes, err := db.GetAllActiveStockCodes(uid)
		if err != nil {
			log.Printf("[watchlist] 获取持仓股失败 uid=%d: %v", uid, err)
			return nil
		}
		return codes

	case model.ScopeCustom:
		var codes []string
		if stocksJSON != "" {
			if err := json.Unmarshal([]byte(stocksJSON), &codes); err != nil {
				log.Printf("[watchlist] 解析自定义股票失败: %v", err)
				return nil
			}
		}
		return codes

	default:
		// all / 其他 scope 不参与高频刷新
		return nil
	}
}
