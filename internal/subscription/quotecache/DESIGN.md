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
├── types.go       — 全部数据类型
├── cache.go       — QuoteCache 接口 + 实现 + 调度器 + 订阅推送
├── cache_test.go  — 16 个单元测试
├── doc.go         — 包文档
└── DESIGN.md      — 本文件
```

## 依赖关系

```
                    ┌──────────────────────┐
                    │     quotecache        │
                    │  定义 IntradayCollector│
                    │  定义所有数据类型      │
                    └──────────┬───────────┘
                               │ 依赖反转
               ┌───────────────┼───────────────┐
               ▼               ▼               ▼
         cmd/server    internal/adapter   internal/model
        (组装注入)      (实现 Collector)   (type alias)
```

零内部依赖：不 import 任何 `stock-ai/internal/*` 包。

## 数据模型

### 原始数据 — MinuteData

```go
type MinuteData struct {
    Bars     []MinuteBar  // 分时 bar，按时间升序
    PreClose int64        // 昨收（分）
    Date     string       // "2026-06-20"

    // 以下由 collectorAdapter 从 adapter.IntradayData 同步
    Name, Current, High, Low, Volume, Amount, Change, ChangePct, Turnover
    Pe, Pb, MarketCap, FloatMarketCap, Amplitude
    Depth *MarketDepth    // 五档买卖盘口
}
```

### 缓存单元 — CachedQuoteData

```go
type CachedQuoteData struct {
    Code     string
    Name     string         // 从 Intraday.Name 同步
    Intraday *MinuteData    // 唯一原始数据
    Meta     map[string]any // 扩展数据（上层 hook 注入）
}
```

### 日线计算 — Daily()

从分时 bar Lazy 聚合，首次调用计算并缓存，Intraday 刷新时 Invalidate 清缓存：

| OHLCV 字段 | 计算规则 |
|------------|----------|
| Open | 首根 bar 价格 |
| Close | 末根 bar 价格 |
| High | 全部 bar 最高价 |
| Low | 全部 bar 最低价 |
| Volume | 末根 bar 累计成交量 |
| Amount | 末根 bar 累计成交额 |
| ChangePct | (Close - PreClose) / PreClose × 100 |

### QuoteData — 推送快照

```go
type QuoteData struct {
    Code, Name, Date string
    Price, PreClose   float64  // 元
    ChangePct, Turnover, Amplitude float64  // %
    Volume            int64    // 股
    Minutes           []MinuteBarInfo

    // 扩展行情（从 adapter 直传）
    MarketCap, FloatMarketCap float64  // 亿
    Pe, Pb                    float64
    Depth                     *MarketDepth  // 分/股
}
```

## 刷新策略

三级优先级，单一 scheduler goroutine 统一调度：

| 优先级 | 刷新间隔 | 并发度 | TTL |
|--------|----------|--------|-----|
| High | 1 分钟 | 全并发 | 60s |
| Normal | 5 分钟 | semaphore(50) | 300s |
| Low | 10:30/12:00/14:00 | semaphore(100) | 900s |

## 订阅推送

```go
Subscribe(codes []string) (<-chan QuoteEvent, func())
```

取消函数关闭 channel 并清理内部索引。内部维护正向 (subByID) + 反向 (codeToSubs) 两套索引，O(1) 定位关注者。

## 核心接口

### Config

```go
type Config struct {
    Collector    CollectorChain           // 分时数据采集器链（必须）
    OnQuoteReady func(*CachedQuoteData)   // 可选 hook
    IsActive     func() bool              // 活跃时段判断
}
```

### QuoteCache

```go
type QuoteCache interface {
    Get(ctx, code) (*CachedQuoteData, error)
    Subscribe(codes) (<-chan QuoteEvent, func())
    Subscriber() QuoteSubscriber
    SetPriority(codes, p Priority)
    Start() / Stop() / Stats()
}
```

### IntradayCollector

```go
type IntradayCollector interface {
    GetIntraday(ctx, code) (*MinuteData, error)
}
type CollectorChain []IntradayCollector
```

## 数据流

```
adapter (腾讯自选股 minute/query)
       │  实现 IntradayCollector
       ▼
┌──────────────────────────────────────┐
│            quotecache                 │
│  scheduler ──▶ collector.Collect(code)│
│       ▼                              │
│  CachedQuoteData.Intraday = data     │
│  CachedQuoteData.Name = intraday.Name│
│  CachedQuoteData.Invalidate()        │
│  OnQuoteReady(CachedQuoteData)       │
│  notifySubscribers(code)             │
│       └── buildEvent → QuoteEvent    │
└──────────┬───────────────────────────┘
           ▼
        Monitor (4 条规则检查)
```

## 类型桥接

使用 Go type alias 将 model 类型指向 quotecache：

```go
// internal/model/monitor_quote.go
type QuoteData      = quotecache.QuoteData
type QuoteEvent     = quotecache.QuoteEvent
type QuoteSubscriber = quotecache.QuoteSubscriber
type MinuteBar      = quotecache.MinuteBarInfo
```

零拷贝零桥接，`provideQuoteSubscriber` 简化为 `return cache.Subscriber()`。

## 上层组装

```go
// cmd/server/wire.go
func provideQuoteCacheConfig(reg *adapter.Registry) quotecache.Config {
    var chain quotecache.CollectorChain
    if ds, ok := reg.Get(tencentstock.AdapterName); ok {
        chain = append(chain, &collectorAdapter{ds: ds})
    }
    return quotecache.Config{Collector: chain}
}
```

`collectorAdapter` 优先调用 `ds.GetIntraday()`，降级 `ds.GetTodayData()`。

## 单位约定

| 类型 | 价格字段 | 量字段 | 额字段 | 比率字段 |
|------|----------|--------|--------|----------|
| adapter | 分 | 股 | 分 | % |
| quotecache 内部 | 分 | 股 | 分 | % |
| QuoteData (推送) | 元 | 股 | — | % |
| MarketDepth | 分 | 股 | — | — |

## TODO

- [ ] 周/月/年 K 线计算
- [ ] RefreshOne 失败重试策略
- [ ] 多 adapter 降级链（当前仅 tencentstock）
