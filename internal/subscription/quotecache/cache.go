// Package quotecache provides a memory-based quote cache for A-share intraday data.
//
// It serves as the single source of truth for real-time market data within the system,
// analogous to an in-memory Redis. Two access patterns are supported:
//
//   - Active pull:  Get(ctx, code) → *CachedQuoteData
//   - Subscription: Subscribe(codes) → (<-chan QuoteEvent, func())
//
// The package has zero dependencies on other stock-ai packages. Its only external
// interface is IntradayCollector, which provides raw minute-bar data from data sources.
//
// Daily OHLCV is computed lazily from intraday bars. Weekly/Monthly/Yearly aggregation
// is not handled (TODO).
//
// Refresh strategy by priority:
//
//	High:   every 1 minute, full concurrency (held & monitored stocks)
//	Normal: every 5 minutes, semaphore(50) (searched & watched stocks)
//	Low:    10:30 / 12:00 / 14:00, semaphore(100) (indices & temp queries)
package quotecache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
//  Config
// ============================================================================

// Config 创建配置（零依赖，全部可选）。
type Config struct {
	// Collector 分时数据采集器链（必须）。
	Collector CollectorChain

	// OnQuoteReady 行情就绪回调（可选）。
	// 可用于上层注入 snapshot 计算、DB 写入等后处理逻辑。
	// 在 fetch 成功后、通知订阅者前调用。
	OnQuoteReady func(data *CachedQuoteData)

	// IsActive 是否活跃（可选）。
	// 返回 false 时暂停所有定时刷新。用于注入 utils.IsTradingHours 等判断。
	// 为 nil 时始终活跃。
	IsActive func() bool
}

// ============================================================================
//  QuoteCache 接口
// ============================================================================

// QuoteCache 行情缓存接口。
type QuoteCache interface {
	// Get 获取指定股票的缓存数据。缓存未命中时通过 Collector 获取。
	Get(ctx context.Context, code string) (*CachedQuoteData, error)

	// Subscribe 订阅行情推送。
	// 返回事件 channel 和取消函数。调用取消函数后 channel 被关闭，内部清理。
	Subscribe(codes []string) (<-chan QuoteEvent, func())

	// Subscriber 返回 QuoteSubscriber 函数（依赖注入用）。
	Subscriber() QuoteSubscriber

	// SetPriority 批量设置优先级。
	SetPriority(codes []string, p Priority)

	// Start 启动后台刷新。
	Start()

	// Stop 停止后台刷新。
	Stop()

	// Stats 返回缓存统计。
	Stats() Stats
}

// ============================================================================
//  内部类型
// ============================================================================

// cacheItem 缓存项
type cacheItem struct {
	data      *CachedQuoteData
	fetchedAt time.Time
	priority  Priority
}

// subEntry 订阅者
type subEntry struct {
	id    int
	codes []string
	ch    chan QuoteEvent
}

// ============================================================================
//  quoteCacheImpl
// ============================================================================

type quoteCacheImpl struct {
	collector    CollectorChain
	onQuoteReady func(*CachedQuoteData)
	isActive     func() bool

	mu       sync.RWMutex
	items    map[string]*cacheItem
	priority map[string]Priority

	// Subscription
	subMu      sync.RWMutex
	subByID    map[int]*subEntry
	codeToSubs map[string]map[int]chan QuoteEvent
	nextSubID  int

	// Lifecycle
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Stats
	hitCount  int64
	missCount int64
}

// New creates a QuoteCache instance.
func New(cfg Config) QuoteCache {
	if cfg.Collector == nil {
		panic("quotecache: Collector is required")
	}
	return &quoteCacheImpl{
		collector:    cfg.Collector,
		onQuoteReady: cfg.OnQuoteReady,
		isActive:     cfg.IsActive,
		items:        make(map[string]*cacheItem),
		priority:     make(map[string]Priority),
		subByID:      make(map[int]*subEntry),
		codeToSubs:   make(map[string]map[int]chan QuoteEvent),
		stopCh:       make(chan struct{}),
	}
}

// ============================================================================
//  Lifecycle
// ============================================================================

// Start 启动定时刷新 + 每日清理。
func (q *quoteCacheImpl) Start() {
	q.wg.Add(1)
	go q.scheduler()
}

// Stop 停止后台 goroutine。
func (q *quoteCacheImpl) Stop() {
	close(q.stopCh)
	q.wg.Wait()
}

// ============================================================================
//  Core: Get
// ============================================================================

// Get 获取缓存数据。
func (q *quoteCacheImpl) Get(ctx context.Context, code string) (*CachedQuoteData, error) {
	q.mu.RLock()
	item, ok := q.items[code]
	if ok && time.Since(item.fetchedAt) < time.Duration(item.priority.TTL())*time.Second {
		q.mu.RUnlock()
		atomic.AddInt64(&q.hitCount, 1)
		return item.data, nil
	}
	q.mu.RUnlock()

	atomic.AddInt64(&q.missCount, 1)
	return q.fetch(ctx, code)
}

// ============================================================================
//  Subscribe / Unsubscribe
// ============================================================================

// Subscribe 订阅行情推送。
func (q *quoteCacheImpl) Subscribe(codes []string) (<-chan QuoteEvent, func()) {
	q.subMu.Lock()
	defer q.subMu.Unlock()

	ch := make(chan QuoteEvent, 64)
	subID := q.nextSubID
	q.nextSubID++

	// 正向索引（管理用）
	q.subByID[subID] = &subEntry{
		id:    subID,
		codes: append([]string{}, codes...), // copy
		ch:    ch,
	}

	// 反向索引（推送用）
	for _, code := range codes {
		if q.codeToSubs[code] == nil {
			q.codeToSubs[code] = make(map[int]chan QuoteEvent)
		}
		q.codeToSubs[code][subID] = ch
	}

	// 返回取消函数（闭包捕获 subID 和 codes，接口不暴露 subID）
	return ch, func() {
		q.unsubscribe(subID, codes)
	}
}

// Subscriber 返回 QuoteSubscriber 函数（依赖注入用）。
func (q *quoteCacheImpl) Subscriber() QuoteSubscriber {
	return func(codes []string) (<-chan QuoteEvent, func()) {
		return q.Subscribe(codes)
	}
}

// unsubscribe 取消订阅（内部实现）。
func (q *quoteCacheImpl) unsubscribe(subID int, codes []string) {
	q.subMu.Lock()
	defer q.subMu.Unlock()

	// 清理反向索引
	for _, code := range codes {
		if subs, ok := q.codeToSubs[code]; ok {
			delete(subs, subID)
			if len(subs) == 0 {
				delete(q.codeToSubs, code)
			}
		}
	}

	// 关闭 channel + 删除正向索引
	if sub, ok := q.subByID[subID]; ok {
		close(sub.ch)
		delete(q.subByID, subID)
	}
}

// ============================================================================
//  Priority
// ============================================================================

// SetPriority 批量设置优先级。
func (q *quoteCacheImpl) SetPriority(codes []string, p Priority) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, code := range codes {
		q.priority[code] = p
		// 同步更新已有缓存项的优先级
		if item, ok := q.items[code]; ok {
			item.priority = p
		}
	}
}

// ============================================================================
//  Stats
// ============================================================================

// Stats 返回缓存统计。
func (q *quoteCacheImpl) Stats() Stats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var high, normal, low int
	for _, v := range q.priority {
		switch v {
		case PriorityHigh:
			high++
		case PriorityNormal:
			normal++
		case PriorityLow:
			low++
		}
	}

	return Stats{
		Size:        len(q.items),
		HighCount:   high,
		NormalCount: normal,
		LowCount:    low,
		HitCount:    atomic.LoadInt64(&q.hitCount),
		MissCount:   atomic.LoadInt64(&q.missCount),
	}
}

// ============================================================================
//  Internal: Fetch
// ============================================================================

// fetch 从 collector 获取数据并写入缓存。
func (q *quoteCacheImpl) fetch(ctx context.Context, code string) (*CachedQuoteData, error) {
	intraday, err := q.collector.Collect(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("quote fetch %s: %w", code, err)
	}

	data := &CachedQuoteData{Code: code, Intraday: intraday}
	if intraday.Name != "" {
		data.Name = intraday.Name
	}
	if q.onQuoteReady != nil {
		q.onQuoteReady(data)
	}

	pri := q.getPriority(code)

	q.mu.Lock()
	q.items[code] = &cacheItem{
		data:      data,
		fetchedAt: time.Now(),
		priority:  pri,
	}
	item := q.items[code]
	q.mu.Unlock()

	q.notifySubscribers(code, item.data)
	return item.data, nil
}

// ============================================================================
//  Internal: Refresh
// ============================================================================

// nextMinuteAlignment 计算到下一分钟第5秒的等待时间。
func nextMinuteAlignment(now time.Time) time.Duration {
	// 对齐到下一分钟的 :05 秒（如 10:00:05, 10:01:05）
	next := now.Truncate(time.Minute).Add(time.Minute + 5*time.Second)
	return next.Sub(now)
}

// scheduler 统一调度协程。每分钟第5秒触发，避免 ticker 零散偏移。
func (q *quoteCacheImpl) scheduler() {
	defer q.wg.Done()

	lowExecuted := make(map[int]bool)

	var timer *time.Timer
	for {
		now := time.Now()
		delay := nextMinuteAlignment(now)
		if timer == nil {
			timer = time.NewTimer(delay)
		} else {
			timer.Reset(delay)
		}

		select {
		case now = <-timer.C:
			if q.isActive != nil && !q.isActive() {
				// 非活跃时段：仅 9:00 重置标记 + 清理过期缓存
				if now.Hour() == 9 && now.Minute() == 0 {
					lowExecuted = make(map[int]bool)
					q.cleanStale()
				}
				continue
			}

			// 高优先级：每分钟全并发
			q.refreshPriority(PriorityHigh, 0)

			// 普通优先级：每 5 分钟 semaphore(50)
			if now.Minute()%5 == 0 {
				q.refreshPriority(PriorityNormal, 50)
			}

			// 低优先级：10:30 / 12:00 / 14:00 semaphore(100)
			minOfDay := now.Hour()*60 + now.Minute()
			for _, sched := range []int{10*60 + 30, 12 * 60, 14 * 60} {
				if minOfDay == sched && !lowExecuted[sched] {
					lowExecuted[sched] = true
					go q.refreshPriority(PriorityLow, 100)
				}
			}

			// 每日 9:00 重置低优先级标记
			if now.Hour() == 9 && now.Minute() == 0 {
				lowExecuted = make(map[int]bool)
			}
		case <-q.stopCh:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}

// refreshPriority 刷新指定优先级的所有股票。
// concurrency=0 表示全并发，>0 表示 semaphore 容量。
func (q *quoteCacheImpl) refreshPriority(p Priority, concurrency int) {
	codes := q.getCodesByPriority(p)
	if len(codes) == 0 {
		return
	}

	if concurrency == 0 {
		var wg sync.WaitGroup
		for _, code := range codes {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				q.refreshOne(c)
			}(code)
		}
		wg.Wait()
	} else {
		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup
		for _, code := range codes {
			wg.Add(1)
			sem <- struct{}{}
			go func(c string) {
				defer wg.Done()
				defer func() { <-sem }()
				q.refreshOne(c)
			}(code)
		}
		wg.Wait()
	}
}

// refreshOne 刷新单只股票。
func (q *quoteCacheImpl) refreshOne(code string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查 TTL
	q.mu.RLock()
	item, ok := q.items[code]
	q.mu.RUnlock()

	if ok && time.Since(item.fetchedAt) < time.Duration(item.priority.TTL())*time.Second {
		return
	}

	// 采集
	intraday, err := q.collector.Collect(ctx, code)
	if err != nil {
		return
	}

	// 更新缓存
	q.mu.Lock()
	if existing, ok := q.items[code]; ok {
		existing.data.Intraday = intraday
		existing.data.Invalidate()
		existing.fetchedAt = time.Now()
	} else {
		pri := q.getPriority(code)
		data := &CachedQuoteData{Code: code, Intraday: intraday}
		if intraday.Name != "" {
			data.Name = intraday.Name
		}
		if q.onQuoteReady != nil {
			q.onQuoteReady(data)
		}
		q.items[code] = &cacheItem{
			data:      data,
			fetchedAt: time.Now(),
			priority:  pri,
		}
	}
	// Re-read under lock for notify
	item, ok = q.items[code]
	q.mu.Unlock()

	if ok {
		q.notifySubscribers(code, item.data)
	}
}

// ============================================================================
//  Internal: Notification
// ============================================================================

// notifySubscribers 行情刷新后通知关注该 code 的订阅者。
//
// 通过反向索引 codeToSubs O(1) 定位，A/B 股票互不影响。
func (q *quoteCacheImpl) notifySubscribers(code string, data *CachedQuoteData) {
	q.subMu.RLock()
	subs, ok := q.codeToSubs[code]
	if !ok {
		q.subMu.RUnlock()
		return
	}
	q.subMu.RUnlock()

	evt := q.buildEvent(data)
	if evt == nil {
		return
	}

	q.subMu.RLock()
	defer q.subMu.RUnlock()
	subs, ok = q.codeToSubs[code]
	if !ok {
		return
	}
	for _, ch := range subs {
		select {
		case ch <- *evt:
		default:
		}
	}
}

// buildEvent 构建推送事件。
func (q *quoteCacheImpl) buildEvent(data *CachedQuoteData) *QuoteEvent {
	if data == nil {
		return nil
	}
	daily := data.Daily()
	if daily == nil {
		return nil
	}
	qd := &QuoteData{
		Code:      data.Code,
		Name:      data.Name,
		Price:     float64(daily.Close) / 100,
		ChangePct: daily.ChangePct,
		Volume:    daily.Volume,
		Turnover:  daily.Turnover,
	}
	if data.Intraday != nil {
		qd.Date = data.Intraday.Date
		qd.PreClose = float64(data.Intraday.PreClose) / 100
		for _, b := range data.Intraday.Bars {
			qd.Minutes = append(qd.Minutes, MinuteBarInfo{
				Time:  b.Time,
				Price: float64(b.Price) / 100,
			})
		}
		qd.Turnover = data.Intraday.Turnover
		qd.MarketCap = data.Intraday.MarketCap
		qd.FloatMarketCap = data.Intraday.FloatMarketCap
		qd.Pe = data.Intraday.Pe
		qd.Pb = data.Intraday.Pb
		qd.Amplitude = data.Intraday.Amplitude
		qd.Depth = data.Intraday.Depth
	}
	return &QuoteEvent{Code: data.Code, Data: qd}
}

// ============================================================================
//  Internal: Helpers
// ============================================================================

// getPriority 读取某股票的优先级（无锁版本）。
func (q *quoteCacheImpl) getPriority(code string) Priority {
	if p, ok := q.priority[code]; ok {
		return p
	}
	return PriorityNormal
}

// getCodesByPriority 返回指定优先级的所有股票代码。
func (q *quoteCacheImpl) getCodesByPriority(p Priority) []string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	codes := make([]string, 0)
	for code, pri := range q.priority {
		if pri == p {
			codes = append(codes, code)
		}
	}
	// Also include items with this priority even if not in priority map
	for code, item := range q.items {
		if item.priority == p {
			if _, inMap := q.priority[code]; !inMap {
				codes = append(codes, code)
			}
		}
	}
	return codes
}

// cleanStale 清理过期缓存（每日 9:00 调用）。
func (q *quoteCacheImpl) cleanStale() {
	q.mu.Lock()
	defer q.mu.Unlock()

	count := len(q.items)
	q.items = make(map[string]*cacheItem)
	if count > 0 {
		log.Printf("[quotecache] cleaned %d stale entries", count)
	}
}
