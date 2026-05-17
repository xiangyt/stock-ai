package quotecache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/utils"
)

// ============================================================================
//  QuoteCache 实时行情缓存接口
//
//  管理多周期行情数据的缓存生命周期，统一存储 adapter.StockPriceDaily。
//  外部通过 CachedStock 读取缓存数据并转换为指标引擎所需的格式。
// ============================================================================

// QuoteCache 实时行情缓存接口
type QuoteCache interface {
	// Get 获取指定股票的缓存数据（缓存未命中时调用 adapter 获取）
	Get(ctx context.Context, code string) (*CachedQuoteData, error)

	// Promote 将股票提升为高优先级（持仓股调用）
	Promote(code string)

	// Demote 将股票降级为普通优先级
	Demote(code string)

	// SetLoadHoldingCodes 注入加载持仓股 codes 的函数
	SetLoadHoldingCodes(fn func() ([]string, error))

	// Start 启动后台刷新 worker
	Start()

	// Stop 停止后台刷新 worker
	Stop()

	// Stats 返回缓存统计信息（命中/未命中/大小等）
	Stats() CacheStats
}

// CachedQuoteData 缓存的行情数据（日/周/月/年四周期）
type CachedQuoteData struct {
	Code    string
	Daily   *adapter.StockPriceDaily // 当日行情
	Weekly  *adapter.StockPriceDaily // 本周行情
	Monthly *adapter.StockPriceDaily // 本月行情
	Yearly  *adapter.StockPriceDaily // 本年行情
}

// CacheStats 缓存统计
type CacheStats struct {
	Size        int   `json:"size"`
	HighCount   int   `json:"high_count"`
	NormalCount int   `json:"normal_count"`
	HitCount    int64 `json:"hit_count"`
	MissCount   int64 `json:"miss_count"`
}

// ============================================================================
//  quoteCacheImpl 实现层（双优先级 worker pool）
// ============================================================================

// cacheItem 单个缓存项
type cacheItem struct {
	data      *CachedQuoteData
	fetchedAt time.Time
	priority  string // "high" 或 "normal"
}

// quoteCacheImpl QuoteCache 实现
type quoteCacheImpl struct {
	mu          sync.RWMutex
	items       map[string]*cacheItem // code → 缓存项
	highSet     map[string]struct{}   // 高优先级集合
	highChan    chan string           // 高优先级刷新通道（无缓冲）
	normalChan  chan string           // 普通优先级刷新通道（无缓冲）
	registry    *adapter.Registry     // 数据源注册中心
	workerCount int                   // 后台 worker 数量
	ticker      *time.Ticker          // 统一 1 分钟 tick
	highTTL     time.Duration         // 5 分钟
	normalTTL   time.Duration         // 15 分钟
	stopCh      chan struct{}
	wg          sync.WaitGroup
	hitCount    int64 // 原子统计
	missCount   int64 // 原子统计

	// 加载所有用户持仓股 codes 的函数（由外部注入，解耦 DB 依赖）
	loadHoldingCodes func() ([]string, error)
}

// NewQuoteCache 创建行情缓存实例
func NewQuoteCache(reg *adapter.Registry, workerCount int) QuoteCache {
	if workerCount <= 0 {
		workerCount = 3
	}
	return &quoteCacheImpl{
		items:       make(map[string]*cacheItem),
		highSet:     make(map[string]struct{}),
		highChan:    make(chan string),
		normalChan:  make(chan string),
		registry:    reg,
		workerCount: workerCount,
		highTTL:     5 * time.Minute,
		normalTTL:   15 * time.Minute,
		stopCh:      make(chan struct{}),
	}
}

// SetLoadHoldingCodes 注入加载持仓股 codes 的函数
func (q *quoteCacheImpl) SetLoadHoldingCodes(fn func() ([]string, error)) {
	q.loadHoldingCodes = fn
}

// Start 启动后台刷新 worker + 统一 1 分钟 Ticker
func (q *quoteCacheImpl) Start() {
	// 启动 worker pool
	for i := 0; i < q.workerCount; i++ {
		q.wg.Add(1)
		go q.worker()
	}

	// 加载所有用户持仓股 codes 并设为高优先级
	q.reloadHoldingCodes()

	// 统一 1 分钟 ticker
	q.ticker = time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-q.ticker.C:
				q.onTick()
			case <-q.stopCh:
				return
			}
		}
	}()
}

// Stop 停止后台刷新 worker
func (q *quoteCacheImpl) Stop() {
	q.ticker.Stop()
	close(q.stopCh)
	q.wg.Wait()
}

// onTick 每分钟触发，按条件分发刷新任务
func (q *quoteCacheImpl) onTick() {
	now := time.Now()

	// 交易日 9:00 整：重新加载所有用户持仓股 codes 并设为高优先级
	if utils.IsTradingDay() {
		if now.Hour() == 9 && now.Minute() == 0 {
			q.reloadHoldingCodes()
			return
		}
	} else {
		return
	}

	// 非交易时段，跳过
	if !utils.IsTradingHours() {
		return
	}

	// 分钟 % 3 == 0：刷新高优先级
	if now.Minute()%3 == 0 {
		go q.refreshByPriority("high")
	}

	// 分钟 % 10 == 0：刷新普通优先级
	if now.Minute()%10 == 0 {
		go q.refreshByPriority("normal")
	}
}

// reloadHoldingCodes 从 DB 加载所有用户持仓股 codes 并设为高优先级
func (q *quoteCacheImpl) reloadHoldingCodes() {
	if q.loadHoldingCodes == nil {
		return
	}
	codes, err := q.loadHoldingCodes()
	if err != nil {
		log.Printf("[QuoteCache] 加载持仓股 codes 失败: %v", err)
		return
	}
	if len(codes) == 0 {
		return
	}
	for _, code := range codes {
		q.Promote(code)
	}
	log.Printf("[QuoteCache] 已加载 %d 只持仓股为高优先级", len(codes))
}

// Get 获取指定股票的缓存行情数据（日/周/月/年四周期）
func (q *quoteCacheImpl) Get(ctx context.Context, code string) (*CachedQuoteData, error) {
	q.mu.RLock()
	item, ok := q.items[code]
	if ok && time.Since(item.fetchedAt) < q.getTTL(item.priority) {
		q.mu.RUnlock()
		atomic.AddInt64(&q.hitCount, 1)
		return item.data, nil
	}
	q.mu.RUnlock()

	atomic.AddInt64(&q.missCount, 1)
	return q.fetchAndCache(ctx, code)
}

// Promote 将股票提升为高优先级
func (q *quoteCacheImpl) Promote(code string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.highSet[code] = struct{}{}
	if item, ok := q.items[code]; ok {
		item.priority = "high"
	}
}

// Demote 将股票降级为普通优先级
func (q *quoteCacheImpl) Demote(code string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.highSet, code)
	if item, ok := q.items[code]; ok {
		item.priority = "normal"
	}
}

// Stats 返回缓存统计信息
func (q *quoteCacheImpl) Stats() CacheStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	highCount := len(q.highSet)
	normalCount := len(q.items) - highCount
	return CacheStats{
		Size:        len(q.items),
		HighCount:   highCount,
		NormalCount: normalCount,
		HitCount:    atomic.LoadInt64(&q.hitCount),
		MissCount:   atomic.LoadInt64(&q.missCount),
	}
}

// fetchAndCache 调用数据源获取日/周/月/年四周期行情并写入缓存
func (q *quoteCacheImpl) fetchAndCache(ctx context.Context, code string) (*CachedQuoteData, error) {
	ds, err := q.getAdapter()
	if err != nil {
		return nil, err
	}

	data := &CachedQuoteData{Code: code}

	// 并发获取四周期数据
	var wg sync.WaitGroup
	var daily, weekly, monthly, yearly *adapter.StockPriceDaily
	var dailyErr, weeklyErr, monthlyErr, yearlyErr error

	wg.Add(4)
	go func() {
		defer wg.Done()
		daily, dailyErr = ds.GetTodayData(ctx, code)
	}()
	go func() {
		defer wg.Done()
		weekly, weeklyErr = ds.GetThisWeekData(ctx, code)
	}()
	go func() {
		defer wg.Done()
		monthly, monthlyErr = ds.GetThisMonthData(ctx, code)
	}()
	go func() {
		defer wg.Done()
		yearly, yearlyErr = ds.GetThisYearData(ctx, code)
	}()
	wg.Wait()

	// 至少要有日数据才算成功
	if dailyErr != nil {
		return nil, fmt.Errorf("获取 %s 当日行情失败: %w", code, dailyErr)
	}

	data.Daily = daily
	if weeklyErr == nil {
		data.Weekly = weekly
	}
	if monthlyErr == nil {
		data.Monthly = monthly
	}
	if yearlyErr == nil {
		data.Yearly = yearly
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	priority := "normal"
	if _, isHigh := q.highSet[code]; isHigh {
		priority = "high"
	}

	q.items[code] = &cacheItem{
		data:      data,
		fetchedAt: time.Now(),
		priority:  priority,
	}

	return data, nil
}

// getAdapter 从注册中心获取可用数据源
func (q *quoteCacheImpl) getAdapter() (adapter.DataSource, error) {
	for _, name := range q.registry.Names() {
		if ds, ok := q.registry.Get(name); ok {
			return ds, nil
		}
	}
	return nil, fmt.Errorf("无可用数据源")
}

// worker 后台刷新 worker goroutine
func (q *quoteCacheImpl) worker() {
	defer q.wg.Done()
	for {
		select {
		case code, ok := <-q.highChan:
			if !ok {
				return
			}
			q.refreshCode(code)
		case code, ok := <-q.normalChan:
			if !ok {
				return
			}
			q.refreshCode(code)
		case <-q.stopCh:
			return
		}
	}
}

// refreshCode 刷新单个股票代码的缓存
func (q *quoteCacheImpl) refreshCode(code string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	q.mu.RLock()
	item, ok := q.items[code]
	ttl := q.normalTTL
	if ok {
		ttl = q.getTTL(item.priority)
	}
	q.mu.RUnlock()

	// 未过期则跳过
	if ok && time.Since(item.fetchedAt) < ttl {
		return
	}

	_, err := q.fetchAndCache(ctx, code)
	if err != nil {
		// 刷新失败不阻塞，等待下次 ticker 触发
	}
}

// refreshByPriority 按优先级刷新所有对应股票
func (q *quoteCacheImpl) refreshByPriority(priority string) {
	q.mu.RLock()
	var codes []string
	if priority == "high" {
		codes = make([]string, 0, len(q.highSet))
		for code := range q.highSet {
			codes = append(codes, code)
		}
	} else {
		codes = make([]string, 0, len(q.items))
		for code := range q.items {
			if _, isHigh := q.highSet[code]; !isHigh {
				codes = append(codes, code)
			}
		}
	}
	q.mu.RUnlock()

	ch := q.highChan
	if priority == "normal" {
		ch = q.normalChan
	}

	for _, code := range codes {
		select {
		case ch <- code:
		case <-q.stopCh:
			return
		}
	}
}

// getTTL 根据优先级返回 TTL
func (q *quoteCacheImpl) getTTL(priority string) time.Duration {
	if priority == "high" {
		return q.highTTL
	}
	return q.normalTTL
}
