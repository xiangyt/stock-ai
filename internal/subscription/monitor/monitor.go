package monitor

import (
	"context"
	"fmt"
	"log"
	"sync"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/notifier"
)

// ============================================================================
//  Monitor — 盯盘监控调度器（单例）
//
//  生命周期:
//    Start() → 加载活跃配置 → 每个 code 启动独立 goroutine
//    NotifyChange() → configLoop() 处理配置变更
//    Stop() → 停止所有 worker
//
//  架构: 每个 code 独立 channel + goroutine，并行处理行情事件。
// ============================================================================

// codeWorker 单个股票的处理协程
type codeWorker struct {
	code   string
	ch     <-chan model.QuoteEvent
	cancel func() // unsubscribe
}

// Monitor 盯盘监控调度器
type Monitor struct {
	subscribe model.QuoteSubscriber // 行情订阅函数（注入）
	ntf       notifier.Notifier

	// code → goroutine 管理
	workersMu sync.Mutex
	workers   map[string]*codeWorker

	// code → 监控该 code 的配置列表（O(1) 查找，processEvent 使用）
	codeConfigs   map[string][]*model.MonitorConfig
	codeConfigsMu sync.RWMutex

	// 动态配置
	configMu sync.RWMutex
	configs  map[uint]*model.MonitorConfig // configID → 配置
	changeCh chan ConfigChange             // 配置变更通道（cap=64）

	// 检查器
	checker  *AlertChecker
	cooldown *CooldownController
	push     *PushBuilder

	ctx    context.Context
	cancel context.CancelFunc
}

// NewMonitor 创建 Monitor 实例
func NewMonitor(subscribe model.QuoteSubscriber, ntf notifier.Notifier) *Monitor {
	return &Monitor{
		subscribe:    subscribe,
		ntf:          ntf,
		workers:      make(map[string]*codeWorker),
		codeConfigs:  make(map[string][]*model.MonitorConfig),
		configs:      make(map[uint]*model.MonitorConfig),
		changeCh:     make(chan ConfigChange, 64),
		checker:      NewAlertChecker(),
		cooldown:     NewCooldownController(),
		push:         NewPushBuilder(),
	}
}

// ============================================================================
//  生命周期
// ============================================================================

// Start 从 DB 加载活跃配置并启动监控
func (m *Monitor) Start() error {
	m.ctx, m.cancel = context.WithCancel(context.Background())

	// 1. 加载所有活跃配置
	cfgs, err := db.GetAllActiveMonitorConfigs()
	if err != nil {
		return fmt.Errorf("加载监控配置失败: %w", err)
	}
	m.configMu.Lock()
	for _, cfg := range cfgs {
		m.configs[cfg.ID] = &cfg
	}
	m.configMu.Unlock()
	log.Printf("[Monitor] 已加载 %d 条活跃监控配置", len(m.configs))

	// 2. 为每个 code 启动独立 worker
	m.syncWorkers(m.unionCodes())

	// 3. 启动配置变更监听 loop
	go m.configLoop()

	return nil
}

// Stop 停止所有 worker
func (m *Monitor) Stop() {
	m.workersMu.Lock()
	for _, w := range m.workers {
		w.cancel()
	}
	m.workers = make(map[string]*codeWorker)
	m.workersMu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}
}

// ============================================================================
//  公开接口
// ============================================================================

// NotifyChange 配置变更通知（由 MonitorConfigService CRUD 后调用，非阻塞）
func (m *Monitor) NotifyChange(change ConfigChange) {
	select {
	case m.changeCh <- change:
	default:
		log.Printf("[Monitor] 变更通道已满，丢弃事件: %+v", change)
	}
}

// NotifyHoldingChanged 持仓股变动通知（外部触发，如 portfolio CRUD 后调用）
func (m *Monitor) NotifyHoldingChanged() {
	m.NotifyChange(ConfigChange{Type: ChangeHoldingChanged, ConfigID: 0})
}

// ============================================================================
//  配置管理
// ============================================================================

// configLoop 处理配置变更事件
func (m *Monitor) configLoop() {
	for {
		select {
		case change := <-m.changeCh:
			m.handleConfigChange(change)
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Monitor) handleConfigChange(change ConfigChange) {
	switch change.Type {
	case ChangeCreated, ChangeEnabled, ChangeUpdated:
		cfg, err := db.GetMonitorConfigByIDForMonitor(change.ConfigID)
		if err != nil {
			log.Printf("[Monitor] 重新加载配置 %d 失败: %v", change.ConfigID, err)
			return
		}
		m.configMu.Lock()
		if cfg.IsActive {
			m.configs[cfg.ID] = cfg
		} else {
			delete(m.configs, cfg.ID)
		}
		m.configMu.Unlock()
		m.syncWorkers(m.unionCodes())

	case ChangeDeleted, ChangeDisabled:
		m.configMu.Lock()
		delete(m.configs, change.ConfigID)
		m.configMu.Unlock()
		m.syncWorkers(m.unionCodes())

	case ChangeHoldingChanged:
		// 持仓股变动时只重算 ScopeHeld 的 code 集合，config 不变
		m.syncWorkers(m.unionCodes())
	}
}

// syncWorkers 增量同步 worker + 重建 code→configs 索引
func (m *Monitor) syncWorkers(codes []string) {
	wanted := make(map[string]bool, len(codes))
	for _, c := range codes {
		wanted[c] = true
	}

	m.workersMu.Lock()
	defer m.workersMu.Unlock()

	// 1. 移除已不需要的 worker
	for code, w := range m.workers {
		if !wanted[code] {
			w.cancel()
			delete(m.workers, code)
		}
	}

	// 2. 新增缺少的 worker
	added := 0
	for code := range wanted {
		if _, exists := m.workers[code]; exists {
			continue
		}
		ch, unsub := m.subscribe([]string{code})
		w := &codeWorker{code: code, ch: ch, cancel: unsub}
		m.workers[code] = w
		go m.processCode(code, ch)
		added++
	}

	// 3. 重建 code→configs 索引（供 processEvent O(1) 查找）
	m.configMu.RLock()
	idx := make(map[string][]*model.MonitorConfig, len(codes))
	for _, cfg := range m.configs {
		for _, c := range m.resolveCodes(cfg) {
			idx[c] = append(idx[c], cfg)
		}
	}
	m.configMu.RUnlock()

	m.codeConfigsMu.Lock()
	m.codeConfigs = idx
	m.codeConfigsMu.Unlock()

	if added > 0 || len(m.workers) != len(codes) {
		log.Printf("[Monitor] Workers 同步: %d 活跃 (新增 %d)", len(m.workers), added)
	}
}

// unionCodes 计算所有活跃配置的股票代码并集
func (m *Monitor) unionCodes() []string {
	m.configMu.RLock()
	defer m.configMu.RUnlock()

	codeSet := make(map[string]struct{})
	for _, cfg := range m.configs {
		codes := m.resolveCodes(cfg)
		for _, c := range codes {
			codeSet[c] = struct{}{}
		}
	}

	result := make([]string, 0, len(codeSet))
	for c := range codeSet {
		result = append(result, c)
	}
	return result
}

// resolveCodes 解析监控配置的股票代码列表
func (m *Monitor) resolveCodes(cfg *model.MonitorConfig) []string {
	switch cfg.Scope {
	case model.ScopeHeld:
		codes, err := db.GetAllActiveStockCodes(cfg.UID)
		if err != nil {
			log.Printf("[Monitor] 获取持仓股失败 uid=%d: %v", cfg.UID, err)
			return nil
		}
		return codes

	case model.ScopeCustom:
		stocks, err := cfg.ParseStocks()
		if err != nil {
			log.Printf("[Monitor] 解析自选股失败 config=%d: %v", cfg.ID, err)
			return nil
		}
		return stocks
	}

	return nil
}

// ============================================================================
//  事件处理（每个 code 一个 goroutine）
// ============================================================================

// processCode 处理单个股票的所有行情事件
func (m *Monitor) processCode(code string, ch <-chan model.QuoteEvent) {
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			m.processEvent(event)
		case <-m.ctx.Done():
			return
		}
	}
}

// processEvent 处理单次行情推送事件
func (m *Monitor) processEvent(event model.QuoteEvent) {
	if event.Data == nil {
		return
	}

	// O(1) 从索引取该 code 的配置列表
	m.codeConfigsMu.RLock()
	configs := m.codeConfigs[event.Code]
	m.codeConfigsMu.RUnlock()

	for _, cfg := range configs {
		rule, err := cfg.ParseRule()
		if err != nil || rule == nil {
			continue
		}
		cd, err := cfg.ParseCooldown()
		if err != nil {
			continue
		}

		alerts := m.checker.Check(*rule, event.Data)
		for _, alert := range alerts {
			if !m.cooldown.ShouldAlert(cfg.ID, event.Code, alert.SubType, cd.IntervalMinutes, cd.DailyMax) {
				continue
			}

			msg := m.push.Build(cfg, alert, event.Data)

			bots, err := db.GetMonitorConfigBots(cfg.ID)
			if err != nil {
				log.Printf("[Monitor] 获取 Bot 失败 config=%d: %v", cfg.ID, err)
				continue
			}
			PushToBots(m.ntf, bots, msg)
		}
	}
}
