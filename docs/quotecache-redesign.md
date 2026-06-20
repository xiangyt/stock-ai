# QuoteCache 重构方案

> 目标：将 quotecache 改造为内存版 Redis，统一支撑系统中所有 A 股盘中数据消费场景。

## 一、现状 vs 目标

| 维度 | 现状 | 目标 |
|------|------|------|
| 数据获取 | 只调 `GetTodayData` 拿日线 | 只调 `GetIntradayData` 拿分时 (TODO) |
| 日/周/月/年 | 声明了字段但从未填充 | 全部从分时 + DB历史日K 计算得出 |
| 分时数据 | 类型定义了但死代码 | 核心原始数据，一切计算的基础 |
| CachedStock | 嵌入 DBStock + 手动拼接日K头 | 完全通过 QuoteCache 获取全部周期数据 |
| 优先级管理 | Promote/SetLoadHoldingCodes 有定义无调用 | 引用计数管理，启动时+运行时动态升降 |
| 盘后处理 | 19:00 从 DB 刷新覆盖缓存 | 移除（收盘后分时 bar 即是收盘价） |
| 刷新频率 | 高 5min / 低 3次/天 | 高 1min / 普通 5min / 低 3次/天 |

## 二、数据模型重构

### 2.1 MinuteData 扩展

```go
// types.go
type MinuteBar struct {
    Time   string `json:"time"`   // "09:35"
    Price  int64  `json:"price"`  // 当前价(分)
    Volume int64  `json:"volume"` // 累计成交量(股)
    Amount int64  `json:"amount"` // 累计成交额(分) ← 新增
}

type MinuteData struct {
    Bars     []MinuteBar `json:"bars"`
    PreClose int64       `json:"pre_close"` // 昨日收盘价(分)
    Date     string      `json:"date"`      // ← 新增，校验用
}
```

### 2.2 CachedQuoteData 重构

```go
type CachedQuoteData struct {
    Code     string
    Name     string

    // === 唯一原始数据（从 adapter 获取，TODO: DataSource.GetIntradayData） ===
    Intraday *MinuteData

    // === Lazy 计算缓存（Intraday 更新时清空） ===
    daily   *adapter.StockPriceDaily  // 不导出，通过 Daily() 访问
    weekly  *adapter.StockPriceDaily
    monthly *adapter.StockPriceDaily
    yearly  *adapter.StockPriceDaily

    // === 预计算（fetchAndCache 时通过 DB 财报+股本计算） ===
    Snapshot *model.StockDailySnapshot
}
```

**删除字段**：`Daily`, `Weekly`, `Monthly`, `Yearly`（4 个公开字段）

**新增方法**：
```go
func (d *CachedQuoteData) Daily() *adapter.StockPriceDaily
func (d *CachedQuoteData) Weekly(histFn func(string, int) ([]*adapter.StockPriceDaily, error)) *adapter.StockPriceDaily
func (d *CachedQuoteData) Monthly(histFn ...) *adapter.StockPriceDaily
func (d *CachedQuoteData) Yearly(histFn ...) *adapter.StockPriceDaily
func (d *CachedQuoteData) InvalidateComputed()  // Intraday 更新时调用
```

### 2.3 日线计算（Daily）

```go
func (d *CachedQuoteData) computeDaily() *adapter.StockPriceDaily {
    bars := d.Intraday.Bars
    if len(bars) == 0 {
        return nil
    }
    first, last := bars[0], bars[len(bars)-1]

    daily := &adapter.StockPriceDaily{
        Code:      d.Code,
        Date:      d.Intraday.Date,
        Open:      first.Price,
        Close:     last.Price,
        High:      maxPrice(bars),
        Low:       minPrice(bars),
        Volume:    last.Volume,   // 累计值，最后一根即当日总量
        Amount:    last.Amount,
        Turnover:  0,             // TODO: 需要总股本计算换手率
    }

    // 涨跌幅 = (现价 - 昨收) / 昨收 * 100
    if d.Intraday.PreClose > 0 {
        daily.ChangePct = float64(last.Price-d.Intraday.PreClose) /
            float64(d.Intraday.PreClose) * 100
    }

    return daily
}
```

### 2.4 周/月/年计算（Weekly/Monthly/Yearly）

以 `Weekly` 为例：

```
本周数据 = 本周一~周四(DB历史日K) + 今天(Intraday 计算的 Daily)

周一 Open  → 全周 Open
今天 Close → 全周 Close
max(周一~今天 High) → 全周 High
min(周一~今天 Low)  → 全周 Low
sum(周一~今天 Volume) → 全周 Volume
```

实现：

```go
func (d *CachedQuoteData) computePeriod(
    histFn func(code string, startDate, endDate string) ([]*adapter.StockPriceDaily, error),
    startDate, endDate string,
) *adapter.StockPriceDaily {
    // 1. 取历史日K
    history, err := histFn(d.Code, startDate, endDate)
    if err != nil {
        return nil
    }

    // 2. 取今日计算值（如果有 Intraday）
    today := d.Daily()

    // 3. 拼接所有日K
    all := append(history, today)

    // 4. 聚合
    result := &adapter.StockPriceDaily{
        Code:  d.Code,
        Open:  all[0].Open,
        Close: all[len(all)-1].Close,
    }
    for _, k := range all {
        if k.High > result.High { result.High = k.High }
        if k.Low < result.Low || result.Low == 0 { result.Low = k.Low }
        result.Volume += k.Volume
        result.Amount += k.Amount
    }
    return result
}
```

## 三、QuoteCache 依赖注入

由于 CachedStock 不再嵌入 DBStock，历史日K 数据源需要注入到 QuoteCache：

```go
type QuoteCache interface {
    // ... 现有方法 ...

    // SetHistoryProvider 注入历史日K数据源（计算周/月/年必需）
    SetHistoryProvider(fn func(code string, startDate, endDate string) ([]*adapter.StockPriceDaily, error))
}
```

## 四、CachedStock 重构

### 4.1 从嵌入 DBStock → 纯 QuoteCache 代理

```go
// 旧
type CachedStock struct {
    *stocksource.DBStock  // 嵌入，历史K线
    cache  QuoteCache
    code   string
}

// 新
type CachedStock struct {
    cache QuoteCache
    code  string
}
```

### 4.2 方法适配

```go
// GetDailyKline: 原来＝ DBStock历史 + cache当日拼接
// 新＝ 直接调 cache 的日线计算
func (c *CachedStock) GetDailyKline(ctx, adj string) ([]*adapter.StockPriceDaily, error) {
    data, err := c.cache.Get(ctx, c.code)
    if err != nil {
        return nil, err
    }
    // cache 内部已有完整日线
    daily := data.Daily()
    // 历史日K通过 cache 的 historyProvider 获取
    return c.cache.GetDailyKline(ctx, c.code, adj)
}

// GetDailySnapshot: 原来＝ cache.Get + 降级 DBStock
// 新＝ 只用 cache
func (c *CachedStock) GetDailySnapshot(ctx) (*model.StockDailySnapshot, error) {
    data, err := c.cache.Get(ctx, c.code)
    if err != nil {
        return nil, err
    }
    return data.Snapshot, nil
}
```

## 五、刷新策略优化

### 5.1 三级优先级

| 级别 | 刷新间隔 | 并发度 | 适用场景 |
|------|----------|--------|----------|
| 高 (high) | 1 分钟 | 全并发 | 持仓股、Monitor 监控股 |
| 普通 (normal) | 5 分钟 | semaphore(50) | 搜索结果、自选股 |
| 低 (low) | 10:30/12:00/14:00 | semaphore(100) | 大盘指数、临时查询 |

### 5.2 引用计数优先级管理

```go
type quoteCacheImpl struct {
    // ...
    priorityLevel map[string]string   // code → "high"|"normal"|"low"
    refCount      map[string]int      // code → 引用计数
}

func (q *quoteCacheImpl) Promote(code string) {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.refCount[code]++
    q.priorityLevel[code] = "high"
}

func (q *quoteCacheImpl) Demote(code string) {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.refCount[code]--
    if q.refCount[code] <= 0 {
        delete(q.refCount, code)
        q.priorityLevel[code] = "low"
    }
}
```

### 5.3 高优先级刷新 Worker

```go
func (q *quoteCacheImpl) highPriorityWorker() {
    defer q.wg.Done()
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if !utils.IsTradingHours() {
                continue
            }
            q.refreshHighPriority()  // 全并发刷新
        case <-q.stopCh:
            return
        }
    }
}
```

### 5.4 定时清理

```go
// 每日 9:00
func (q *quoteCacheImpl) cleanStaleCache() {
    q.mu.Lock()
    defer q.mu.Unlock()

    // 清空所有 Intraday 数据（昨日过期）
    for code, item := range q.items {
        item.data.Intraday = nil
        item.data.InvalidateComputed()
        item.priorityLevel = "low"  // 全部降级
    }

    // 重新加载持仓股为高优先级
    q.reloadHoldingCodes()  // Promotes them back to "high"
}
```

**移除 19:00 DB 刷新**（收盘后分时最后 bar 即为收盘价，无需再从 DB 覆盖）。

## 六、外部调用链路修复

当前有定义无调用的问题，重构后补齐：

```go
// Wire 注入时
func InitializeApp(cfg *config.Config) (*App, error) {
    // ...
    app.QuoteCache.SetLoadHoldingCodes(func() ([]string, error) {
        return portfolio.GetAllHoldingCodes(app.DB)
    })
    app.QuoteCache.SetHistoryProvider(func(code, start, end string) ([]*adapter.StockPriceDaily, error) {
        return stocksource.GetKlinesBetween(app.DB, code, start, end)
    })
    // ...
}
```

```go
// Monitor 创建/销毁时
func (m *Monitor) Start() {
    for _, code := range m.codes {
        m.quoteCache.Promote(code)  // 升为高频刷新
    }
}

func (m *Monitor) Stop() {
    for _, code := range m.codes {
        m.quoteCache.Demote(code)   // 降级
    }
}
```

## 七、实施步骤

| 序号 | 步骤 | 涉及文件 | 依赖 |
|------|------|----------|------|
| 1 | `MinuteData` 加 `Amount`/`Date` 字段 | `types.go` | — |
| 2 | `DataSource` 接口预留 `GetIntradayData` (TODO) | `adapter/interface.go` | — |
| 3 | `CachedQuoteData` 重构：删除4字段，加 `Daily()`/`Weekly()`/`Monthly()`/`Yearly()` 方法 + lazy cache | `cache.go` | 1 |
| 4 | 实现日/周/月/年计算逻辑 | `cache.go` (~150行新增) | 3 |
| 5 | QuoteCache 加 `SetHistoryProvider` 注入 | `cache.go` | 4 |
| 6 | `fetchAndCache` 改为调 `GetIntradayData` (TODO标记，暂时仍调 `GetTodayData` 兼容) | `cache.go` | 2, 3 |
| 7 | 三级优先级 + 引用计数管理 | `cache.go` | — |
| 8 | 刷新策略调整 (1min/5min/定时) | `cache.go` | 7 |
| 9 | `CachedStock` 重构：移除 DBStock 嵌入 | `stock.go` | 5 |
| 10 | 移除 19:00 DB 刷新逻辑 | `cache.go` | 3 |
| 11 | `toQuoteData` 适配新结构 | `cache.go` | 3 |
| 12 | Wire 注入链路修复 (SetLoadHoldingCodes / SetHistoryProvider) | `cmd/server/wire.go`, `main.go` | 5 |
| 13 | 全量编译验证 | 全项目 | 1~12 |

## 八、Adpater TODO 说明

`DataSource.GetIntradayData` 暂不实现，标记 TODO。过渡期 `fetchAndCache` 仍调用 `GetTodayData`（现有逻辑），日线直接使用 adapter 返回的 `StockPriceDaily` 填充 `dailyCache`，周/月/年的计算逻辑已经在缓存中实现，等 adapter 就绪后切换到分时数据源即可。
