# internal/monitor — 盯盘监控包

> **事件驱动的实时盯盘监控系统**。监听行情数据流，按用户配置的规则检查异动，冷却过滤后通过多平台（钉钉/飞书/企微）推送告警。

---

## 1. 文件结构

| 文件 | 职能 |
|------|------|
| `monitor.go` | 核心调度器：生命周期管理、Worker 管理、配置热重载、事件处理主循环 |
| `event_bus.go` | 配置变更事件定义（`ChangeType` 枚举 + `ConfigChange`） |
| `checker.go` | 规则检查器：4 种规则（当日涨幅 / 急拉急跌 / 量比异动 / 封单） |
| `cooldown.go` | 冷却控制器：双规则冷却（最小间隔 + 日上限），防重复推送 |
| `push.go` | 消息构建与推送：模板渲染 + 多机器人分发 |

---

## 2. 架构概览

```
                     ┌──────────────────────────────────────┐
                     │         cmd/server/main.go            │
                     │  组装所有组件 + 注入回调 + 启动/停止    │
                     └──────┬──────────┬──────────┬──────────┘
                            │          │          │
           ┌────────────────┼──────────┼──────────┼─────────────────┐
           ▼                ▼          ▼          ▼                 ▼
   ┌────────────┐   ┌────────────┐  ┌────────┐  ┌──────────┐  ┌─────────────┐
   │ quotecache │   │  monitor   │  │  model │  │ notifier │  │  service    │
   │            │   │  (本包)    │  │        │  │          │  │             │
   │QuoteEventBus◄──┤ subscribe  │──┤QuoteData  │──┤ .Send() │  │ CRUD →      │
   │NewQuote    │   │            │  │QuoteEvt│  │          │  │ NotifyChange│
   │Subscriber  │   └─────┬──────┘  │Config  │  └──────────┘  └──────┬──────┘
   └────────────┘         │         └────────┘                      │
                          │                                         │
                          ▼                                         ▼
                   ┌──────────┐                            ┌──────────────┐
                   │    db    │                            │  router.go   │
                   │  DAO 层  │                            │ MonitorConfig│
                   └──────────┘                            │ ServiceRef   │
                                                           └──────────────┘
```

**关键设计原则**：

- **依赖倒置** — Monitor 通过 `QuoteSubscriber`（函数类型）和 `Notifier`（接口）与外部解耦
- **转换内化** — `CachedQuoteData → QuoteData` 转换在 quotecache 的 `notifySubscribers` 中完成，`Subscribe` 直接返回 `model.QuoteEvent`，无额外 goroutine/channel
- **单代码单协程** — 每只被监控的股票拥有独立 goroutine，事件处理互不阻塞

---

## 3. 核心数据结构

### 3.1 Monitor（调度器，单例）

```go
type Monitor struct {
    subscribe  model.QuoteSubscriber   // 行情订阅（依赖注入）
    ntf        notifier.Notifier       // 消息推送接口

    workers    map[string]*codeWorker  // code → goroutine
    codeConfigs map[string][]*MonitorConfig  // code → 配置列表（O(1) 查找）
    configs    map[uint]*MonitorConfig // configID → 配置

    changeCh   chan ConfigChange       // 配置变更通道（cap=64）
    checker    *AlertChecker           // 规则检查
    cooldown   *CooldownController     // 冷却控制
    push       *PushBuilder            // 消息构建
}
```

### 3.2 codeWorker（单股票协程单元）

```go
type codeWorker struct {
    code   string
    ch     <-chan model.QuoteEvent   // 该股票专用的行情事件通道
    cancel func()                     // 取消订阅的回调
}
```

### 3.3 ConfigChange（配置变更事件）

```go
type ChangeType string  // created / updated / deleted / enabled / disabled / holding_changed

type ConfigChange struct {
    Type     ChangeType
    ConfigID uint
}
```

### 3.4 规则检查相关

```go
type Alert struct {
    RuleType    string   // daily_change / rapid_move / volume_ratio / seal_board
    SubType     string   // surge_big / rapid_up / high_volume 等
    Label       string   // 中文标签："涨停" / "大跌" / "急拉"
    ChangePct   float64  // 涨跌幅
    VolumeRatio float64  // 量比
    // ... 其他规则特定字段
}
```

---

## 4. 事件流（端到端）

### 4.1 行情事件流（高频实时）

```
Adapter.fetch → quotecache.refreshCode → notifySubscribers(CachedQuoteData → QuoteData)
    → QuoteEventBus.Subscribe → Monitor.subscribe → processCode goroutine（每 code 一个）
    → processEvent:
        ├─ codeConfigs 索引 O(1) 取该 code 的配置列表
        ├─ checker.Check(rule, data) — 遍历所有活跃规则
        ├─ cooldown.ShouldAlert(configID, code, alertType) — 冷却过滤
        ├─ push.Build(template, data, alert) — 模板渲染
        └─ PushToBots — 多平台推送
```

### 4.2 配置变更流（低频，用户操作触发）

```
HTTP API → MonitorConfigService.CRUD
    → notifyChangeFn(ChangeType, configID)
    → Monitor.changeCh (cap=64, 非阻塞写)
    → configLoop goroutine
    → handleConfigChange:
        ├─ Created/Enabled/Updated → DB 重载单条 → 更新 configs → syncWorkers
        ├─ Deleted/Disabled      → 删除 configs → syncWorkers
        └─ HoldingChanged        → 直接 syncWorkers（重新解析 ScopeHeld）
```

---

## 5. 启动与生命周期

```go
// main.go 中的组装顺序

// 1. 创建 Monitor（依赖注入）
mon = monitor.NewMonitor(
    quoteCache.Subscriber(),  // 行情订阅（quotecache 内化转换）
    ntf,                       // 消息推送
)

// 2. 注入配置变更回调
router.MonitorConfigServiceRef.SetNotifyChange(func(ct monitor.ChangeType, id uint) {
    mon.NotifyChange(monitor.ConfigChange{Type: ct, ConfigID: id})
})

// 3. 启动
mon.Start()
    ├─ 加载所有 is_active=true 的配置
    ├─ 为每只股票启动独立 Worker
    └─ 启动 configLoop 监听配置变更

// 4. 关闭
mon.Stop()  // 取消所有 Worker、关闭 channel、停止 configLoop
```

---

## 6. Worker 同步机制（syncWorkers）

当配置变更时，`syncWorkers` 通过 Diff 算法增量更新 Worker 集合：

```
输入: codes []string (当前需要的全部股票代码)

Step 1: wanted = set(codes)
Step 2: 遍历 workers，不在 wanted → cancel() + delete
Step 3: 遍历 wanted，不在 workers → subscribe + 启动新 goroutine
Step 4: 重建 codeConfigs 索引：
         遍历 configs → resolveCodes 展开 → code → []*MonitorConfig
```

### 6.1 股票代码解析（resolveCodes）

```go
func (m *Monitor) resolveCodes(cfg *MonitorConfig) []string {
    switch cfg.Scope {
    case ScopeHeld:   return db.GetAllActiveStockCodes(cfg.UID)  // 持仓股
    case ScopeCustom: return cfg.ParseStocks()                    // 自选股 JSON
    }
}
```

---

## 7. 四大规则检查器

### 7.1 当日涨幅（daily_change）

6 档级联检查（涨停/跌停触发后直接 return，不再检查大涨/小涨）：

| 档位 | 默认阈值 | 说明 |
|------|---------|------|
| 涨停 (limit_up) | ≥ 9.8% | 最高优先级，触发后跳过后续档位 |
| 大涨 (surge_big) | [9%, 9.8%) | |
| 小涨 (surge_small) | [5%, 9%) | |
| 小跌 (drop_small) | [-5%, 0) | |
| 大跌 (drop_big) | [-9%, -5%) | |
| 跌停 (limit_down) | ≤ -9.8% | 最高优先级，触发后跳过后续档位 |

每档有独立开关。

### 7.2 急拉急跌（rapid_move）

- 使用分时数据 `Minutes` 计算指定窗口内的涨跌幅
- 无分时数据时降级为全天涨跌幅
- 统一幅度阈值 + 上拉/下砸独立开关

### 7.3 量比异动（volume_ratio）

- `ratio = 今日成交量 / 近5日日均成交量`
- 日均量从 DB K 线表查询
- 数据不足时静默跳过

### 7.4 封单数量（seal_board）

- **占位实现**，始终返回 nil
- 需要 Level 2 行情数据（买一/卖一挂单手数），当前不可用

---

## 8. 冷却机制（CooldownController）

双层冷却，防止同一告警重复推送：

| 维度 | 机制 | 说明 |
|------|------|------|
| 时间间隔 | `lastAlert[typeKey]` | key = `configID:code:alertType`，距上次告警不足 interval 分钟则跳过 |
| 每日上限 | `dailyCount[dayKey]` | key = `configID:code:alertType:日期`，超过 daily_max 则当日不再推送 |
| 跨天重置 | `today` 字段 | 检测到日期变化时自动清空 `dailyCount` |

冷却配置按 `MonitorConfig.Cooldown` JSON 字段，支持 `interval_minutes` 和 `daily_max`。

---

## 9. 消息推送

### 9.1 模板渲染（PushBuilder.Build）

支持 `${变量名}` 模板语法，可用变量：

| 变量 | 含义 | 来源 |
|------|------|------|
| `${name}` | 股票名称 | QuoteData.Name |
| `${code}` | 股票代码 | QuoteData.Code |
| `${price}` | 当前价 | QuoteData.Price |
| `${change_pct}` | 涨跌幅 | QuoteData.ChangePct |
| `${alert_label}` | 告警标签 | Alert.Label |
| `${volume}` | 成交量 | QuoteData.Volume |
| `${turnover}` | 换手率 | QuoteData.Turnover |

### 9.2 多平台分发

通过 `PushToBots` → `notifier.Send` 发送到关联的机器人。`Notifier` 接口支持三个渠道：

- **钉钉** — Webhook + HMAC-SHA256 加签
- **飞书** — 自定义机器人
- **企微** — 群机器人

失败自动重试 1 次（500ms 间隔）。

---

## 10. 并发模型

```
                    ┌─────────────────┐
                    │   changeCh (64)  │ ← 配置变更（低频）
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  configLoop     │ 单 goroutine
                    │  串行消费变更    │
                    └────────┬────────┘
                             │ syncWorkers
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        ┌──────────┐  ┌──────────┐  ┌──────────┐
        │ 000001   │  │ 600036   │  │ ...      │
        │ goroutine│  │ goroutine│  │ goroutine│
        │processCode   processCode│  │          │
        └──────────┘  └──────────┘  └──────────┘
              │              │              │
              ▼              ▼              ▼
        共享只读索引：codeConfigs (RWMutex) + configs (RWMutex)
```

- 每个被监控的股票代码拥有独立 goroutine
- 行情事件处理互不阻塞
- 配置变更通过 channel 通知，串行消费保证一致性
- 共享索引使用 RWMutex 保护，读多写少

---

## 11. 外部依赖

| 依赖 | 用途 | 解耦方式 |
|------|------|---------|
| `quotecache` | 行情数据源 | `QuoteCache.Subscriber()` 返回 `model.QuoteSubscriber`；转换在 notifySubscribers 完成 |
| `notifier` | 消息推送 | `Notifier` 接口 |
| `model` | 数据模型 | DDD 分层，Monitor 依赖 model 包的类型定义 |
| `db` | 数据库访问 | 直接调用 DAO 函数（配置加载 / 机器人查询 / 持仓查询 / 日均量） |
| `service` | 业务逻辑 | 通过 `NotifyChange` 回调注入，service 不直接依赖 monitor |
