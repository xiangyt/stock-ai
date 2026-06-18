# 回测模块可插拔架构重构方案

> 版本: v1.1 | 日期: 2026-06-17 | 状态: 已实施
> v1.1 更新：接口设计改为极简核心接口 + 可选扩展接口（参照 Signal 系统 BaseSignal 模式）
> 实施后最终结构：types/（公共类型+注册表）+ exit/（5个checker）+ alloc/（5个分配器）+ exports.go（类型别名兼容）
> 详见：[回测引擎详细文档](backtest-engine.md)

---

## 目录

- [1. 背景与问题分析](#1-背景与问题分析)
- [2. 重构目标](#2-重构目标)
- [3. 卖出规则可插拔架构](#3-卖出规则可插拔架构)
- [4. 仓位管理可插拔架构](#4-仓位管理可插拔架构)
- [5. 文件结构](#5-文件结构)
- [6. 数据迁移](#6-数据迁移)
- [7. 实施任务清单](#7-实施任务清单)

---

## 1. 背景与问题分析

### 1.1 现有卖出规则架构

```
ExitRules (struct)
  ├── StopLoss    *StopLossRule      ← 固定字段
  ├── TakeProfit  *TakeProfitRule    ← 固定字段
  ├── TimeExit    *TimeExitRule      ← 固定字段
  └── ExitSignals []any              ← P2 预留

ExitChecker interface
  └── Check(pos, bar, rules *ExitRules)  ← 传入整个 ExitRules

HoldingPosition (struct)
  ├── PreStopLoss   float64          ← 固定字段
  ├── PreTakeProfit float64           ← 固定字段
  └── ExpiryDate    string           ← 固定字段

newExitCheckerChain()
  └── return {StopLossChecker, TakeProfitChecker, TimeExitChecker}  ← 硬编码
```

**问题清单：**

| # | 问题 | 影响 |
|---|------|------|
| 1 | `Check()` 接收 `*model.ExitRules`，每个 checker 能看到全部规则 | 新增规则类型需改 ExitRules struct |
| 2 | `HoldingPosition` 预算字段写死（PreStopLoss/PreTakeProfit/ExpiryDate） | TrailingStop 需要 HighWater、SegmentProfit 需要 已触发档位 |
| 3 | `newExitCheckerChain()` 硬编码三个 checker | 无法按配置动态组装 |
| 4 | 只支持整仓卖出（`executeSell` 删 position） | 分段止盈需要部分卖出 |
| 5 | 无生命周期钩子 | TrailingStop 需在建仓时初始化、每日更新最高价 |

### 1.2 现有仓位管理架构

```go
// PositionRules — 仅一个 allocation 字符串字段
type PositionRules struct {
    MaxPositions int     `json:"max_positions"`
    MaxSinglePct float64 `json:"max_single_pct"`
    Allocation   string  `json:"allocation"`  // "equal" | 预留
}

// calcAllocation — 一个函数搞定，无接口
func calcAllocation(cash, maxPositions, currentHoldingCount int, maxSinglePct, totalEquity float64) float64
```

**问题清单：**

| # | 问题 | 影响 |
|---|------|------|
| 1 | 无 Allocator 接口，单一函数 | 无法插入信号加权、波动率加权等策略 |
| 2 | 函数签名只传标量 | 无法传入候选股的信号评分、波动率等上下文 |
| 3 | allocation 仅字符串 | 无法携带策略参数（如加权系数、风险目标） |

---

## 2. 重构目标

| 维度 | 目标 |
|------|------|
| **卖出规则** | **极简核心接口**（`Check`+`Name`），生命周期和优先级通过可选扩展接口实现，注册表工厂模式，支持部分卖出 |
| **仓位管理** | Allocator 接口、注册表工厂模式、候选上下文传递、多种分配策略 |
| **向后兼容** | JSON 字段格式迁移，旧数据自动转换 |
| **代码组织** | 每个规则/分配器独立文件，新增不改现有文件 |
| **接口设计原则** | 参照项目 Signal 系统 `BaseSignal` 模式：核心接口极简，扩展能力通过可选接口按需实现 |

---

## 3. 卖出规则可插拔架构

### 3.1 核心接口设计

**设计原则：一个接口，一切卖出规则都是它的实现。** 未来新增规则只需实现这个接口 + 注册工厂，引擎零改动。

```go
// ExitChecker 卖出规则接口 — 唯一需要实现的接口
type ExitChecker interface {
    // Check 检查是否触发卖出，返回 nil 表示不触发
    Check(pos *HoldingPosition, bar DayBar) *ExitDecision
    // Name 规则类型名（用于状态隔离和日志）
    Name() string
}
```

接口只有 2 个方法，极简。优先级和生命周期钩子通过**可选扩展接口**实现，引擎用类型断言按需调用：

```go
// 以下接口均为可选 — checker 按需实现，不实现就跳过

// PriorityChecker 需要控制执行顺序的 checker 实现
type PriorityChecker interface {
    Priority() int // 数字越小越优先，默认 100
}

// EntryHook 需要建仓时初始化状态的 checker 实现
type EntryHook interface {
    OnEntry(pos *HoldingPosition, entryPrice float64, entryDate string)
}

// DailyUpdateHook 需要每日更新状态的 checker 实现（如移动止盈更新最高价）
type DailyUpdateHook interface {
    OnDailyUpdate(pos *HoldingPosition, bar DayBar)
}
```

**为什么用可选接口而不是全塞进 ExitChecker？**

| 方式 | 新增简单规则时 | 问题 |
|------|----------------|------|
| **全塞进一个接口**（旧设计） | 必须实现 5 个方法，3 个是空实现 | 接口臃肿，违反 ISP |
| **可选扩展接口**（新设计） | 只实现 `Check` + `Name` 即可 | 需要状态的 checker 才实现对应 Hook |

引擎用类型断言按需调用：
```go
for _, c := range chain {
    if h, ok := c.(EntryHook); ok { h.OnEntry(pos, entryPrice, entryDate) }
}
// 每日循环：
for _, c := range chain {
    if h, ok := c.(DailyUpdateHook); ok { h.OnDailyUpdate(pos, bar) }
}
// Check 前按优先级排序（PriorityChecker 未实现 → 默认 100）
```

**关键改进：**
- `Check()` 不再接收 `*model.ExitRules`，checker 自包含配置参数
- 生命周期钩子是可选的，简单规则不需要写空实现
- 与项目 Signal 系统的 `BaseSignal` 模式一致

### 3.2 ExitDecision 支持部分卖出

```go
type ExitDecision struct {
    Reason   string  // "stop_loss" | "take_profit" | "time_exit" | "trailing_stop" | "segment_profit" | "signal" | "force_close"
    Price    float64
    Quantity int     // 0 = 整仓卖出; >0 = 部分卖出指定数量
}
```

引擎执行逻辑：
- `Quantity == 0` → 整仓卖出，从 holdings 删除（现有逻辑）
- `Quantity > 0` → 部分卖出，减少 pos.Quantity，保留持仓

### 3.3 HoldingPosition 可扩展状态

```go
type HoldingPosition struct {
    StockCode  string
    Quantity   int
    EntryPrice float64
    EntryDate  string
    HoldDays   int

    // checker 专属状态（由 OnEntry/OnDailyUpdate 维护）
    // key = checker.Name(), value = checker 自定义结构
    State map[string]any
}

// SetState checker 写入自己的状态（key = checker.Name()）
func (p *HoldingPosition) SetState(key string, val any) { p.State[key] = val }

// GetState checker 读取自己的状态
func (p *HoldingPosition) GetState(key string) any { return p.State[key] }
```

`PreStopLoss`/`PreTakeProfit`/`ExpiryDate` 不再是固定字段，改为 checker 在 `OnEntry` 时通过 `SetState(checker.Name(), ...)` 写入 `State`。

### 3.4 注册表 + 工厂模式

```go
// ExitRuleConfig JSON 中的单条规则配置
type ExitRuleConfig struct {
    Type     string          `json:"type"`      // "stop_loss" | "take_profit" | ...
    Enabled  bool            `json:"enabled"`
    Params   json.RawMessage `json:"params"`    // 类型专属参数
    Priority *int            `json:"priority,omitempty"` // 可选，覆盖默认优先级
}

// ExitCheckerFactory 从 JSON params 创建 checker
type ExitCheckerFactory func(params json.RawMessage) (ExitChecker, error)

// 注册表
var exitCheckerFactories = map[string]ExitCheckerFactory{}

func RegisterExitChecker(ruleType string, factory ExitCheckerFactory)
func BuildExitChain(configs []ExitRuleConfig) ([]ExitChecker, error)
```

`BuildExitChain` 用工厂创建 checker 后，若 `ExitRuleConfig.Priority` 非 nil 则覆盖 `PriorityChecker.Priority()` 的默认值。新增规则只需：
1. 实现 `ExitChecker`（按需实现 `PriorityChecker`/`EntryHook`/`DailyUpdateHook`）
2. 写一个 `ExitCheckerFactory` 函数
3. 在 `init()` 里调用 `RegisterExitChecker`

### 3.5 新的 exit_rules JSON Schema

```jsonc
{
  "rules": [
    {
      "type": "stop_loss",
      "enabled": true,
      "params": { "threshold_pct": -8.0 }
    },
    {
      "type": "take_profit",
      "enabled": true,
      "params": { "threshold_pct": 20.0 }
    },
    {
      "type": "time_exit",
      "enabled": true,
      "params": { "hold_days": 60 }
    },
    {
      "type": "trailing_stop",
      "enabled": true,
      "params": { "trail_pct": 5.0, "activation_pct": 10.0 }
    },
    {
      "type": "segment_profit",
      "enabled": false,
      "params": {
        "levels": [
          { "threshold_pct": 10.0, "sell_ratio": 0.5 },
          { "threshold_pct": 20.0, "sell_ratio": 0.5 }
        ]
      }
    }
  ],
  "slippage_pct": 0.3
}
```

### 3.6 Go 结构体（新版 model）

```go
// ExitRules 卖出规则集（JSON 持久化）
type ExitRules struct {
    Rules       []ExitRuleConfig `json:"rules"`
    SlippagePct float64          `json:"slippage_pct"` // 默认 0.3
}

// ExitRuleConfig 单条规则配置
type ExitRuleConfig struct {
    Type     string          `json:"type"`
    Enabled  bool            `json:"enabled"`
    Params   json.RawMessage `json:"params"`
    Priority int             `json:"priority,omitempty"`
}
```

### 3.7 内置 Checker 实现

每个 checker 只实现需要的接口，无需空实现：

| Checker | ExitChecker | PriorityChecker | EntryHook | DailyUpdateHook | 文件 |
|---------|:-----------:|:---------------:|:---------:|:---------------:|------|
| StopLoss | ✅ Check+Name | ✅ Priority=1 | ✅ OnEntry | — | `exit_stop_loss.go` |
| TakeProfit | ✅ Check+Name | ✅ Priority=2 | ✅ OnEntry | — | `exit_take_profit.go` |
| TimeExit | ✅ Check+Name | ✅ Priority=3 | ✅ OnEntry | — | `exit_time_exit.go` |
| TrailingStop | ✅ Check+Name | ✅ Priority=2 | ✅ OnEntry | ✅ OnDailyUpdate | `exit_trailing_stop.go` |
| SegmentProfit | ✅ Check+Name | ✅ Priority=2 | ✅ OnEntry | — | `exit_segment_profit.go` |

#### 代码示例 — StopLossChecker（最简模式）

```go
type StopLossChecker struct {
    thresholdPct float64 // 自包含配置
    slippagePct  float64
}

func (s *StopLossChecker) Name() string { return "stop_loss" }
func (s *StopLossChecker) Priority() int { return 1 }

// 实现 EntryHook — 建仓时算止损价
func (s *StopLossChecker) OnEntry(pos *HoldingPosition, entryPrice float64, entryDate string) {
    stopLoss := entryPrice * (1 + s.thresholdPct/100)
    pos.SetState("stop_loss", stopLoss) // 写入 pos.State["stop_loss"]
}

// 不实现 DailyUpdateHook — 引擎自动跳过

func (s *StopLossChecker) Check(pos *HoldingPosition, bar DayBar) *ExitDecision {
    stopLoss := pos.GetState("stop_loss").(float64)
    if bar.Low <= stopLoss {
        execPrice := stopLoss * (1 - s.slippagePct/100)
        if execPrice < bar.Low { execPrice = bar.Low }
        return &ExitDecision{Reason: "stop_loss", Price: execPrice}
    }
    return nil
}
```

#### 代码示例 — TrailingStopChecker（最复杂模式）

```go
type TrailingStopChecker struct {
    trailPct      float64
    activationPct float64
}

type trailingState struct {
    highWater  float64
    activated  bool
}

func (t *TrailingStopChecker) Name() string { return "trailing_stop" }
func (t *TrailingStopChecker) Priority() int { return 2 }

func (t *TrailingStopChecker) OnEntry(pos *HoldingPosition, entryPrice float64, _ string) {
    pos.SetState("trailing_stop", &trailingState{highWater: entryPrice})
}

// 实现 DailyUpdateHook — 每日更新最高价和激活状态
func (t *TrailingStopChecker) OnDailyUpdate(pos *HoldingPosition, bar DayBar) {
    st := pos.GetState("trailing_stop").(*trailingState)
    if bar.High > st.highWater { st.highWater = bar.High }
    if !st.activated && bar.Close >= pos.EntryPrice*(1+t.activationPct/100) {
        st.activated = true
    }
}

func (t *TrailingStopChecker) Check(pos *HoldingPosition, bar DayBar) *ExitDecision {
    st := pos.GetState("trailing_stop").(*trailingState)
    if !st.activated { return nil } // 未激活不检查
    triggerPrice := st.highWater * (1 - t.trailPct/100)
    if bar.Low <= triggerPrice {
        return &ExitDecision{Reason: "trailing_stop", Price: triggerPrice}
    }
    return nil
}
```

#### 各 Checker 规格速查

**StopLossChecker** (`exit_stop_loss.go`)

| 项 | 内容 |
|----|------|
| type | `"stop_loss"` |
| priority | 1 (最高) |
| params | `{ "threshold_pct": -8.0 }` |
| OnEntry | 计算 `pre_stop_loss = entryPrice × (1 + threshold_pct/100)` |
| Check | 读取 `pre_stop_loss`，判断 `bar.Low <= pre_stop_loss`，含滑点逻辑 |

**TakeProfitChecker** (`exit_take_profit.go`)

| 项 | 内容 |
|----|------|
| type | `"take_profit"` |
| priority | 2 |
| params | `{ "threshold_pct": 20.0 }` |
| OnEntry | 计算 `pre_take_profit = entryPrice × (1 + threshold_pct/100)` |
| Check | 判断 `bar.High >= pre_take_profit`，限价单成交 |

**TimeExitChecker** (`exit_time_exit.go`)

| 项 | 内容 |
|----|------|
| type | `"time_exit"` |
| priority | 3 |
| params | `{ "hold_days": 60 }` |
| OnEntry | 计算 `expiry_date = addTradingDays(entryDate, hold_days)` |
| Check | 判断 `bar.Date >= expiry_date`，按收盘价成交 |

**TrailingStopChecker** (`exit_trailing_stop.go`) — 新增

| 项 | 内容 |
|----|------|
| type | `"trailing_stop"` |
| priority | 2 |
| params | `{ "trail_pct": 5.0, "activation_pct": 10.0 }` |
| OnEntry | 初始化 `high_water = entryPrice`, `activated = false` |
| OnDailyUpdate | `high_water = max(high_water, bar.High)`; 若涨幅超 `activation_pct` 则 `activated = true` |
| Check | 仅 `activated == true` 时检查：`bar.Low <= high_water × (1 - trail_pct/100)` |
| 说明 | 移动止盈：涨到 activation_pct 才激活，之后从最高点回撤 trail_pct 触发卖出 |

**SegmentProfitChecker** (`exit_segment_profit.go`) — 新增

| 项 | 内容 |
|----|------|
| type | `"segment_profit"` |
| priority | 2 |
| params | `{ "levels": [{"threshold_pct": 10, "sell_ratio": 0.5}, {"threshold_pct": 20, "sell_ratio": 0.5}] }` |
| OnEntry | 初始化 `triggered` 数组，`original_qty = pos.Quantity` |
| Check | 遍历 levels，未触发的 level：`bar.High >= entryPrice × (1 + threshold_pct/100)` → 触发部分卖出，`Quantity = original_qty × sell_ratio`。全部 level 触发后 → Quantity=0 整仓清 |
| 部分卖出 | `ExitDecision.Quantity > 0`，引擎减少 pos.Quantity 不删除持仓 |

### 3.8 引擎主循环改动

```go
// 建仓时 — 调用所有实现 EntryHook 的 checker
func onEntry(chain []ExitChecker, pos *HoldingPosition, entryPrice float64, entryDate string) {
    pos.State = make(map[string]any)
    for _, c := range chain {
        if h, ok := c.(EntryHook); ok {
            h.OnEntry(pos, entryPrice, entryDate)
        }
    }
}

// 每日卖出检查 — 先 DailyUpdateHook 再 Check
for code, pos := range rc.holdings {
    bar := bars[code]

    // 更新所有实现 DailyUpdateHook 的 checker 状态
    for _, c := range chain {
        if h, ok := c.(DailyUpdateHook); ok {
            h.OnDailyUpdate(pos, bar)
        }
    }

    // 按优先级排序后执行 Check（未实现 PriorityChecker 的默认 100）
    for _, c := range getSortedChain(chain) {
        if decision := c.Check(pos, bar); decision != nil {
            if decision.Quantity > 0 {
                s.executePartialSell(rc, code, pos, bar, decision, date)
            } else {
                s.executeSell(rc, code, pos, bar, decision, date)
            }
            break // 触发一个就卖出，不再检查低优先级规则
        }
    }
}

// getSortedChain 按 Priority 排序（PriorityChecker 未实现 → 默认 100）
func getSortedChain(chain []ExitChecker) []ExitChecker {
    sorted := make([]ExitChecker, len(chain))
    copy(sorted, chain)
    sort.Slice(sorted, func(i, j int) bool {
        pi := 100; if pc, ok := sorted[i].(PriorityChecker); ok { pi = pc.Priority() }
        pj := 100; if pc, ok := sorted[j].(PriorityChecker); ok { pj = pc.Priority() }
        return pi < pj
    })
    return sorted
}
```

---

## 4. 仓位管理可插拔架构

### 4.1 核心接口设计

```go
// Allocator 资金分配策略接口
type Allocator interface {
    // Allocate 计算每只候选股的买入金额
    Allocate(req *AllocRequest) *AllocResult
    // Name 策略名
    Name() string
}

// AllocRequest 分配请求（包含上下文信息）
type AllocRequest struct {
    Cash             float64           // 可用现金
    TotalEquity      float64           // 总权益
    MaxPositions     int               // 最大持仓数
    CurrentHoldings  int               // 当前持仓数
    MaxSinglePct     float64           // 单票上限百分比, 0=不限
    CashBufferPct    float64           // 现金缓冲比例, 默认 5%
    Candidates       []CandidateInfo   // 通过买入信号的候选股
}

// CandidateInfo 候选股信息（供加权策略使用）
type CandidateInfo struct {
    Code         string
    Price        float64
    SignalScore  float64  // 信号评分 0-100（可选）
    Volatility   float64  // 20日波动率（可选）
}

// AllocResult 分配结果
type AllocResult struct {
    Allocations map[string]float64  // code -> 买入金额
}

// AllocatorFactory 从 JSON params 创建分配器
type AllocatorFactory func(params json.RawMessage) (Allocator, error)

// 注册表
var allocatorFactories = map[string]AllocatorFactory{}

func RegisterAllocator(name string, factory AllocatorFactory)
func CreateAllocator(allocType string, params json.RawMessage) (Allocator, error)
```

### 4.2 新的 position_rules JSON Schema

```jsonc
{
  "max_positions": 5,
  "max_single_pct": 20.0,
  "cash_buffer_pct": 5.0,
  "allocation": {
    "type": "equal",
    "params": {}
  }
}
```

### 4.3 Go 结构体（新版 model）

```go
type PositionRules struct {
    MaxPositions  int           `json:"max_positions"`
    MaxSinglePct  float64       `json:"max_single_pct"`
    CashBufferPct float64       `json:"cash_buffer_pct"`  // 默认 5.0
    Allocation    AllocationConfig `json:"allocation"`
}

type AllocationConfig struct {
    Type   string          `json:"type"`
    Params json.RawMessage `json:"params"`
}
```

### 4.4 内置 Allocator 实现

#### EqualAllocator (`alloc_equal.go`)

| 项 | 内容 |
|----|------|
| type | `"equal"` |
| 逻辑 | 等权分配：`perStock = cash × (1-buffer) / maxPositions`，受 max_single_pct 上限约束 |
| 说明 | 现有逻辑直接迁移 |

#### SignalWeightedAllocator (`alloc_signal.go`) — 新增

| 项 | 内容 |
|----|------|
| type | `"signal_weighted"` |
| params | `{ "min_weight": 0.5, "max_weight": 1.5 }` |
| 逻辑 | 按信号评分加权：`weight = min + (score/100) × (max-min)`，归一化后分配 |
| 说明 | 评分高的票多配资金，评分低的少配 |

#### VolatilityWeightedAllocator (`alloc_volatility.go`) — 新增

| 项 | 内容 |
|----|------|
| type | `"volatility_weighted"` |
| params | `{ "target_vol": 0.02 }` |
| 逻辑 | 反波动率加权：`weight = target_vol / stock_vol`，归一化后分配 |
| 说明 | 低波动多配，高波动少配，类似风险平价思路 |

#### RiskParityAllocator (`alloc_risk_parity.go`) — 新增

| 项 | 内容 |
|----|------|
| type | `"risk_parity"` |
| params | `{}` |
| 逻辑 | 各股贡献相同风险：`weight_i ∝ 1/vol_i`，归一化 |
| 说明 | 与波动率加权类似但更严格的风险均衡 |

#### CustomWeightAllocator (`alloc_custom.go`) — 新增

| 项 | 内容 |
|----|------|
| type | `"custom_weight"` |
| params | `{ "weights": {"000001": 1.0, "600519": 1.5} }` |
| 逻辑 | 按用户手动设定的权重分配 |
| 说明 | 灵活但需手动维护，适合有主观判断的场景 |

### 4.5 引擎买入流程改动

```go
// 现有逻辑：单只股票逐一计算 allocAmount
// 新逻辑：收集所有通过信号的候选股，一次性传入 Allocator

// 1. 收集候选股信息
candidates := []CandidateInfo{}
for _, code := range passedCodes {
    candidates = append(candidates, CandidateInfo{
        Code:        code,
        Price:       bars[code].Close,
        SignalScore: results[code].Score,    // 从信号评估结果获取
        Volatility:  calcVolatility(code),   // 从K线计算
    })
}

// 2. 调用 Allocator 一次性分配
allocReq := &AllocRequest{
    Cash:            rc.cash,
    TotalEquity:     totalEquity,
    MaxPositions:    posRules.MaxPositions,
    CurrentHoldings: len(rc.holdings),
    MaxSinglePct:    posRules.MaxSinglePct,
    CashBufferPct:   posRules.CashBufferPct,
    Candidates:      candidates,
}
allocResult := allocator.Allocate(allocReq)

// 3. 按分配结果执行买入
for _, cand := range candidates {
    amount := allocResult.Allocations[cand.Code]
    if amount <= 0 { continue }
    buyQty := int(amount/cand.Price/100) * 100
    // ... 执行买入逻辑
}
```

---

## 5. 文件结构（最终实施结果）

```
internal/backtest/
├── types/                            公共类型 + 注册表（零依赖）
│   ├── exit.go                       ExitChecker/PriorityChecker/EntryHook/DailyUpdateHook + HoldingPosition + ExitDecision + DayBar + 注册表 + BuildExitChain
│   └── alloc.go                      Allocator + AllocRequest/Result/CandidateInfo + CalcBuyAmount + 注册表 + CreateAllocator
│
├── exit/                             卖出规则实现（依赖 types）
│   ├── stop_loss.go                  StopLossChecker
│   ├── take_profit.go                TakeProfitChecker
│   ├── time_exit.go                  TimeExitChecker
│   ├── trailing_stop.go              TrailingStopChecker（新增）
│   └── segment_profit.go             SegmentProfitChecker（新增，支持部分卖出）
│
├── alloc/                            仓位分配实现（依赖 types）
│   ├── equal.go                      EqualAllocator
│   ├── signal.go                     SignalWeightedAllocator（新增）
│   ├── volatility.go                 VolatilityWeightedAllocator（新增）
│   ├── risk_parity.go                RiskParityAllocator（新增）
│   └── custom.go                     CustomWeightAllocator（新增）
│
├── exports.go                        类型别名（types → backtest 平铺，兼容引擎代码）
├── init.go                           空白导入 exit/alloc 触发注册
├── service.go                        引擎主循环（修改）
├── engine_helpers.go                 工具函数（修改）
├── metrics.go                        绩效计算
├── model.go / dao.go / handler.go    数据模型/访问/HTTP
└── engine.go                         引擎辅助
```

### 5.2 依赖关系

```
types  ←  exit, alloc  ←  backtest (via init.go 空白导入)
  ↑                          │
  └──────────────────────────┘ (exports.go 类型别名)
```

无循环依赖。子包只依赖 `types`，不依赖 `backtest`。

### 5.3 修改的现有文件

| 文件 | 改动 |
|------|------|
| `internal/model/backtest_rules.go` | ExitRules/PositionRules 结构体重构 |
| `internal/backtest/engine_helpers.go` | 删除 `newExitCheckerChain()` |
| `internal/backtest/service.go` | 买入/卖出流程适配新接口，新增 `executePartialSell` |

### 5.4 删除的文件

| 文件 | 原因 |
|------|------|
| `exit_check.go` | 旧接口/checker 定义（迁移至 types/exit.go + exit/*.go） |
| `exit_checker.go` / `exit_registry.go` | 迁移至 types/exit.go |
| `allocator.go` / `alloc_registry.go` | 迁移至 types/alloc.go |
| `exit_stop_loss.go` ~ `exit_segment_profit.go` | 迁移至 exit/ |
| `alloc_equal.go` ~ `alloc_custom.go` | 迁移至 alloc/ |
| `position_mgr.go` | 旧函数已迁移至 checker/allocator |

---

## 6. 数据迁移

### 6.1 JSON 格式转换

旧格式 → 新格式自动转换（在 Go 反序列化时处理）：

```go
// ExitRules UnmarshalJSON — 兼容旧格式
func (e *ExitRules) UnmarshalJSON(data []byte) error {
    // 尝试新格式
    var newFmt struct {
        Rules       []ExitRuleConfig `json:"rules"`
        SlippagePct float64          `json:"slippage_pct"`
    }
    if err := json.Unmarshal(data, &newFmt); err == nil && len(newFmt.Rules) > 0 {
        e.Rules = newFmt.Rules
        e.SlippagePct = newFmt.SlippagePct
        return nil
    }
    // 旧格式兼容
    var oldFmt struct {
        StopLoss    *StopLossRule   `json:"stop_loss"`
        TakeProfit  *TakeProfitRule `json:"take_profit"`
        TimeExit    *TimeExitRule   `json:"time_exit"`
        SlippagePct float64         `json:"slippage_pct"`
    }
    if err := json.Unmarshal(data, &oldFmt); err == nil {
        e.Rules = convertOldExitRules(oldFmt)
        e.SlippagePct = oldFmt.SlippagePct
        return nil
    }
    return fmt.Errorf("invalid exit_rules JSON")
}
```

`PositionRules.UnmarshalJSON` 同理处理 `"allocation": "equal"` 旧格式 → `{ "type": "equal", "params": {} }`。

### 6.2 无需 DDL 变更

`exit_rules` 和 `position_rules` 在 DB 中是 JSON 字符串，结构变更不影响 schema。旧数据在读取时自动转换。

---

## 7. 实施任务清单

### 阶段一：卖出规则重构

| # | 任务 | 文件 | 依赖 |
|---|------|------|------|
| 1 | 重构 ExitRules/ExitRuleConfig 结构体 + 兼容反序列化 + `HoldingPosition`（加 State 字段，删 PreStopLoss 等） | `model/backtest_rules.go` | — |
| 2 | 新建 exit_checker.go：4 个接口定义 + HoldingPosition(含 GetState/SetState) + ExitDecision | `backtest/exit_checker.go` | 1 |
| 3 | 新建 exit_registry.go：注册表 + BuildExitChain + getSortedChain（类型断言调用 Hook） | `backtest/exit_registry.go` | 2 |
| 4 | 实现 StopLossChecker（实现 ExitChecker + PriorityChecker + EntryHook） | `backtest/exit_stop_loss.go` | 2, 3 |
| 5 | 实现 TakeProfitChecker（实现 ExitChecker + PriorityChecker + EntryHook） | `backtest/exit_take_profit.go` | 2, 3 |
| 6 | 实现 TimeExitChecker（实现 ExitChecker + PriorityChecker + EntryHook） | `backtest/exit_time_exit.go` | 2, 3 |
| 7 | 实现 TrailingStopChecker（实现 ExitChecker + PriorityChecker + EntryHook + DailyUpdateHook） | `backtest/exit_trailing_stop.go` | 2, 3 |
| 8 | 实现 SegmentProfitChecker（实现 ExitChecker + PriorityChecker + EntryHook，支持部分卖出） | `backtest/exit_segment_profit.go` | 2, 3 |
| 9 | 删除旧 exit_check.go | — | 4,5,6,7,8 |
| 10 | 更新 engine_helpers.go：newExitCheckerChain → BuildExitChain | `backtest/engine_helpers.go` | 3 |
| 11 | 更新 service.go：建仓/卖出流程适配（类型断言调用 Hook + 部分卖出 executePartialSell） | `backtest/service.go` | 10 |
| 12 | 更新 position_mgr.go：删除 calcEntryExitPrices（迁移到各 checker 的 OnEntry） | `backtest/position_mgr.go` | 11 |

### 阶段二：仓位管理重构

| # | 任务 | 文件 | 依赖 |
|---|------|------|------|
| 13 | 重构 PositionRules/AllocationConfig 结构体 + 兼容反序列化 | `model/backtest_rules.go` | 1 |
| 14 | 新建 allocator.go：接口 + AllocRequest/Result/CandidateInfo | `backtest/allocator.go` | — |
| 15 | 新建 alloc_registry.go：注册表 + CreateAllocator | `backtest/alloc_registry.go` | 14 |
| 16 | 实现 EqualAllocator（从 position_mgr.go 迁移） | `backtest/alloc_equal.go` | 14, 15 |
| 17 | 实现 SignalWeightedAllocator | `backtest/alloc_signal.go` | 14, 15 |
| 18 | 实现 VolatilityWeightedAllocator | `backtest/alloc_volatility.go` | 14, 15 |
| 19 | 实现 RiskParityAllocator | `backtest/alloc_risk_parity.go` | 14, 15 |
| 20 | 实现 CustomWeightAllocator | `backtest/alloc_custom.go` | 14, 15 |
| 21 | 更新 service.go：买入流程适配 Allocator 接口 | `backtest/service.go` | 16-20 |
| 22 | 更新 position_mgr.go：删除 calcAllocation（迁移到 alloc_equal.go） | `backtest/position_mgr.go` | 21 |

### 阶段三：初始化注册 + 编译验证

| # | 任务 | 文件 | 依赖 |
|---|------|------|------|
| 23 | init() 函数注册所有内置 checker 和 allocator | `backtest/init.go` (新建) | 12, 22 |
| 24 | 编译验证 + 修复所有引用 | 全项目 | 23 |
| 25 | 更新设计文档 backtest-design.md | `docs/backtest-design.md` | 24 |

---

## 附录：架构对比

### 卖出规则 — 重构前后对比

| 维度 | 重构前 | 重构后 |
|------|--------|--------|
| 核心接口 | `ExitChecker` 含 3 个方法，传 `*ExitRules` | `ExitChecker` 极简 2 方法，checker 自包含 |
| 生命周期钩子 | 无 | 可选接口 `EntryHook` + `DailyUpdateHook`，引擎类型断言调用 |
| 优先级 | 硬编码 | 可选接口 `PriorityChecker`，未实现默认 100 |
| 持仓状态 | 固定字段 PreStopLoss/PreTakeProfit/ExpiryDate | `State map[string]any`，各 checker 隔离 |
| 规则注册 | 硬编码 `newExitCheckerChain()` | `RegisterExitChecker` + `BuildExitChain` 工厂模式 |
| 配置格式 | 固定字段 stop_loss/take_profit/time_exit | `rules[]` 数组，每条 `{type, params, priority?}` |
| 新增规则 | 改 struct + 改 chain + 改 position | 实现接口 + 注册工厂，不改现有代码 |
| 部分卖出 | 不支持 | `ExitDecision.Quantity > 0` 支持分批出场 |

### 仓位管理 — 重构前后对比

| 维度 | 重构前 | 重构后 |
|------|--------|--------|
| 接口 | 无（单一函数 calcAllocation） | `Allocator` 接口 |
| 候选上下文 | 无（只传标量） | `CandidateInfo{Code, Price, SignalScore, Volatility}` |
| 分配策略 | 仅 equal | equal / signal_weighted / volatility_weighted / risk_parity / custom_weight |
| 配置格式 | `"allocation": "equal"` | `"allocation": {"type": "equal", "params": {}}` |
| 新增策略 | 改函数加 if-else | 加一个文件 + 注册 |
