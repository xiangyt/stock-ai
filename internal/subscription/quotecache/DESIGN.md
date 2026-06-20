# quotecache 设计文档

## 定位

内存版行情缓存（简易 Redis），统一支撑系统中所有 A 股盘中数据消费场景。零内部依赖，遵循依赖反转原则。

**两种获取方式：**

| 方式 | 接口 | 说明 |
|------|------|------|
| 主动拉取 | `Get(ctx, code)` | 缓存命中直接返回，未命中通过 Collector 获取 |
| 订阅推送 | `Subscribe(codes)` | 返回 channel + 取消函数，刷新后主动推送 |

## 文件结构

```
internal/subscription/quotecache/
├── collector.go   — IntradayCollector 接口（唯一外部依赖）
├── types.go       — 全部数据类型（MinuteBar / QuoteData / CachedQuoteData / ...）
├── cache.go       — QuoteCache 接口 + 实现 + 调度器 + 订阅推送
├── cache_test.go  — 16 个单元测试
├── doc.go         — 包文档
└── DESIGN.md      — 本文件
```

## 依赖关系

```
                    ┌──────────────────────┐
                    │     quotecache        │
                    │                      │
                    │  定义 IntradayCollector│
                    │  定义 QuoteEvent      │
                    │  定义 QuoteData       │
                    │  定义 StockPriceDaily │
                    └──────────┬───────────┘
                               │ 依赖反转
               ┌───────────────┼───────────────┐
               ▼               ▼               ▼
         cmd/server    internal/adapter   internal/model
        (组装注入)      (实现 Collector)   (类型桥接)
```

零内部依赖：不 import 任何 `stock-ai/internal/*` 包。所有类型本地定义。

## 数据模型

### 原始数据 — Intraday

```go
type MinuteData struct {
    Bars     []MinuteBar  // 分时 bar，按时间升序
    PreClose int64        // 昨日收盘价（分）
    Date     string       // "2026-06-20"
}

type MinuteBar struct {
    Time   string  // "09:35"
    Price  int64   // 当前价（分）
    Volume int64   // 累计成交量（股）
    Amount int64   // 累计成交额（分）
}
```

### 缓存单元 — CachedQuoteData

```go
type CachedQuoteData struct {
    Code     string         // 股票代码
    Name     string         // 股票名称
    Intraday *MinuteData    // 唯一原始数据（分时 bar）
    Meta     map[string]any // 扩展数据（上层 hook 注入）

    // 私有：日线 Lazy 计算缓存
    dailyMu sync.Mutex
    daily   *StockPriceDaily
}
```

### 日线计算 — Daily()

从 Intraday 分时 bar 聚合计算，Lazy + 缓存：

| OHLCV 字段 | 计算规则 |
|------------|----------|
| Open | 首根 bar 价格 |
| Close | 末根 bar 价格 |
| High | 全部 bar 最高价 |
| Low | 全部 bar 最低价 |
| Volume | 末根 bar 累计成交量（分时成交量是累计值） |
| Amount | 末根 bar 累计成交额 |
| ChangePct | (Close - PreClose) / PreClose * 100 |

首次调用时计算并缓存，`Intraday` 刷新时调用 `Invalidate()` 清缓存。周/月/年暂不处理。

## 刷新策略

三级优先级，单一 scheduler goroutine 统一调度：

| 优先级 | 刷新间隔 | 并发度 | 适用场景 | TTL |
|--------|----------|--------|----------|-----|
| High | 1 分钟 | 全并发 | 持仓股、Monitor 监控股 | 60s |
| Normal | 5 分钟 | semaphore(50) | 搜索结果、自选股 | 300s |
| Low | 10:30/12:00/14:00 | semaphore(100) | 大盘指数、临时查询 | 900s |

- `IsActive func() bool` 注入交易时间判断，非活跃时段暂停刷新
- 每日 9:00 自动清空过期缓存

### 调度器

```go
func (q *quoteCacheImpl) scheduler() {
    ticker := time.NewTicker(1 * time.Minute)
    for {
        select {
        case now := <-ticker.C:
            if !isActive() { continue }

            // High: 每分钟全并发
            q.refreshPriority(PriorityHigh, 0)

            // Normal: 每 5 分钟 semaphore(50)
            if now.Minute()%5 == 0 {
                q.refreshPriority(PriorityNormal, 50)
            }

            // Low: 10:30 / 12:00 / 14:00 semaphore(100)
            for _, sched := range []int{10*60+30, 12*60, 14*60} {
                if minOfDay == sched && !executed[sched] {
                    executed[sched] = true
                    go q.refreshPriority(PriorityLow, 100)
                }
            }
        }
    }
}
```

## 订阅推送

### 接口

```go
// Subscribe 返回事件 channel 和取消函数
// 取消函数关闭 channel 并清理内部数据结构
Subscribe(codes []string) (<-chan QuoteEvent, func())
```

### 内部索引

维护两套索引：

| 索引 | 结构 | 用途 | 复杂度 |
|------|------|------|--------|
| 正向 (subByID) | `subID → subEntry` | 生命周期管理 | Subscribe O(1), Unsubscribe O(1) |
| 反向 (codeToSubs) | `code → (subID → channel)` | 推送定位 | notifySubscribers O(k), k=关注该 code 的数量 |

A 股票和 B 股票的订阅互不影响 — `notifySubscribers` 通过 `codeToSubs[code]` 直接定位，不再遍历全部订阅者。

### 取消流程

```
cancel() → unsubscribe(subID, codes)
            ├── 清理反向索引（遍历 codes，从 codeToSubs 中删除 subID）
            ├── close(channel)
            └── 从 subByID 中删除
```

## 核心接口

### Config

```go
type Config struct {
    Collector    CollectorChain           // 分时数据采集器链（必须）
    OnQuoteReady func(*CachedQuoteData)   // 可选：行情就绪后处理 hook
    IsActive     func() bool              // 可选：活跃时段判断（nil = 始终活跃）
}
```

### QuoteCache

```go
type QuoteCache interface {
    Get(ctx context.Context, code string) (*CachedQuoteData, error)
    Subscribe(codes []string) (<-chan QuoteEvent, func())
    Subscriber() QuoteSubscriber
    SetPriority(codes []string, p Priority)
    Start()
    Stop()
    Stats() Stats
}
```

### IntradayCollector

```go
type IntradayCollector interface {
    GetIntraday(ctx context.Context, code string) (*MinuteData, error)
}

type CollectorChain []IntradayCollector
// Collect 按链顺序尝试，第一个成功即返回
```

## 数据流

```
adapter (腾讯/同花顺/东方财富)
       │  实现 IntradayCollector（上层注入）
       ▼
┌──────────────────────────────────────┐
│            quotecache                 │
│                                      │
│  scheduler ──▶ collector.Collect(code)
│       │                              │
│       ▼                              │
│  CachedQuoteData.Intraday = data     │
│  CachedQuoteData.Invalidate()        │
│  OnQuoteReady(CachedQuoteData)       │  ← 上层注入 snapshot 计算
│  notifySubscribers(code)             │
│       │                              │
│  ┌────┴────┐                         │
│  ▼         ▼                         │
│ Get()    Subscribe() ──▶ Monitor     │
└──────────────────────────────────────┘
```

## 扩展机制

`OnQuoteReady` 是可选的后处理 hook，每次 fetch 成功后调用。上层通过此 hook 注入 snapshot 计算：

```go
OnQuoteReady: func(data *CachedQuoteData) {
    snap := computeSnapshot(data.Daily())
    data.Meta["snapshot"] = snap
}
```

`CachedQuoteData.Meta` 是 `map[string]any`，quotecache 不关心其内容。

## 类型桥接

~~由于 quotecache 是零依赖底层包，`model.QuoteEvent` / `model.QuoteSubscriber` 等类型仍保留在 `internal/model/` 中。类型桥接在 `cmd/server/wire.go` 的 `provideQuoteSubscriber` 中完成：~~

~~```go
func provideQuoteSubscriber(cache quotecache.QuoteCache) model.QuoteSubscriber { ... }
```~~

**已消除。** 使用 Go type alias (`type A = B`) 将 `model` 中的类型直接指向 `quotecache`：

```go
// internal/model/monitor_quote.go
type QuoteData      = quotecache.QuoteData
type QuoteEvent     = quotecache.QuoteEvent
type QuoteSubscriber = quotecache.QuoteSubscriber
type MinuteBar      = quotecache.MinuteBarInfo
```

零拷贝、零转换、零桥接代码。`provideQuoteSubscriber` 简化为一行：

```go
func provideQuoteSubscriber(cache quotecache.QuoteCache) quotecache.QuoteSubscriber {
    return cache.Subscriber()
}
```

## 上层组装

```go
// cmd/server/wire.go

func provideQuoteCacheConfig(reg *adapter.Registry) quotecache.Config {
    var chain quotecache.CollectorChain
    for _, name := range []string{"tencentstock", "ths2", "ths", "eastmoney"} {
        if ds, ok := reg.Get(name); ok {
            chain = append(chain, &collectorAdapter{ds: ds})
        }
    }
    return quotecache.Config{
        Collector: chain,
        IsActive:  utils.IsTradingHours,
        OnQuoteReady: func(data *quotecache.CachedQuoteData) {
            snap, _ := datacollect.BuildDailySnapshot(data.Code, adapterDaily(data))
            if snap != nil {
                data.Meta["snapshot"] = snap
            }
        },
    }
}
```

## TODO

- [ ] adapter 层实现真正的 `GetIntraday`（当前 `collectorAdapter` 用 `GetTodayData` 降级，生成伪单 bar）
- [ ] 周/月/年 K 线计算
- [ ] `model.QuoteEvent` → `quotecache.QuoteEvent` 统一（消除类型桥接）
- [ ] RefreshOne 失败重试策略
