# 回测引擎详细文档 (v1.1)

> 版本: 2026-06-17 | 编译状态: 通过 | 包结构: types + exit + alloc 三层分离

---

## 1. 概览

回测引擎基于 **异步 goroutine 执行** + **轮询状态** 模式运行。前端发起回测请求后立即获得 `run_id`，引擎在后台 goroutine 中逐日执行回测，前端通过 polling API 查询状态和结果。

| 维度 | 值 |
|------|-----|
| 运行模式 | 异步 goroutine，通过 runContext 隔离 |
| 状态机 | pending → running → done / failed |
| 执行粒度 | 逐日（A 股交易日列表） |
| 股票池 | 前端传入或从 DB 全市场加载 |
| 并发控制 | 信号评估并发度 10；买入/卖出串行 |
| 手续费 | 佣金（万分之x）+ 印花税（卖出 0.05%） |

---

## 2. 包结构

```
internal/backtest/
├── types/                      公共类型 + 注册表（零依赖）
│   ├── exit.go                 ExitChecker / HoldingPosition / BuildExitChain + 注册表
│   └── alloc.go                Allocator / AllocRequest / CalcBuyAmount + 注册表
│
├── exit/                       卖出规则实现（依赖 types）
│   ├── stop_loss.go
│   ├── take_profit.go
│   ├── time_exit.go
│   ├── trailing_stop.go
│   └── segment_profit.go
│
├── alloc/                      仓位分配实现（依赖 types）
│   ├── equal.go
│   ├── signal.go
│   ├── volatility.go
│   ├── risk_parity.go
│   └── custom.go
│
├── exports.go                  类型别名（types → backtest 平铺，兼容引擎代码）
├── init.go                     空白导入 exit/alloc 触发注册
├── service.go                  引擎主循环 + 异步执行管理
├── engine_helpers.go           交易日 / 手续费 / 策略条件解析
├── metrics.go                  绩效指标计算
├── model.go                    DB 运行态数据模型
├── dao.go                      数据库访问层
├── handler.go                  HTTP 路由处理器
└── engine.go                   引擎辅助（单文件入口）
```

**依赖方向（无循环）：**
```
types  ←  exit, alloc  ←  backtest (via init.go 空白导入)
  ↑                          │
  └──────────────────────────┘ (exports.go 类型别名)
```

**层次职责：**

| 文件 | 职责 |
|------|------|
| `types/exit.go` | 接口定义（ExitChecker + 可选接口） + HoldingPosition + ExitDecision + DayBar + 注册表 + BuildExitChain |
| `types/alloc.go` | Allocator 接口 + AllocRequest/Result/CandidateInfo + 注册表 + CreateAllocator |
| `exit/*.go` | 5个卖出规则实现（各实现 ExitChecker + 按需实现可选接口） |
| `alloc/*.go` | 5个分配策略实现（各实现 Allocator 接口） |
| `exports.go` | 类型别名，将 types 包的类型平铺到 backtest 保持引擎代码兼容 |
| `init.go` | 空白导入 `exit` 和 `alloc` 触发子包 init() 注册 |
| `service.go` | 异步执行管理、run 方法（逐日主循环）、买入/卖出/清仓执行 |
| `engine_helpers.go` | 工具函数：交易日、手续费、序列化、策略条件解析 |
| `metrics.go` | 绩效指标计算：总收益、年化收益、最大回撤、夏普比率、胜率、盈亏比 |
| `model.go` | DB/运行态数据模型 |
| `dao.go` | 数据库访问层 |
| `handler.go` | HTTP 接口 |
| `model/backtest_rules.go` | ExitRules / PositionRules / AllocationConfig 结构体 + JSON 兼容反序列化 |

---

## 3. 运行流程

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│  frontend    │────▸│  POST /api/  │────▸│  Service.Initiate│
│  (发起回测)   │     │  v1/strategies│     │  (创建run记录+启动 │
│              │     │  /:id/backtest│     │   goroutine)     │
└──────────────┘     └──────────────┘     └────────┬─────────┘
                                                   │
                                          ┌────────▼─────────┐
                                          │    run() 主循环    │
                                          │  (goroutine 异步)  │
                                          └────────┬─────────┘
                                                   │
                          ┌───────────────────────┼───────────────────────┐
                          │                       │                       │
                    ┌─────▼─────┐          ┌─────▼─────┐          ┌─────▼─────┐
                    │  步骤1:    │          │  步骤2:    │          │  步骤3:    │
                    │  卖出检查   │          │  买入检查   │          │  记录快照   │
                    │ (holdings) │          │ (candidates│          │ (snapshot) │
                    │            │          │  + alloc)  │          │            │
                    └────────────┘          └────────────┘          └────────────┘
                                                   重复直到交易日结束

                                                     ▼
                                              ┌──────────────┐
                                              │  强制清仓     │
                                              │  计算绩效     │
                                              │  持久化结果   │
                                              └──────────────┘
```

### 3.1 逐日主循环（伪代码）

```
1. 计算交易日列表 → tradingDays[]
2. 解析策略条件 → strategyConditions{买入信号, 卖出规则, 仓位规则}
3. 构建卖出规则链 → exitChain[] (BuildExitChain)
4. 初始化 runContext{cash = initCapital, holdings = {}}
5. FOR EACH date IN tradingDays:
   a. loadDayBars(date) → 加载当日 K 线
   b. FOREACH pos IN holdings:
      - 调用 DailyUpdateHook.OnDailyUpdate(pos, bar)
      - 按优先级调用 ExitChecker.Check(pos, bar)
      - 触发 → executeSell 或 executePartialSell
   c. IF 持仓数 < MaxPositions:
      - 收集未持仓候选股
      - 信号评估 (indicator.Engine.Execute)
      - 创建 Allocator + Allocate 资金分配
      - 逐只买入 + 调用 EntryHook.OnEntry
   d. 计算权益 → 记录 DailySnapshot
6. 强制清仓（最后一天收盘价）
7. CalcMetrics → 持久化交易记录 + 更新 run 结果
```

---

## 4. 数据模型

### 4.1 运行态结构

#### runContext — 回测运行上下文

| 字段 | 类型 | 说明 |
|------|------|------|
| `runID` | uint64 | 回测运行 ID |
| `initialCapital` | float64 | 初始资金（如 100000） |
| `cash` | float64 | 当前可用现金 |
| `holdings` | map[string]*HoldingPosition | 当前持仓（stockCode → 持仓） |
| `dailySnapshots` | []DailySnapshot | 每日净值快照（内存累计） |
| `trades` | []BacktestTrade | 交易记录（内存累计，结束后批量写入） |
| `commissionRate` | float64 | 佣金率（万分之x，默认 2.5） |
| `minCommission` | bool | 是否不免五（true = 最低 5 元） |

#### HoldingPosition — 持仓

| 字段 | 类型 | 说明 |
|------|------|------|
| `StockCode` | string | 6 位股票代码 |
| `Quantity` | int | 股数 |
| `EntryPrice` | float64 | 建仓价 |
| `EntryDate` | string | 建仓日期 |
| `HoldDays` | int | 持仓天数 |
| `State` | map[string]any | **v1.1 新增**：各 checker 专属状态，key=checker.Name() |

#### ExitDecision — 卖出决策

| 字段 | 类型 | 说明 |
|------|------|------|
| `Reason` | string | 卖出原因：stop_loss / take_profit / time_exit / trailing_stop / segment_profit / force_close |
| `Price` | float64 | 执行价 |
| `Quantity` | int | **v1.1 新增**：0=整仓卖出，>0=部分卖出指定数量 |

#### DayBar — 日K线（回测简化结构）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Date` | string | 日期 YYYY-MM-DD |
| `Open` | float64 | 开盘价（元） |
| `High` | float64 | 最高价（元） |
| `Low` | float64 | 最低价（元） |
| `Close` | float64 | 收盘价（元） |

### 4.2 持久化结构

#### BacktestRun — 回测运行记录（表: backtest_runs）

| 字段 | 说明 |
|------|------|
| `ID` | 主键 |
| `StrategyID` | 策略 ID |
| `UID` | 用户 ID |
| `StockPool` | 股票池 JSON 数组 |
| `StartDate / EndDate` | 回测区间 |
| `InitialCapital` | 初始资金 |
| `FinalEquity` | 最终权益（回测完成后填充） |
| `ExitRules` | 卖出规则 JSON |
| `PositionRules` | 仓位规则 JSON |
| `Status` | pending / running / done / failed |
| `ProgressPct` | 进度百分比 |
| `TotalReturn / AnnualReturn / MaxDrawdown / SharpeRatio / WinRate / ProfitFactor` | 绩效指标 |
| `TradeCount` | 交易总数 |
| `StopLossCount / TakeProfitCount / TimeExitCount` | 各退出原因计数 |

#### BacktestTrade — 交易记录（表: backtest_trades）

| 字段 | 说明 |
|------|------|
| `RunID` | 关联 run ID |
| `TradeType` | 1=买入，2=卖出 |
| `Quantity / Price / Amount` | 数量/单价/金额 |
| `Commission / StampTax` | 佣金/印花税 |
| `ExitReason` | 卖出原因（买入为 null） |
| `ProfitLoss / ProfitLossPct` | 盈亏金额/百分比（卖出时计算） |

#### DailySnapshot — 每日快照（表: daily_snapshots）

| 字段 | 说明 |
|------|------|
| `SnapDate` | 快照日期 |
| `TotalEquity / Cash / MarketValue` | 总权益/现金/持仓市值 |
| `PositionCount` | 当日持仓数 |
| `DailyReturn / CumulativeReturn` | 日收益/累计收益 |

---

## 5. 卖出规则系统（可插拔）

### 5.1 接口体系

```
                    ExitChecker （核心接口）
                    Check(pos, bar) *ExitDecision
                    Name() string
                         │
          ┌──────────────┼──────────────┐
          │              │              │
   PriorityChecker  EntryHook    DailyUpdateHook
    Priority() int  OnEntry(...)  OnDailyUpdate(...)
    （可选，默认100） （可选）       （可选）
```

### 5.2 接口说明

| 接口 | 方法 | 何时调用 | 必须实现 |
|------|------|---------|---------|
| `ExitChecker` | `Check(pos, bar)` | 每日卖出检查 | ✅ |
| `ExitChecker` | `Name()` | 状态隔离 key | ✅ |
| `PriorityChecker` | `Priority()` | 排序执行链 | ❌（默认 100） |
| `EntryHook` | `OnEntry(pos, entryPrice, entryDate)` | 建仓时 | ❌ |
| `DailyUpdateHook` | `OnDailyUpdate(pos, bar)` | 每日 Check 前 | ❌ |
| `SlippageInjector` | `SetSlippage(pct)` | BuildExitChain 时 | ❌（仅 StopLoss） |

### 5.3 内置 Checker

#### StopLossChecker — 固定止损

| 属性 | 值 |
|------|-----|
| 实现接口 | ExitChecker + PriorityChecker(1) + EntryHook + SlippageInjector |
| params | `{"threshold_pct": -8.0}` — 负数表示跌幅 |
| OnEntry | 计算 `preStopLoss = entryPrice × (1 + threshold_pct/100)`，存入 `State["stop_loss"].preStopLoss` |
| Check | `bar.Low <= preStopLoss` → 执行价 = preStopLoss × (1 - slippage/100) |
| 特殊逻辑 | 含滑点折价：卖价 = 止损价 × (1 - 滑点%)，不低于当日最低价 |

#### TakeProfitChecker — 固定止盈

| 属性 | 值 |
|------|-----|
| 实现接口 | ExitChecker + PriorityChecker(2) + EntryHook |
| params | `{"threshold_pct": 20.0}` — 正数表示涨幅 |
| OnEntry | 计算 `preTakeProfit = entryPrice × (1 + threshold_pct/100)` |
| Check | `bar.High >= preTakeProfit` → 按止盈价限价单成交 |

#### TimeExitChecker — 到期退出

| 属性 | 值 |
|------|-----|
| 实现接口 | ExitChecker + PriorityChecker(3) + EntryHook |
| params | `{"hold_days": 60}` |
| OnEntry | 计算 `expiryDate = entryDate + hold_days`（自然日） |
| Check | `bar.Date >= expiryDate` → 按收盘价成交 |

#### TrailingStopChecker — 移动止盈（新增）

| 属性 | 值 |
|------|-----|
| 实现接口 | ExitChecker + PriorityChecker(2) + EntryHook + **DailyUpdateHook** |
| params | `{"trail_pct": 5.0, "activation_pct": 10.0}` |
| OnEntry | 初始化 `highWater = entryPrice`, `activated = false` |
| OnDailyUpdate | 更新 `highWater = max(highWater, bar.High)`；涨幅超 `activation_pct` 则 `activated = true` |
| Check | 仅激活后生效：`bar.Low <= highWater × (1 - trail_pct/100)` |
| 使用场景 | 涨到激活阈值后启动跟踪，锁定利润的同时允许继续上涨 |

#### SegmentProfitChecker — 分段止盈（新增）

| 属性 | 值 |
|------|-----|
| 实现接口 | ExitChecker + PriorityChecker(2) + EntryHook |
| params | `{"levels": [{"threshold_pct": 10, "sell_ratio": 0.5}, {"threshold_pct": 20, "sell_ratio": 0.5}]}` |
| OnEntry | 初始化 `triggered[]` 数组，`originalQty = pos.Quantity` |
| Check | 遍历未触发的 level，达到阈值则部分卖出对应比例；全部触发后清仓 |
| 部分卖出 | `ExitDecision.Quantity > 0` → 引擎调用 `executePartialSell`，减少 `pos.Quantity` 不删除持仓 |

### 5.4 扩展机制

新增卖出规则只需 3 步，不改任何现有代码：

```go
// 文件: internal/backtest/exit/my_rule.go
package exit

import "stock-ai/internal/backtest/types"

type MyChecker struct { thresholdPct float64 }

func (m *MyChecker) Check(pos *types.HoldingPosition, bar types.DayBar) *types.ExitDecision { ... }
func (m *MyChecker) Name() string { return "my_rule" }

func newMyChecker(params json.RawMessage) (types.ExitChecker, error) { ... }

func init() { types.RegisterExitChecker("my_rule", newMyChecker) }
```

可选接口按需实现：优先级 → `PriorityChecker` + `SetPriority`；建仓初始化 → `EntryHook`；每日更新 → `DailyUpdateHook`。

---

## 6. 仓位管理系统（可插拔）

### 6.1 核心接口

```go
type Allocator interface {
    Allocate(req *AllocRequest) *AllocResult
    Name() string
}
```

### 6.2 AllocRequest — 分配请求

| 字段 | 说明 |
|------|------|
| `Cash` | 可用现金 |
| `TotalEquity` | 总权益（现金 + 持仓市值） |
| `MaxPositions` | 最大持仓数（0=不限） |
| `CurrentHoldings` | 当前持仓数 |
| `MaxSinglePct` | 单票上限（如 20 = 单票不超过总资产的 20%） |
| `CashBufferPct` | 现金缓冲比例（默认 5%，每次买入最多用 95% 现金） |
| `Candidates` | 通过买入信号的候选股列表 |

### 6.3 CandidateInfo — 候选股上下文

| 字段 | 说明 |
|------|------|
| `Code` | 股票代码 |
| `Price` | 当日收盘价 |
| `SignalScore` | 信号评分（当前默认 1.0，未来可从 Engine.Execute 结果获取） |
| `Volatility` | 近 20 日年化波动率（用于 volatility_weighted / risk_parity） |

### 6.4 内置分配器

| 分配器 | type 值 | 逻辑 | 需要候选上下文 |
|--------|--------|------|--------------|
| `EqualAllocator` | `"equal"` | 等权：可用槽位数均分 | 无 |
| `SignalWeightedAllocator` | `"signal_weighted"` | 信号评分加权：评分高者多配 | SignalScore |
| `VolatilityWeightedAllocator` | `"volatility_weighted"` | 波低者多配：weight ∝ 1/vol | Volatility |
| `RiskParityAllocator` | `"risk_parity"` | 风险平价：weight ∝ 1/vol² | Volatility |
| `CustomWeightAllocator` | `"custom_weight"` | 手动设权重：params.weight | 手动配置 |

### 6.5 通用约束

所有分配器共享以下引擎层面的约束：

- **100 股整数倍**：`calcBuyAmount` 按 `100` 股取整
- **现金缓冲**：`CashBufferPct` 默认 5%（最多投入 95% 现金）
- **单票上限**：`MaxSinglePct` 约束单只股票最高持仓比例
- **槽位限制**：`MaxPositions` 限制同时持仓数（0=不限）
- **资金不足跳过**：买入成本超过可用现金时跳过该股

---

## 7. 费用计算

| 费用项 | 公式 | 适用场景 |
|--------|------|---------|
| **佣金** | `amount × (rate / 10000)` | 买入 + 卖出 |
| **最低佣金** | `max(5.0, 佣金)`（minCommission=true 时） | 买入 + 卖出 |
| **印花税** | `amount × 0.0005` | 仅卖出 |
| **滑点** | `止损价 × (1 - slippage_pct/100)` | 仅止损触发时 |

### 7.1 部分卖出时的费用

`executePartialSell` 按实际卖出数量计算佣金和印花税，盈亏按持仓均价（`EntryPrice`）计算。

---

## 8. 绩效指标

| 指标 | 公式 | 说明 |
|------|------|------|
| **总收益率** | `(finalEquity - initCapital) / initCapital × 100` | 回测区间总收益 |
| **年化收益率** | `(finalEquity/initCapital)^(252/tradingDays) - 1 × 100` | 假设每年 252 个交易日 |
| **最大回撤** | `min((equity - peak) / peak × 100)` 历史最高 | 峰值到谷底的最大跌幅 |
| **夏普比率** | `(meanReturn - rf)/stdDev × √252` | 风险调整后收益（rf=2%年化） |
| **胜率** | `盈利交易数 / 总卖出交易数 × 100` | 卖出交易中盈利的占比 |
| **盈亏比** | `总盈利 / |总亏损|` | 盈利总额与亏损总额之比 |

---

## 9. JSON 配置格式

### 9.1 卖出规则 (ExitRules)

```jsonc
{
  "rules": [
    {
      "type": "stop_loss",
      "enabled": true,
      "params": { "threshold_pct": -8.0 },
      "priority": 1
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
      "enabled": false,
      "params": { "trail_pct": 5.0, "activation_pct": 10.0 }
    },
    {
      "type": "segment_profit",
      "enabled": false,
      "params": {
        "levels": [
          { "threshold_pct": 10, "sell_ratio": 0.5 },
          { "threshold_pct": 20, "sell_ratio": 0.5 }
        ]
      }
    }
  ],
  "slippage_pct": 0.3
}
```

**兼容旧格式**（v1.0）：引擎通过 `UnmarshalJSON` 自动转换：
```jsonc
{ "stop_loss": { "enabled": true, "threshold_pct": -8.0 }, "take_profit": ... }
```

### 9.2 仓位规则 (PositionRules)

```jsonc
{
  "max_positions": 5,
  "max_single_pct": 20,
  "allocation": { "type": "equal", "params": {} },
  "cash_buffer_pct": 5
}
```

**分配器特定格式：**

```jsonc
{ "allocation": { "type": "signal_weighted", "params": {} } }
{ "allocation": { "type": "volatility_weighted", "params": {} } }
{ "allocation": { "type": "risk_parity", "params": {} } }
{ "allocation": { "type": "custom_weight", "params": { "weights": { "000001": 1.0, "600519": 1.5 } } } }
```

**兼容旧格式**：`"allocation": "equal"` 自动转换为 `{ "type": "equal", "params": {} }`

---

## 10. API 接口

### 发起回测

```
POST /api/v1/strategies/:id/backtest
```
```jsonc
{
  "stock_pool": ["000001", "600519"],
  "start_date": "2024-01-01",
  "end_date": "2024-12-31",
  "initial_capital": 100000,
  "exit_rules_override": null,        // 可选：覆盖策略默认卖出规则
  "position_rules_override": null     // 可选：覆盖策略默认仓位规则
}
```
响应: `{"code": 0, "data": {"run_id": 1, "status": "pending"}}`

### 查询状态

```
GET /api/v1/backtest/runs/:id/status
```
响应: `{"code": 0, "data": {"status": "running", "progress_pct": 45}}`

### 获取结果

```
GET /api/v1/backtest/runs/:id
```
响应包含所有绩效指标：total_return / annual_return / max_drawdown / sharpe_ratio / win_rate / profit_factor

### 获取交易记录

```
GET /api/v1/backtest/runs/:id/trades?page=1&page_size=20
```

### 获取净值曲线

```
GET /api/v1/backtest/runs/:id/snapshots
```

### 获取历史运行

```
GET /api/v1/strategies/:id/backtest/runs?limit=20
```

### 删除运行

```
DELETE /api/v1/backtest/runs/:id
```

---

## 11. 文件清单

```
internal/backtest/
├── types/                            公共类型 + 注册表（零依赖）
│   ├── exit.go                       ExitChecker/HoldingPosition + BuildExitChain + 注册表
│   └── alloc.go                      Allocator/AllocRequest + CalcBuyAmount + 注册表
│
├── exit/                             卖出规则实现（依赖 types）
│   ├── stop_loss.go                  StopLossChecker
│   ├── take_profit.go                TakeProfitChecker
│   ├── time_exit.go                  TimeExitChecker
│   ├── trailing_stop.go              TrailingStopChecker（新增 v1.1）
│   └── segment_profit.go             SegmentProfitChecker（新增 v1.1，支持部分卖出）
│
├── alloc/                            仓位分配实现（依赖 types）
│   ├── equal.go                      EqualAllocator
│   ├── signal.go                     SignalWeightedAllocator（新增 v1.1）
│   ├── volatility.go                 VolatilityWeightedAllocator（新增 v1.1）
│   ├── risk_parity.go                RiskParityAllocator（新增 v1.1）
│   └── custom.go                     CustomWeightAllocator（新增 v1.1）
│
├── exports.go                        类型别名（types → backtest 平铺，兼容引擎代码）
├── init.go                           空白导入 exit/alloc 触发注册
├── service.go                        (修改 v1.1) 引擎主循环 + 买入/卖出流程 + executePartialSell
├── engine_helpers.go                 (修改 v1.1) 交易日/手续费/策略条件解析
├── metrics.go                        绩效指标计算
├── model.go                          DB/运行态数据模型
├── dao.go                            数据库访问层
├── handler.go                        HTTP 路由处理器
└── engine.go                         引擎辅助

internal/model/
└── backtest_rules.go                 (修改 v1.1) ExitRules/PositionRules/AllocationConfig + JSON 兼容

已删除 (v1.1):
├── exit_check.go                     旧接口/checker 定义（迁移至 types/exit.go + exit/*.go）
├── exit_stop_loss.go ~ segment_profit.go  迁移至 exit/
├── exit_checker.go / exit_registry.go    迁移至 types/
├── allocator.go / alloc_registry.go      迁移至 types/
├── alloc_equal.go ~ alloc_custom.go      迁移至 alloc/
└── position_mgr.go                   旧函数已迁移至 checker/allocator
```
