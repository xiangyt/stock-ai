package quotecache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
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

	// SetLoadHoldingCodes 注入加载持仓股 codes 的函数
	SetLoadHoldingCodes(fn func() ([]string, error))

	// ReloadHighPriority 手动触发高优先级行情重载（外部调用）
	ReloadHighPriority()

	// Start 启动后台刷新协程
	Start()

	// Stop 停止后台刷新协程
	Stop()

	// Stats 返回缓存统计信息（命中/未命中/大小等）
	Stats() CacheStats

	// QuoteEventBus 行情事件推送
	QuoteEventBus

	// Subscriber 返回 model.QuoteSubscriber（Monitor 依赖注入用）
	Subscriber() model.QuoteSubscriber
}

// CachedQuoteData 缓存的行情数据（日/周/月/年四周期 + 快照 + 分时）
type CachedQuoteData struct {
	Code     string                    // 股票代码
	Name     string                    // 股票名称（TODO: 从 adapter 获取）
	Daily    *adapter.StockPriceDaily  // 当日行情
	Weekly   *adapter.StockPriceDaily  // 本周行情
	Monthly  *adapter.StockPriceDaily  // 本月行情
	Yearly   *adapter.StockPriceDaily  // 本年行情
	Minute   *MinuteData               // 分时行情（TODO: adapter 扩展后填充）
	Snapshot *model.StockDailySnapshot // 完整快照（市值/PE/PB/ROE 等 20+ 字段）
}

// ============================================================================
//  QuoteEventBus — 行情事件推送
//
//  内部使用 model.QuoteEvent（而非自定义 QuoteEvent），
//  在 notifySubscribers 中直接完成 CachedQuoteData → QuoteData 转换，
//  消费者无需再做二次转换。
// ============================================================================

// QuoteEventBus 行情事件总线接口
//
// 消费方通过此接口订阅感兴趣的股票代码，
// 当 quotecache 刷新行情数据后通过 channel 推送更新。
//
// 当前实现：quotecache 内存版
// 未来可替换：Redis Pub/Sub 等分布式实现
type QuoteEventBus interface {
	// Subscribe 订阅指定股票代码的行情更新
	// 返回带缓冲的 channel（cap=64）和订阅 ID
	Subscribe(codes []string) (<-chan model.QuoteEvent, int)

	// Unsubscribe 取消订阅
	Unsubscribe(subID int)
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
//  默认配置常量
// ============================================================================

// defaultAdapterPriority 数据源优先级列表（从高到低）
var defaultAdapterPriority = []string{
	"tencentstock", // 腾讯源（最高优先）
	"ths2",         // 同花顺v2（quota-h 新接口）
	"ths",          // 同花顺
	"eastmoney",    // 东方财富
}

// lowPrioritySchedule 低优先级定时执行时间（交易日 10:30、12:00、14:00）
var lowPrioritySchedule = []struct {
	hour   int
	minute int
}{
	{10, 30},
	{12, 0},
	{14, 0},
}

// lowPriorityConcurrency 低优先级并发度（semaphore 容量）
const lowPriorityConcurrency = 100

// highTTL / normalTTL 缓存有效期
const (
	highTTL   = 5 * time.Minute
	normalTTL = 15 * time.Minute
)

// ============================================================================
//  quoteCacheImpl 实现层（双协程刷新策略）
// ============================================================================

// cacheItem 单个缓存项
type cacheItem struct {
	data      *CachedQuoteData
	fetchedAt time.Time
	priority  string // "high" 或 "normal"
}

// subEntry 单个订阅者
type subEntry struct {
	id    int
	codes map[string]struct{} // 关注的股票代码集合
	ch    chan model.QuoteEvent // 推送 channel（缓冲 64）
}

// quoteCacheImpl QuoteCache 实现
type quoteCacheImpl struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem // code → 缓存项
	highSet map[string]struct{}   // 高优先级集合

	// 订阅推送
	subMu       sync.RWMutex
	subscribers []*subEntry
	nextSubID   int

	registry        *adapter.Registry // 数据源注册中心
	adapterPriority []string          // 数据源优先级列表

	stopCh chan struct{}
	wg     sync.WaitGroup

	hitCount  int64 // 原子统计
	missCount int64 // 原子统计

	// 加载所有用户持仓股 codes 的函数（由外部注入，解耦 DB 依赖）
	loadHoldingCodes func() ([]string, error)
}

// NewQuoteCache 创建行情缓存实例
func NewQuoteCache(reg *adapter.Registry) QuoteCache {
	priority := make([]string, len(defaultAdapterPriority))
	copy(priority, defaultAdapterPriority)

	return &quoteCacheImpl{
		items:           make(map[string]*cacheItem),
		highSet:         make(map[string]struct{}),
		registry:        reg,
		adapterPriority: priority,
		stopCh:          make(chan struct{}),
	}
}

// SetLoadHoldingCodes 注入加载持仓股 codes 的函数
func (q *quoteCacheImpl) SetLoadHoldingCodes(fn func() ([]string, error)) {
	q.loadHoldingCodes = fn
}

// Start 启动双协程后台刷新 + 初始加载持仓股
func (q *quoteCacheImpl) Start() {
	// 初始加载持仓股并清理昨日缓存
	q.reloadHoldingCodes()

	// 启动高优先级刷新协程（每分钟 tick，交易时段 + 5的倍数分钟触发）
	q.wg.Add(1)
	go q.highPriorityWorker()

	// 启动低优先级刷新协程（定时 3 次，100 并发）
	q.wg.Add(1)
	go q.lowPriorityWorker()

	// 启动定时清理协程：9:00 整 + 19:00 整清理缓存
	q.wg.Add(1)
	go q.scheduleCleanupWorker()
}

// Stop 停止后台刷新协程
func (q *quoteCacheImpl) Stop() {
	close(q.stopCh)
	q.wg.Wait()
}

// ============================================================================
//  高优先级 Worker：每 5 分钟全并发刷新持仓股
// ============================================================================

func (q *quoteCacheImpl) highPriorityWorker() {
	defer q.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			if !utils.IsTradingHours() {
				continue
			}
			if now.Minute()%5 != 0 {
				continue
			}
			q.refreshHighPriority()
		case <-q.stopCh:
			return
		}
	}
}

// refreshHighPriority 全并发刷新所有高优先级股票
func (q *quoteCacheImpl) refreshHighPriority() {
	codes := q.getHighPriorityCodes()
	if len(codes) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, code := range codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			q.refreshCode(c)
		}(code)
	}
	wg.Wait()
}

// ============================================================================
//  低优先级 Worker：交易时段定时执行，100 并发度
// ============================================================================

func (q *quoteCacheImpl) lowPriorityWorker() {
	defer q.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// 记录当天已执行的调度点，避免重复执行
	executed := make(map[int]bool)

	for {
		select {
		case <-ticker.C:
			if !utils.IsTradingDay() || !utils.IsTradingHours() {
				continue
			}

			now := time.Now()
			key := now.Hour()*60 + now.Minute()

			// 检查当前时间是否匹配某个调度点且未执行过
			for _, sched := range lowPrioritySchedule {
				schedKey := sched.hour*60 + sched.minute
				if key == schedKey && !executed[schedKey] {
					executed[schedKey] = true
					go q.refreshLowPriority()
				}
			}

			// 每天 9:00 重置已执行标记（配合 reloadHoldingCodes 清理缓存）
			if now.Hour() == 9 && now.Minute() == 0 {
				executed = make(map[int]bool) // 新 map，旧 GC 回收
			}

		case <-q.stopCh:
			return
		}
	}
}

// refreshLowPriority 以 100 并发度分批刷新普通优先级股票
func (q *quoteCacheImpl) refreshLowPriority() {
	codes := q.getNormalPriorityCodes()
	if len(codes) == 0 {
		return
	}

	utils.ConcurrentExec(codes, lowPriorityConcurrency, func(_ int, code string) error {
		q.refreshCode(code)
		return nil
	})
}

// scheduleCleanupWorker 定时协程：每天 9:00 清理缓存 + 19:00 从 DB 刷新日K 并计算快照
func (q *quoteCacheImpl) scheduleCleanupWorker() {
	defer q.wg.Done()

	for {
		now := time.Now()

		// 找下一个最近的 9:00 或 19:00
		nineAM := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
		sevenPM := time.Date(now.Year(), now.Month(), now.Day(), 19, 0, 0, 0, now.Location())

		var next time.Time
		switch {
		case now.Before(nineAM):
			next = nineAM
		case now.Before(sevenPM):
			next = sevenPM
		default:
			next = nineAM.Add(24 * time.Hour)
		}

		select {
		case <-time.After(time.Until(next)):
			h := next.Hour()
			switch h {
			case 9:
				log.Printf("[QuoteCache] 定时清理缓存 (09:00)")
				q.cleanStaleCache()
			case 19:
				log.Printf("[QuoteCache] 定时 DB 日K 刷新 (19:00)")
				q.refreshAllFromDB()
			}
		case <-q.stopCh:
			return
		}
	}
}

// ============================================================================
//  核心方法：Get / Promote / ReloadHighPriority / Stats
// ============================================================================

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

// ReloadHighPriority 手动触发高优先级行情重载
func (q *quoteCacheImpl) ReloadHighPriority() {
	log.Printf("[QuoteCache] 收到手动重载请求，开始刷新 %d 只高优先级股票", len(q.highSet))
	go q.refreshHighPriority()
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

// ============================================================================
//  QuoteEventBus 实现：Subscribe / Unsubscribe
// ============================================================================

// Subscribe 订阅指定股票代码的行情更新
func (q *quoteCacheImpl) Subscribe(codes []string) (<-chan model.QuoteEvent, int) {
	q.subMu.Lock()
	defer q.subMu.Unlock()

	ch := make(chan model.QuoteEvent, 64)
	codeSet := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		codeSet[c] = struct{}{}
	}

	subID := q.nextSubID
	q.nextSubID++
	q.subscribers = append(q.subscribers, &subEntry{
		id:    subID,
		codes: codeSet,
		ch:    ch,
	})

	return ch, subID
}

// Unsubscribe 取消订阅
func (q *quoteCacheImpl) Unsubscribe(subID int) {
	q.subMu.Lock()
	defer q.subMu.Unlock()

	for i, sub := range q.subscribers {
		if sub.id == subID {
			close(sub.ch)
			q.subscribers = append(q.subscribers[:i], q.subscribers[i+1:]...)
			return
		}
	}
}

// notifySubscribers 行情刷新后通知所有匹配的订阅者
// 在此直接完成 CachedQuoteData → model.QuoteData 转换，消费者无需二次转换
func (q *quoteCacheImpl) notifySubscribers(code string, data *CachedQuoteData) {
	q.subMu.RLock()
	defer q.subMu.RUnlock()

	qd := toQuoteData(data)
	if qd == nil {
		return
	}
	event := model.QuoteEvent{Code: code, Data: qd}
	for _, sub := range q.subscribers {
		if _, ok := sub.codes[code]; ok {
			select {
			case sub.ch <- event:
			default:
				// channel 满（消费方慢），丢弃事件，不阻塞刷新 loop
			}
		}
	}
}

// reloadHoldingCodes 从 DB 加载所有用户持仓股 codes 并设为高优先级
// 同时清理昨日的所有行情缓存和优先级缓存
func (q *quoteCacheImpl) reloadHoldingCodes() {
	// 清理所有昨日缓存和优先级
	q.cleanStaleCache()

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
	q.mu.Lock()
	for _, code := range codes {
		q.highSet[code] = struct{}{}
	}
	q.mu.Unlock()
	log.Printf("[QuoteCache] 已加载 %d 只持仓股为高优先级", len(codes))
}

// cleanStaleCache 清理昨日的所有行情缓存和高优先级集合
// 在每个交易日 9:00 整 和启动时调用
func (q *quoteCacheImpl) cleanStaleCache() {
	q.mu.Lock()
	defer q.mu.Unlock()

	prevSize := len(q.items)
	q.items = make(map[string]*cacheItem)
	q.highSet = make(map[string]struct{})
	if prevSize > 0 {
		log.Printf("[QuoteCache] 已清理 %d 条昨日缓存", prevSize)
	}
}

// ============================================================================
//  数据获取与缓存
// ============================================================================

// fetchAndCache 调用数据源获取日/周/月/年四周期行情并写入缓存
func (q *quoteCacheImpl) fetchAndCache(ctx context.Context, code string) (*CachedQuoteData, error) {
	ds, err := q.getAdapter()
	if err != nil {
		return nil, err
	}

	daily, err := ds.GetTodayData(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取 %s 当日行情失败: %w", code, err)
	}

	data := &CachedQuoteData{Code: code, Daily: daily}

	// 获取行情成功后立即构建完整快照（DB 财报 + 股本 → 市值/PE/PB/ROE 等 20+ 字段）
	if snap, snapErr := datacollect.BuildDailySnapshot(code, daily); snapErr == nil {
		data.Snapshot = snap
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

	// 通知订阅者（异步写入 channel，消费方慢时不阻塞）
	q.notifySubscribers(code, data)

	return data, nil
}

// getAdapter 按优先级列表顺序查找可用数据源
func (q *quoteCacheImpl) getAdapter() (adapter.DataSource, error) {
	for _, name := range q.adapterPriority {
		if ds, ok := q.registry.Get(name); ok {
			return ds, nil
		}
	}
	// 兜底：遍历注册中心全部数据源
	for _, name := range q.registry.Names() {
		if ds, ok := q.registry.Get(name); ok {
			return ds, nil
		}
	}
	return nil, fmt.Errorf("无可用数据源")
}

// refreshCode 刷新单个股票代码的缓存
func (q *quoteCacheImpl) refreshCode(code string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	q.mu.RLock()
	item, ok := q.items[code]
	ttl := normalTTL
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
		// 刷新失败不阻塞，等待下次触发
	}
}

// ============================================================================
//  辅助方法
// ============================================================================

// getHighPriorityCodes 返回当前所有高优先级股票代码的快照
func (q *quoteCacheImpl) getHighPriorityCodes() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	codes := make([]string, 0, len(q.highSet))
	for code := range q.highSet {
		codes = append(codes, code)
	}
	return codes
}

// getNormalPriorityCodes 返回当前所有普通优先级股票代码的快照
func (q *quoteCacheImpl) getNormalPriorityCodes() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	codes := make([]string, 0, len(q.items)-len(q.highSet))
	for code := range q.items {
		if _, isHigh := q.highSet[code]; !isHigh {
			codes = append(codes, code)
		}
	}
	return codes
}

// getTTL 根据优先级返回 TTL
func (q *quoteCacheImpl) getTTL(priority string) time.Duration {
	if priority == "high" {
		return highTTL
	}
	return normalTTL
}

// ============================================================================
//  19:00 盘后 DB 刷新：从数据库取最新日K 计算快照（不走 adapter）
// ============================================================================

// refreshAllFromDB 对所有已缓存股票（高+低优先级）执行盘后 DB 刷新
func (q *quoteCacheImpl) refreshAllFromDB() {
	codes := q.getAllCachedCodes()
	if len(codes) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, code := range codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			q.refreshCodeFromDB(c)
		}(code)
	}
	wg.Wait()
	log.Printf("[QuoteCache] 19:00 DB 日K 刷新完成，共 %d 只", len(codes))
}

// refreshCodeFromDB 从数据库读取最新一条日K线，构建快照并更新缓存
//
// 与 fetchAndCache 的区别：
//   - 不调用 adapter 数据源，直接读 DB 最新日K
//   - 仅更新 Daily 和 Snapshot（周/月/年不更新）
//   - 用于 19:00 盘后数据确认，确保快照基于收盘价计算
func (q *quoteCacheImpl) refreshCodeFromDB(code string) {
	klines, err := db.FindDailyKlines(code, 0, 1)
	if err != nil || len(klines) == 0 {
		return // 无日K数据，跳过
	}

	k := klines[0]
	price := &adapter.StockPriceDaily{
		Code:      k.StockCode,
		Date:      db.FormatTradeDate(k.TradeDate),
		Open:      int64(k.Open),
		High:      int64(k.High),
		Low:       int64(k.Low),
		Close:     int64(k.Close),
		Volume:    k.Volume,
		Amount:    k.Amount,
		Turnover:  k.TurnoverRate,
	}

	var snap *model.StockDailySnapshot
	if s, snapErr := datacollect.BuildDailySnapshot(code, price); snapErr == nil {
		snap = s
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// 更新已有缓存项，或新建
	if item, ok := q.items[code]; ok {
		item.data.Daily = price
		item.data.Snapshot = snap
		item.fetchedAt = time.Now()
	} else {
		priority := "normal"
		if _, isHigh := q.highSet[code]; isHigh {
			priority = "high"
		}
		q.items[code] = &cacheItem{
			data: &CachedQuoteData{
				Code:     code,
				Daily:    price,
				Snapshot: snap,
			},
			fetchedAt: time.Now(),
			priority:  priority,
		}
	}
}

// getAllCachedCodes 返回当前所有已缓存股票代码（高+低优先级）
func (q *quoteCacheImpl) getAllCachedCodes() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	codes := make([]string, 0, len(q.items))
	for code := range q.items {
		codes = append(codes, code)
	}
	return codes
}

// ============================================================================
//  Subscriber — 为 Monitor 提供 model.QuoteSubscriber
//
//	将 quotecache 内部的 QuoteEventBus 适配为 model.QuoteSubscriber 函数签名。
//	Subscribe 已直接返回 model.QuoteEvent，转换在 notifySubscribers 中完成。
// ============================================================================

// Subscriber 返回 model.QuoteSubscriber
func (q *quoteCacheImpl) Subscriber() model.QuoteSubscriber {
	return func(codes []string) (<-chan model.QuoteEvent, func()) {
		ch, subID := q.Subscribe(codes)
		return ch, func() { q.Unsubscribe(subID) }
	}
}

// toQuoteData 将内部 CachedQuoteData 转为 model.QuoteData
func toQuoteData(d *CachedQuoteData) *model.QuoteData {
	if d == nil || d.Daily == nil {
		return nil
	}
	qd := &model.QuoteData{
		Code:      d.Code,
		Name:      d.Name,
		Price:     float64(d.Daily.Close) / 100,
		ChangePct: d.Daily.ChangePct,
		Volume:    d.Daily.Volume,
		Turnover:  d.Daily.Turnover,
		Snapshot:  d.Snapshot,
	}
	if d.Minute != nil {
		for _, b := range d.Minute.Bars {
			qd.Minutes = append(qd.Minutes, model.MinuteBar{
				Time:  b.Time,
				Price: float64(b.Price) / 100,
			})
		}
	}
	return qd
}

