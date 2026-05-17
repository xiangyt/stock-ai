package subscription

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"stock-ai/internal/adapter"
)

// ============================================================================
//  QuoteCache 实时行情缓存接口 + 实现
// ============================================================================

// QuoteCache 实时行情缓存接口
type QuoteCache interface {
	// Get 获取指定股票的实时行情，缓存未命中时调用 adapter 获取
	Get(ctx context.Context, code string) (*adapter.StockPriceDaily, error)

	// Promote 将股票提升为高优先级（持仓股调用）
	Promote(code string)

	// Demote 将股票降级为普通优先级
	Demote(code string)

	// Refresh 手动触发刷新指定股票（忽略缓存 TTL）
	Refresh(ctx context.Context, code string) (*adapter.StockPriceDaily, error)

	// Start 启动后台刷新 worker
	Start()

	// Stop 停止后台刷新 worker
	Stop()

	// Stats 返回缓存统计信息（命中/未命中/大小等）
	Stats() CacheStats
}

// CacheStats 缓存统计
type CacheStats struct {
	Size        int   `json:"size"`
	HighCount   int   `json:"high_count"`
	NormalCount int   `json:"normal_count"`
	HitCount    int64 `json:"hit_count"`
	MissCount   int64 `json:"miss_count"`
}

// cacheItem 单个缓存项
type cacheItem struct {
	data      *adapter.StockPriceDaily
	fetchedAt time.Time
	priority  string // "high" 或 "normal"
}

// quoteCacheImpl QuoteCache 实现（双优先级 worker pool）
type quoteCacheImpl struct {
	mu         sync.RWMutex
	items      map[string]*cacheItem // code → 缓存项
	highSet    map[string]struct{}   // 高优先级集合
	highChan   chan string           // 高优先级刷新通道（无缓冲）
	normalChan chan string           // 普通优先级刷新通道（无缓冲）
	registry   *adapter.Registry    // 数据源注册中心
	highTicker *time.Ticker          // 3 分钟刷新高优先级
	normalTicker *time.Ticker        // 10 分钟刷新普通优先级
	highTTL    time.Duration         // 5 分钟
	normalTTL  time.Duration         // 15 分钟
	stopCh     chan struct{}
	wg         sync.WaitGroup
	hitCount   int64 // 原子统计
	missCount  int64 // 原子统计
}

// NewQuoteCache 创建行情缓存实例
// registry: 数据源注册中心
// workerCount: 后台 worker 数量，默认 3
func NewQuoteCache(reg *adapter.Registry, workerCount int) QuoteCache {
	if workerCount <= 0 {
		workerCount = 3
	}
	return &quoteCacheImpl{
		items:      make(map[string]*cacheItem),
		highSet:    make(map[string]struct{}),
		highChan:   make(chan string),
		normalChan: make(chan string),
		registry:   reg,
		highTTL:    5 * time.Minute,
		normalTTL:  15 * time.Minute,
		stopCh:     make(chan struct{}),
	}
}

// Start 启动后台刷新 worker + 两个 Ticker
func (q *quoteCacheImpl) Start() {
	// 启动 worker goroutine
	for i := 0; i < 3; i++ {
		q.wg.Add(1)
		go q.worker()
	}

	// 高优先级每 3 分钟刷新
	q.highTicker = time.NewTicker(3 * time.Minute)
	go func() {
		for {
			select {
			case <-q.highTicker.C:
				q.refreshByPriority("high")
			case <-q.stopCh:
				return
			}
		}
	}()

	// 普通优先级每 10 分钟刷新
	q.normalTicker = time.NewTicker(10 * time.Minute)
	go func() {
		for {
			select {
			case <-q.normalTicker.C:
				q.refreshByPriority("normal")
			case <-q.stopCh:
				return
			}
		}
	}()
}

// Stop 停止后台刷新 worker
func (q *quoteCacheImpl) Stop() {
	q.highTicker.Stop()
	q.normalTicker.Stop()
	close(q.stopCh)
	q.wg.Wait()
}

// Get 获取指定股票的实时行情
func (q *quoteCacheImpl) Get(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
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

// Refresh 手动触发刷新指定股票（忽略缓存 TTL）
func (q *quoteCacheImpl) Refresh(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return q.fetchAndCache(ctx, code)
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

// fetchAndCache 调用数据源获取行情并写入缓存
func (q *quoteCacheImpl) fetchAndCache(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	ds, err := q.getAdapter()
	if err != nil {
		return nil, err
	}

	data, err := ds.GetTodayData(ctx, code)
	if err != nil {
		return nil, err
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
