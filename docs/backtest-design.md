# 策略回测系统 — 完整方案设计

> 版本: v1.0 (终版) | 日期: 2026-06-14 | 状态: 待实施

---

## 目录

- [1. 设计总览](#1-设计总览)
- [2. 现状分析](#2-现状分析)
- [3. 需求优先级](#3-需求优先级)
- [4. 策略模型扩展](#4-策略模型扩展)
- [5. 日K 卖出判定模型](#5-日k-卖出判定模型)
- [6. 回测引擎设计](#6-回测引擎设计)
- [7. 数据模型设计](#7-数据模型设计)
- [8. API 设计](#8-api-设计)
- [9. 前端设计](#9-前端设计)
- [10. 扩展性设计](#10-扩展性设计)
- [11. 实施计划](#11-实施计划)
- [附录](#附录)

---

## 1. 设计总览

### 1.1 系统定位

将现有**选股信号系统**升级为**完整交易系统**，填补选股之外的三个维度：

```
选股信号(已有)            本期补全                   未来扩展
┌──────────────┐      ┌──────────────────┐      ┌──────────────┐
│ Conditions   │      │ ExitRules        │      │ 信号驱动卖出    │
│ (买入条件)    │  +   │  止损 / 止盈      │  +   │ 参数优化       │
│ LogicalOp    │      │  时间退出          │      │ 多策略对比      │
└──────────────┘      │  信号卖出(预留)     │      │ 场景分析       │
                      ├──────────────────┤      └──────────────┘
                      │ PositionRules    │
                      │  最大持仓 / 仓位上限 │
                      │  等权分配          │
                      └──────────────────┘
```

### 1.2 核心技术决策速览

| 决策点 | 方案 | 理由 |
|--------|------|------|
| 策略模型扩展方式 | 策略表新增 2 个 JSON 字段 | 不拆表、免 DDL 扩展、原子更新、前后端一致 |
| 卖出判定数据 | 日K OHLC（最低/最高价） | 现有最小粒度，无盘中分时数据 |
| 止损判定 | 最低价 <= 预止损价 | 捕获最坏情况，不做乐观假设 |
| 止盈判定 | 最高价 >= 预止盈价 | 捕获最好情况，限价单逻辑 |
| 止损执行价 | 预止损价 × (1 - slippage) | 市价单有滑点，不低于当日最低价 |
| 止盈执行价 | 预止盈价本身 | 限价单，到了就成交 |
| 同日双触发 | 止损优先 | 风险控制 > 利润保护 |
| 卖出规则架构 | ExitChecker 接口 + 优先级链 | 可插拔，未来扩展只需加一个实现 |
| 手续费 | 复用现有持仓系统费率模型 | 佣金万分之2.5 + 卖出印花税0.1%，MinCommission可配 |
| 执行方式 | 异步 goroutine + 状态机轮询 | 长耗时任务不阻塞 HTTP |

---

## 2. 现状分析

### 2.1 各层状态

| 层级 | 当前状态 | 关键发现 |
|------|----------|----------|
| **前端** | 有壳无实 | `BacktestPage.vue` 770 行，7 Tab 全部硬编码假数据；`runBacktest()` 仅 setTimeout 模拟 |
| **API** | 不存在 | 策略路由仅 9 个 CRUD 端点，无任何 `/backtest` 路径 |
| **业务逻辑** | 不存在 | 无引擎、无逐日遍历、无持仓模拟、无绩效计算 |
| **数据模型** | 仅标记字段 | 策略表有 `backtest_count`/`last_run_at`；无回测结果表 |
| **可复用组件** | 部分就绪 | 见下节 |

### 2.2 可复用基础设施

| 组件 | 路径 | 复用方式 |
|------|------|----------|
| `indicator.Engine.Execute()` | `internal/indicator/` | 无状态信号评估，直接作为每日买入信号的评判器 |
| K 线适配器 | `internal/adapter/{eastmoney,ths,tencentstock}/` | OHLCV 历史数据，日/周/月/年多周期 |
| `ScreenService.BuildAll()` | `internal/service/screen_service.go` | 构建 `[]StockSource` 数据源快照，可加历史日期参数 |
| 手续费模型 | `internal/model/portfolio.go` | `commission_rate`(万分之2.5) + `MinCommission` + A股卖出印花税0.1% |
| 交易日判断 | `utils/trading.go` | `IsTradingDay()` / 节假日列表，回测引擎复用 |
| 前端组件 | `BacktestPage.vue` / `StrategyBuilder.vue` | 策略选择下拉、日期选择器、SVG 图表渲染均可复用 |

### 2.3 信号系统的现状

现有 73 个信号（technical/market/financial/fundamental），全部定位为**选股筛选条件**，不分买卖方向。信号库中实际包含大量卖出倾向信号（MACD 死叉/顶背离、KDJ 死叉/顶背离、RSI 超买…），但策略模型将它们全部塞在 `Conditions` 数组里，用 `LogicalOp` 统一连接，无买入/卖出语义区分。

**本期不触动信号体系**——买入侧继续用现有 `Conditions`，卖出侧本期只做参数化规则（阈值/时间），信号卖出留到 P2。

---

## 3. 需求优先级

### 3.1 P0 — 核心骨架

> 必须实现，否则回测系统不可用

| # | 模块 | 内容 | 依赖 |
|---|------|------|------|
| P0-1 | **策略模型扩展** | ExitRules + PositionRules 两个 JSON 字段 + Go 结构体 + DDL | 无 |
| P0-2 | **卖出规则引擎** | 固定止损、固定止盈、时间到期退出（本期）；信号卖出字段预留 | P0-1 |
| P0-3 | **仓位管理** | 最大持仓数限制、单票仓位上限、等权分配 | P0-1 |
| P0-4 | **回测引擎** | 逐日循环：拉K线→检查卖出→检查买入→记录快照 | P0-2, P0-3 |
| P0-5 | **数据模型** | `backtest_runs` / `backtest_trades` / `daily_snapshots` 三表 | MySQL |
| P0-6 | **绩效指标** | 累计收益/年化收益/最大回撤/夏普比率/胜率/盈亏比 | P0-4 |
| P0-7 | **回测 API** | POST 发起(异步) + GET 状态/详情/交易/快照/历史 | P0-4, P0-5 |
| P0-8 | **前端接入** | 替换 BacktestPage 全部 Mock 数据 + 新增规则配置 UI | P0-7 |

### 3.2 P1 — 增强功能

> 让回测结果有参考价值

| # | 模块 | 内容 |
|---|------|------|
| P1-1 | **基准指数对比** | 回测同步拉取沪深300/中证500同期数据，曲线叠加 + Alpha/Beta 计算 |
| P1-2 | **手续费完整模拟** | 买入扣佣金、卖出扣佣金+印花税，复用现有费率模型 |
| P1-3 | **卖出原因统计** | 每笔卖出标注触发原因，支持按原因看胜率/盈亏比/占比 |
| P1-4 | **回测历史管理** | 策略详情页展示回测历史列表，支持加载/对比/删除 |
| P1-5 | **报告导出** | 生成 HTML 报告（含图表、交易清单、绩效总结），后续可扩展 PDF |

### 3.3 P2 — 高级功能

> 锦上添花，未来迭代

| # | 模块 | 内容 |
|---|------|------|
| P2-1 | **信号驱动卖出** | 利用现有 Signal 体系 + ExitConditions，实现信号型卖出策略 |
| P2-2 | **参数优化/网格搜索** | 定义参数可调范围，自动多组回测，排序展示最优 |
| P2-3 | **多策略对比** | 同屏叠加多条净值曲线，并排对比指标表 |
| P2-4 | **场景分析** | 分牛市/熊市/震荡市分别回测，评估策略环境适应性 |
| P2-5 | **交互式可视化** | K 线图标注买卖点、可缩放拖拽时间轴、点击交易跳转当日K线 |

---

## 4. 策略模型扩展

### 4.1 选型：JSON 字段 vs 拆表

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A. 策略表新增 2 个 JSON 字段** | 不拆表、免 DDL 扩展新规则类型、单条 UPDATE 原子替换、前后端同构序列化 | JSON 无法建索引（回测不需要） |
| B. 新建 exit_rules / position_rules 独立表 | 可独立建索引、关系型查询 | 策略与规则 1:1，拆表无收益；规则类型多样，拆列会导致列爆炸 |

**选择方案 A**——回测场景中规则与策略是 1:1 绑定，JSON 字段更简洁且扩展性更好。

### 4.2 DDL

```sql
-- 策略表新增两个 JSON 字段（不拆表、不改现有逻辑）
ALTER TABLE strategies
    ADD COLUMN exit_rules     JSON COMMENT '卖出规则集: {stop_loss, take_profit, time_exit, exit_signals, slippage_pct}',
    ADD COLUMN position_rules JSON COMMENT '仓位管理: {max_positions, max_single_pct, allocation}';

-- 历史数据设空对象兜底
UPDATE strategies
SET exit_rules = '{}', position_rules = '{}'
WHERE exit_rules IS NULL;
```

### 4.3 exit_rules Schema

```jsonc
{
  "stop_loss": {
    "enabled": true,          // 是否启用止损
    "threshold_pct": -8.0     // 负数百分比, 0 表示禁用
  },
  "take_profit": {
    "enabled": true,          // 是否启用止盈
    "threshold_pct": 20.0     // 正数百分比, 0 表示禁用
  },
  "time_exit": {
    "enabled": true,          // 是否启用时间退出
    "hold_days": 60           // 最大持有交易日数, 0 表示禁用
  },
  "exit_signals": [],         // [P2预留] 信号卖出条件, 本期始终为空数组
  "slippage_pct": 0.3         // 滑点百分比(仅对止损生效)
}
```

### 4.4 position_rules Schema

```jsonc
{
  "max_positions": 5,         // 最大同时持仓数, 0=不限制
  "max_single_pct": 20.0,     // 单票最多占总资产比例(%), 0=不限制
  "allocation": "equal"       // 资金分配: "equal"(等权) | [P2预留]"signal_strength"
}
```

### 4.5 Go 结构体

```go
// =========================== 卖出规则 ===========================

// ExitRules 卖出规则集（JSON 持久化到 strategies.exit_rules）
type ExitRules struct {
    StopLoss    *StopLossRule   `json:"stop_loss"`
    TakeProfit  *TakeProfitRule `json:"take_profit"`
    TimeExit    *TimeExitRule   `json:"time_exit"`
    ExitSignals []SignalConfig  `json:"exit_signals"` // P2 预留
    SlippagePct float64         `json:"slippage_pct"` // 默认 0.3
}

// StopLossRule 止损规则
type StopLossRule struct {
    Enabled      bool    `json:"enabled"`
    ThresholdPct float64 `json:"threshold_pct"` // 负数, 如 -8.0
}

// TakeProfitRule 止盈规则
type TakeProfitRule struct {
    Enabled      bool    `json:"enabled"`
    ThresholdPct float64 `json:"threshold_pct"` // 正数, 如 20.0
}

// TimeExitRule 时间退出规则
type TimeExitRule struct {
    Enabled  bool `json:"enabled"`
    HoldDays int  `json:"hold_days"` // 0 表示不启用
}

// =========================== 仓位管理 ===========================

// PositionRules 仓位管理规则（JSON 持久化到 strategies.position_rules）
type PositionRules struct {
    MaxPositions int     `json:"max_positions"`  // 0=不限制
    MaxSinglePct float64 `json:"max_single_pct"` // 0=不限制
    Allocation   string  `json:"allocation"`      // "equal" | 预留 "signal_strength"
}
```

### 4.6 回测请求参数支持规则覆盖

发起回测时可传入 `exit_rules_override` 局部覆盖策略默认规则，不影响策略本身存储。这在调参对比场景中很重要——同一策略可以快速用不同止损/止盈参数回测。

---

## 5. 日K 卖出判定模型

### 5.1 问题定义

日K 数据每日只有 **Open / High / Low / Close** 四个价格。有两个核心问题：

1. **用哪个价格判断触发？** 止损看最低价？止盈看最高价？还是只用收盘价？
2. **实际成交价怎么算？** 最低价可能是一瞬间的闪崩针，挂单在那个价格未必成交；最高价同理。

### 5.2 四种判定模式对比

| 模式 | 止损触发 | 止盈触发 | 成交价 | 评价 |
|------|----------|----------|--------|------|
| **A. 仅收盘价** | Close <= 止损价 | Close >= 止盈价 | 收盘价 | 过于保守，忽略大量盘中触发 |
| **B. 高低价直判** | Low <= 止损价 | High >= 止盈价 | 最低价/最高价 | 过于乐观，高估收益 |
| **C. 预卖出价 + 高低判定** | Low <= 预止损价 | High >= 预止盈价 | 预卖出价 | 逻辑清晰，但无视滑点 |
| **D. 预卖出价 + 高低判定 + 滑点** ⭐ | Low <= 预止损价 | High >= 预止盈价 | 预止损价×0.997 / 预止盈价 | **最贴近真实** |

### 5.3 终选定案：模式 D

#### 核心逻辑

建仓时即锁定三个出口，存入持仓对象，每日只需一次区间比较：

```
建仓时:
  预止损价  = 买入均价 × (1 + stop_loss_pct)    // -8% → 买入价的 92%
  预止盈价  = 买入均价 × (1 + take_profit_pct)   // +20% → 买入价的 120%
  预到期日  = 建仓日往后数 hold_days 个交易日
```

```
每日判定（优先级链，触发即停）:

  ┌─ P1 止损 ─────────────────────────────────────────┐
  │ IF day.Low <= pos.PreStopLoss                      │
  │   → 触发止损                                        │
  │   → 执行价 = PreStopLoss × (1 - slippage_pct/100)   │
  │   → 但不低于 day.Low（物理约束，不能卖得比最低还低）  │
  └────────────────────────────────────────────────────┘
                          ↓ 未触发
  ┌─ P2 止盈 ─────────────────────────────────────────┐
  │ IF day.High >= pos.PreTakeProfit                    │
  │   → 触发止盈                                        │
  │   → 执行价 = PreTakeProfit（限价单，按目标价成交）    │
  └────────────────────────────────────────────────────┘
                          ↓ 未触发
  ┌─ P3 到期 ─────────────────────────────────────────┐
  │ IF today >= pos.ExpiryDate                          │
  │   → 触发时间退出                                    │
  │   → 执行价 = day.Close（到期日按收盘价清仓）         │
  └────────────────────────────────────────────────────┘
                          ↓ 未触发
  ┌─ P4 信号卖出 ───── [P2 预留, 本期不实现] ─────────┐
  └────────────────────────────────────────────────────┘
```

#### 同日双触发处理

由于只有 OHLC 而不知道日内价格运动顺序，可能出现当日最低 -10%、最高 +25%，同时覆盖止损和止盈线。

**策略：止损优先。** 止损是风控底线，不可被止盈压制。股票当日波动剧烈到既冲高又跳水的程度，说明多空分歧极大，按止损执行更保守合理。

#### 滑点模型

| 参数 | 默认值 | 适用范围 | 为什么 |
|------|--------|----------|--------|
| `slippage_pct` | 0.3% | 仅止损 | 止损是市价单，触发后以市价卖出，成交价通常略差于触发价 |
| — | 0 | 止盈 | 止盈是限价单，挂在目标价上，到了才成交，不存在滑点 |

用户可通过 `exit_rules.slippage_pct` 调整，设为 0 则完全无滑点。对于流动性较差的股票可适当调高。

#### 手续费集成

复用项目现有费率体系：

```
买入: 佣金 = 成交金额 × commission_rate/10000
卖出: 佣金 = 成交金额 × commission_rate/10000 + 印花税 = 成交金额 × 0.1%
```

默认 `commission_rate = 2.5`（万分之 2.5），`MinCommission` 控制是否免五。回测引擎交易执行时调用现有费率计算函数。

### 5.4 伪代码

```go
// HoldingPosition 回测持仓（内存对象，非 DB 模型）
type HoldingPosition struct {
    StockCode     string
    Quantity      int
    EntryPrice    float64   // 加权平均买入价
    EntryDate     string    // 建仓日期（首个交易日）
    PreStopLoss   float64   // 预止损价 = entryPrice * (1 + stopLossPct/100)
    PreTakeProfit float64   // 预止盈价 = entryPrice * (1 + takeProfitPct/100)
    ExpiryDate    string    // 到期日
    HoldDays      int       // 已持有交易日数
}

type ExitDecision struct {
    Reason string   // "stop_loss" | "take_profit" | "time_exit" | "signal"
    Price  float64  // 执行价
}

func (pos *HoldingPosition) CheckExit(bar DayBar, rules ExitRules) *ExitDecision {
    // P1: 止损（最高优先级）
    if rules.StopLoss.Enabled && bar.Low <= pos.PreStopLoss {
        execPrice := pos.PreStopLoss * (1 - rules.SlippagePct/100)
        if execPrice < bar.Low {
            execPrice = bar.Low // 物理约束
        }
        return &ExitDecision{Reason: "stop_loss", Price: execPrice}
    }

    // P2: 止盈
    if rules.TakeProfit.Enabled && bar.High >= pos.PreTakeProfit {
        return &ExitDecision{Reason: "take_profit", Price: pos.PreTakeProfit}
    }

    // P3: 时间退出
    if rules.TimeExit.Enabled && bar.Date >= pos.ExpiryDate {
        return &ExitDecision{Reason: "time_exit", Price: bar.Close}
    }

    // P4: 信号卖出 (P2 预留)
    // if len(rules.ExitSignals) > 0 && pos.evalSignals(bar, rules.ExitSignals) {
    //     return &ExitDecision{Reason: "signal", Price: bar.Close}
    // }

    return nil
}
```

---

## 6. 回测引擎设计

### 6.1 包结构

```
internal/backtest/
    model.go       — 数据模型: BacktestRun / BacktestTrade / DailySnapshot
    dao.go         — 数据持久层
    engine.go      — 引擎主循环
    exit_check.go  — 卖出判定（预卖出价模型 + ExitChecker 接口）
    position_mgr.go— 持仓管理器（建仓/平仓/预卖出价计算）
    metrics.go     — 绩效指标计算
    service.go     — 异步执行管理（goroutine + 状态机）
    handler.go     — HTTP Handler
    benchmark.go   — [P1] 基准指数对齐
```

### 6.2 引擎主循环

```
输入:
  strategy       Strategy       (Conditions + ExitRules + PositionRules)
  stockPool      []string       (候选股票代码)
  dateRange      [start, end]   (回测区间)
  capital        float64        (初始本金)

输出:
  BacktestRun     (含绩效指标汇总)
  []BacktestTrade  (每笔交易)
  []DailySnapshot  (每日净值序列)
```

```
初始化:
  cash = capital
  holdings = {}                 // map[stockCode]*HoldingPosition
  trades = []                   // 交易记录
  dailySnapshots = []           // 每日快照
  tradingDays = getTradingDays(startDate, endDate)  // 排除周末 + A股节假日

主循环 (for each tradingDay):
  ┌─────────────────────────────────────────────┐
  │ 1. 加载行情                                    │
  │    拉取当日 stockPool + 当前持仓 的所有日K     │
  │    构建 map[stockCode]DayBar{OHLCV}           │
  ├─────────────────────────────────────────────┤
  │ 2. 检查卖出（遍历 holdings）                    │
  │    for each holdingPos:                      │
  │      decision = pos.CheckExit(bar, rules)     │
  │      if decision != nil:                     │
  │        cash += 卖出收入 (扣佣金+印花税+滑点)    │
  │        记录 BacktestTrade{exit_reason, ...}   │
  │        从 holdings 移除                       │
  ├─────────────────────────────────────────────┤
  │ 3. 检查买入                                    │
  │    if len(holdings) >= maxPositions → skip   │
  │    for each stock in stockPool (未持仓):      │
  │      if indicator.Engine.Evaluate() 通过:     │
  │        计算买入金额 = calcAllocation()          │
  │        if cash 足够:                          │
  │          买入数量 = floor(金额/价格/100)*100   │
  │          cash -= (买入金额 + 佣金)             │
  │          记录 BacktestTrade{buy, ...}         │
  │          创建 HoldingPosition{预卖出价, 到期日}  │
  │          加入 holdings                        │
  ├─────────────────────────────────────────────┤
  │ 4. 记录快照                                    │
  │    totalEquity = cash + sum(持仓×当日收盘价)    │
  │    dailyReturn = (todayEquity - yesterdayEquity) / yesterdayEquity
  │    写入 DailySnapshot                         │
  └─────────────────────────────────────────────┘

收尾:
  强制清仓: 最后一天所有未卖出持仓按收盘价清仓 (exit_reason="force_close")
  计算绩效指标 → 写入 BacktestRun
  批量持久化 trades + snapshots
```

### 6.3 买入分配算法

```
等权模式 (allocation="equal"):

  availableCash   = cash × 0.95         // 保留 5% 现金缓冲，避免精度问题
  maxSlots        = maxPositions - len(holdings)
  perStockAmount  = availableCash / maxPositions

  if max_single_pct > 0:
    maxAmountPerStock = totalEquity × max_single_pct / 100
    perStockAmount = min(perStockAmount, maxAmountPerStock)

  buyQty = floor(perStockAmount / buyPrice / 100) × 100  // A股 100 股整手
  if buyQty < 100 → 该股票现金不足以买入一手，跳过
```

### 6.4 异步执行模型

```
POST /backtest
  → 创建 BacktestRun(status=pending, progress_pct=0)
  → 返回 {run_id, status:"pending"}
  → goroutine:
      update status = "running"
      for i, day := range tradingDays:
        executeDay(day)
        progress_pct = floor((i+1) / total * 100)
        update DB (进度写入, 每 10 天一次, 不逐日写)
      finalize()
      update status = "done" | "failed"
```

前端轮询 `GET /backtest/runs/:id/status` 获取 `{status, progress_pct}`，进度达到 100% 后拉取完整结果。

---

## 7. 数据模型设计

### 7.1 backtest_runs — 回测运行记录

```sql
CREATE TABLE backtest_runs (
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    strategy_id      BIGINT NOT NULL,
    uid              BIGINT DEFAULT 0,
    stock_pool       JSON NOT NULL,                      -- ["000001","600519"]
    start_date       DATE NOT NULL,
    end_date         DATE NOT NULL,
    initial_capital  DECIMAL(20,4) NOT NULL,
    final_equity     DECIMAL(20,4),                      -- 跑完后填充
    exit_rules       JSON,                               -- 规则快照
    position_rules   JSON,                               -- 规则快照
    status           VARCHAR(20) DEFAULT 'pending',      -- pending|running|done|failed
    progress_pct     INT DEFAULT 0,                      -- 0-100
    error_message    TEXT,

    -- 绩效指标
    total_return     DECIMAL(10,4),                      -- 累计收益率(%), 如 35.28
    annual_return    DECIMAL(10,4),                      -- 年化收益率(%)
    max_drawdown     DECIMAL(10,4),                      -- 最大回撤(%), 如 -12.35
    sharpe_ratio     DECIMAL(10,4),                      -- 夏普比率
    win_rate         DECIMAL(10,4),                      -- 胜率(%)
    profit_factor    DECIMAL(10,4),                      -- 盈亏比
    trade_count      INT DEFAULT 0,                      -- 总交易次数
    stop_loss_count  INT DEFAULT 0,                      -- [P1] 止损次数
    take_profit_count INT DEFAULT 0,                     -- [P1] 止盈次数
    time_exit_count  INT DEFAULT 0,                      -- [P1] 到期退出次数

    created_at       DATETIME NOT NULL,
    updated_at       DATETIME NOT NULL,

    INDEX idx_strategy (strategy_id),
    INDEX idx_uid_status (uid, status)
);
```

### 7.2 backtest_trades — 回测交易明细

```sql
CREATE TABLE backtest_trades (
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id           BIGINT NOT NULL,
    stock_code       CHAR(6) NOT NULL,                   -- 6位纯数字代码
    trade_type       TINYINT NOT NULL,                   -- 1=买入 2=卖出
    quantity         INT NOT NULL,                       -- 数量(股)
    price            DECIMAL(14,4) NOT NULL,             -- 成交价
    amount           DECIMAL(20,4) NOT NULL,             -- 成交金额
    commission       DECIMAL(14,4) DEFAULT 0,            -- 手续费
    stamp_tax        DECIMAL(14,4) DEFAULT 0,            -- 印花税(仅卖出)
    trade_date       DATE NOT NULL,
    exit_reason      VARCHAR(20),                        -- stop_loss|take_profit|time_exit|signal|force_close
    pre_exit_price   DECIMAL(14,4),                      -- 预卖出价
    profit_loss      DECIMAL(14,4),                      -- 单笔盈亏金额
    profit_loss_pct  DECIMAL(10,4),                      -- 单笔盈亏百分比
    created_at       DATETIME NOT NULL,

    INDEX idx_run (run_id),
    INDEX idx_run_stock (run_id, stock_code),
    INDEX idx_run_date (run_id, trade_date)
);
```

### 7.3 daily_snapshots — 每日净值快照

```sql
CREATE TABLE daily_snapshots (
    id                  BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id              BIGINT NOT NULL,
    snap_date           DATE NOT NULL,
    total_equity        DECIMAL(20,4) NOT NULL,           -- 总权益 (市值+现金)
    cash                DECIMAL(20,4) NOT NULL,           -- 现金
    market_value        DECIMAL(20,4) NOT NULL,           -- 持仓总市值
    position_count      INT DEFAULT 0,                    -- 持仓数
    daily_return        DECIMAL(10,4),                    -- 当日收益率(%)
    cumulative_return   DECIMAL(10,4),                    -- 累计收益率(%)
    benchmark_value     DECIMAL(20,4),                    -- [P1] 基准净值
    created_at          DATETIME NOT NULL,

    INDEX idx_run_date (run_id, snap_date)
);
```

### 7.4 策略表扩展 DDL

```sql
-- 新增两个 JSON 字段到 strategies 表
ALTER TABLE strategies
    ADD COLUMN exit_rules     JSON COMMENT '卖出规则: {stop_loss, take_profit, time_exit, exit_signals, slippage_pct}',
    ADD COLUMN position_rules JSON COMMENT '仓位管理: {max_positions, max_single_pct, allocation}';

-- 为历史策略设置默认值
UPDATE strategies SET exit_rules = '{}', position_rules = '{}' WHERE exit_rules IS NULL;
```

---

## 8. API 设计

### 8.1 端点总览

| 方法 | 路径 | 说明 | 优先级 |
|------|------|------|--------|
| POST | `/api/v1/strategies/:id/backtest` | 发起回测(异步) | P0 |
| GET | `/api/v1/strategies/:id/backtest/runs` | 策略的回测历史列表 | P0 |
| GET | `/api/v1/backtest/runs/:id` | 回测详情(含绩效指标汇总) | P0 |
| GET | `/api/v1/backtest/runs/:id/status` | 轮询进度(供前端轮询) | P0 |
| GET | `/api/v1/backtest/runs/:id/trades` | 交易列表(分页, 含卖出原因) | P0 |
| GET | `/api/v1/backtest/runs/:id/snapshots` | 净值曲线数据(供图表绑定) | P0 |
| GET | `/api/v1/backtest/runs/:id/report` | 导出 HTML 报告 | P1 |
| DELETE | `/api/v1/backtest/runs/:id` | 删除回测记录 | P1 |
| GET | `/api/v1/backtest/benchmark` | 查询基准指数同期数据 | P1 |

### 8.2 核心请求/响应

#### POST /api/v1/strategies/:id/backtest

```jsonc
// Request
{
  "stock_pool":       ["000001", "600519", "000858", "300750", "601318"],
  "start_date":       "2025-01-02",
  "end_date":         "2026-06-13",
  "initial_capital":  100000,
  // 可选：覆盖策略默认规则（不影响策略本身存储）
  "exit_rules_override": {
    "stop_loss":    { "enabled": true, "threshold_pct": -8 },
    "take_profit":  { "enabled": true, "threshold_pct": 20 },
    "time_exit":    { "enabled": true, "hold_days": 60 },
    "slippage_pct": 0.3
  }
}

// Response
{ "code": 0, "data": { "run_id": 42, "status": "pending" } }
```

#### GET /api/v1/backtest/runs/:id

```jsonc
{
  "code": 0,
  "data": {
    "id": 42, "status": "done", "strategy_id": 1,
    "start_date": "2025-01-02", "end_date": "2026-06-13",
    "initial_capital": 100000, "final_equity": 135280.50,
    "total_return": 35.28, "annual_return": 22.15,
    "max_drawdown": -12.35, "sharpe_ratio": 1.42,
    "win_rate": 58.3, "profit_factor": 2.15, "trade_count": 47,
    "stop_loss_count": 3, "take_profit_count": 8, "time_exit_count": 4,
    "exit_rules": { "stop_loss": { "enabled": true, "threshold_pct": -8 } }
  }
}
```

#### GET /api/v1/backtest/runs/:id/snapshots

```jsonc
{
  "code": 0,
  "data": {
    "snapshots": [
      { "date": "2025-01-02", "total_equity": 100000, "daily_return": 0, "cumulative_return": 0 },
      { "date": "2025-01-03", "total_equity": 100520, "daily_return": 0.52, "cumulative_return": 0.52 }
    ],
    "benchmark": [  // [P1]
      { "date": "2025-01-02", "value": 1.000 },
      { "date": "2025-01-03", "value": 0.998 }
    ]
  }
}
```

#### GET /api/v1/backtest/runs/:id/trades?page=1&page_size=20

```jsonc
{
  "code": 0,
  "data": {
    "total": 47,
    "items": [
      {
        "id": 1, "stock_code": "600519", "trade_type": 1,
        "quantity": 100, "price": 1850.00, "amount": 185000.00,
        "commission": 4.63, "trade_date": "2025-03-15",
        "exit_reason": null, "profit_loss": null
      },
      {
        "id": 2, "stock_code": "600519", "trade_type": 2,
        "quantity": 100, "price": 1690.50, "amount": 169050.00,
        "commission": 4.23, "stamp_tax": 169.05,
        "trade_date": "2025-06-20", "exit_reason": "stop_loss",
        "pre_exit_price": 1702.00, "profit_loss": -15950.00, "profit_loss_pct": -8.62
      }
    ]
  }
}
```

---

## 9. 前端设计

### 9.1 页面改造总览

保持现有 `BacktestPage.vue` 的 7-Tab 布局，以下为改动清单：

| 区域 | 现状 | 终版 |
|------|------|------|
| 工具栏-策略选择 | 已实现 | **保持不变** |
| 工具栏-日期范围 | 已实现 | **保持不变** |
| 工具栏-本金 | 已实现 | **保持不变** |
| 工具栏-规则配置 | 无 | **新增**：折叠面板，含卖出规则+仓位管理 |
| "模拟交易"按钮 | `setTimeout(2s)` 假执行 | 调用 `POST /backtest` API |
| 状态徽章 | 假状态 | 轮询 `GET /status`，实时展示 pending→running→done |
| 收益概述 Tab | 硬编码假数字 | 绑定 API 返回的真实指标 |
| 交易详情 Tab | 5 条假数据 | 分页加载真实交易，新增"卖出原因""盈亏"列 |
| 每日持仓 Tab | "运行回测后将展示" | 真实渲染每日持仓快照 |
| 日志输出 Tab | 5 条假日志 | 展示回测关键事件(买入/卖出触发/完成) |
| 性能分析 Tab | 4 个占位卡片 | 收益直方图 + 回撤区间图(基于真实数据) |
| SVG 收益曲线 | 假坐标 | 绑定 snapshots API 真实净值序列 |

### 9.2 新增规则配置面板

```
┌─ 卖出规则 ▾ ─────────────────────────────────────────────┐
│  止损:   [✓] 启用    阈值:  [  -8  ]%                     │
│  止盈:   [✓] 启用    阈值:  [  20  ]%                     │
│  到期:   [✓] 启用    持有上限: [ 60 ] 个交易日             │
│  信号卖出: [ ] 暂不支持 (P2 开发中)                        │
│  滑点:   [  0.3  ]%  (仅对止损生效)                        │
├─ 仓位管理 ▾ ─────────────────────────────────────────────┤
│  最大持仓: [  5  ] 只     单票上限: [ 20  ]%               │
│  分配方式: [等权 ▾]                                       │
└──────────────────────────────────────────────────────────┘
```

### 9.3 交易详情表（新增列）

| 日期 | 代码 | 名称 | 方向 | 价格 | 数量 | 金额 | **卖出原因** | **盈亏** | **盈亏%** |
|------|------|------|------|------|------|------|-------------|---------|----------|
| 03-15 | 600519 | 贵州茅台 | 买入 | 1850 | 100 | 185,000 | — | — | — |
| 06-20 | 600519 | 贵州茅台 | 卖出 | 1690.5 | 100 | 169,050 | `stop_loss` | -15,950 | -8.62% |
| 04-10 | 000858 | 五粮液 | 买入 | 140 | 600 | 84,000 | — | — | — |
| 06-05 | 000858 | 五粮液 | 卖出 | 168 | 600 | 100,800 | `take_profit` | +16,800 | +20.0% |
| 05-01 | 300750 | 宁德时代 | 买入 | 200 | 500 | 100,000 | — | — | — |
| 07-01 | 300750 | 宁德时代 | 卖出 | 210 | 500 | 105,000 | `time_exit` | +5,000 | +5.0% |

卖出原因用颜色标签区分：🛑 红色=止损 / ✅ 绿色=止盈 / ⏰ 黄色=到期 / ⬜ 灰色=信号 / 🏁 蓝色=强制清仓

---

## 10. 扩展性设计

### 10.1 ExitChecker 可插拔接口

新增卖出规则只需实现接口 + 注册到优先级链：

```go
// ExitChecker 卖出规则检查器 — 可插拔扩展
type ExitChecker interface {
    // Check 返回是否触发，以及执行价
    Check(pos *HoldingPosition, bar DayBar) *ExitDecision
    // Priority 数字越小越优先检查
    Priority() int
    // Name 规则名称
    Name() string
}

// 引擎中使用优先级链
type exitCheckerChain []ExitChecker

func (c exitCheckerChain) Check(pos *HoldingPosition, bar DayBar) *ExitDecision {
    for _, checker := range c {
        if d := checker.Check(pos, bar); d != nil {
            return d
        }
    }
    return nil
}
```

现有实现：

| Checker | Priority | 本期 |
|---------|----------|------|
| `StopLossChecker` | 1 | ✓ |
| `TakeProfitChecker` | 2 | ✓ |
| `TimeExitChecker` | 3 | ✓ |
| `SignalExitChecker` | 4 | P2 预留 |

未来扩展 `TrailingStopChecker`（移动止盈）、`SegmentProfitChecker`（分段止盈），只需实现接口并插入链中。

### 10.2 exit_rules JSON 扩展路线

```
本期 v1.0             P2 v1.1               远期 v2.0
┌──────────┐      ┌──────────────┐      ┌─────────────────┐
│ 止损/止盈  │  →   │ 信号卖出        │  →   │ 移动止盈(trailing)│
│ 时间退出   │      │ exit_signals[] │      │ 条件组合卖出       │
│ 仓位管理   │      │ 卖出信号 AND/OR │      │ 多级止盈           │
└──────────┘      └──────────────┘      └─────────────────┘

全部通过 exit_rules JSON 字段扩展，无需 DDL。
```

### 10.3 规则快照

`backtest_runs.exit_rules` 和 `backtest_runs.position_rules` 在回测发起时完整拷贝当时策略的规则——确保任何时候回看历史回测，都能看到当时用了什么规则。`exit_rules_override` 覆盖后的结果也一并快照。

### 10.4 前端组件化

卖出规则面板拆分为独立子组件：

```
ExitRulePanel.vue
  ├── StopLossRule.vue      ← 本期
  ├── TakeProfitRule.vue    ← 本期
  ├── TimeExitRule.vue      ← 本期
  ├── SlippageSetting.vue   ← 本期
  ├── SignalExitRule.vue    ← P2 预留 (disabled 状态)
  └── PositionRulePanel.vue
        ├── MaxPositions.vue
        └── AllocationMode.vue
```

新增规则类型时只需加一个组件 + 在 Panel 里注册。

---

## 11. 实施计划

### 第一阶段：P0 核心骨架

| 步骤 | 内容 | 产出文件 |
|------|------|----------|
| 1 | Go 结构体 + JSON 序列化/反序列化 | `internal/model/indicator.go` (ExitRules / PositionRules) |
| 2 | DDL 变更 | `sql/backtest_ddl.sql` |
| 3 | 回测数据模型 | `internal/backtest/model.go` |
| 4 | 卖出判定引擎 | `internal/backtest/exit_check.go` |
| 5 | 回测引擎主循环 | `internal/backtest/engine.go` |
| 6 | 持仓管理器 | `internal/backtest/position_mgr.go` |
| 7 | 绩效指标计算 | `internal/backtest/metrics.go` |
| 8 | DAO 持久层 | `internal/backtest/dao.go` |
| 9 | 异步 Service | `internal/backtest/service.go` |
| 10 | HTTP Handler | `internal/backtest/handler.go` |
| 11 | 路由注册 | `internal/api/router/backtest_router.go` |
| 12 | 前端 API 层 | `web/src/api/strategies.ts` (新增方法) |
| 13 | 前端页面改造 | `web/src/components/BacktestPage.vue` (接入真实API + 新增规则面板) |

### 第二阶段：P1 增强功能

| 步骤 | 内容 |
|------|------|
| 14 | 基准指数数据拉取 + 对齐 + 曲线叠加 |
| 15 | 手续费完整模拟（买入佣金 + 卖出佣金+印花税，集成现有费率模型） |
| 16 | 卖出原因统计（按止损/止盈/到期分组统计胜率/盈亏比/次数） |
| 17 | 回测历史管理（列表/加载/对比/删除） |
| 18 | HTML 报告导出 |

### 第三阶段：P2 高级功能

| 步骤 | 内容 |
|------|------|
| 19 | 信号驱动卖出（SignalExitChecker 实现 + 前端信号选择器） |
| 20 | 参数优化/网格搜索 |
| 21 | 多策略对比 |
| 22 | 场景分析（牛/熊/震荡市分段回测） |
| 23 | 交互式可视化增强 |

---

## 附录

### A. 文件变更清单

**新增文件:**
```
internal/backtest/
    model.go
    dao.go
    engine.go
    exit_check.go
    position_mgr.go
    metrics.go
    service.go
    handler.go

internal/api/router/backtest_router.go
sql/backtest_ddl.sql
docs/backtest-design.md
```

**修改文件:**
```
internal/model/indicator.go          — 新增 ExitRules / PositionRules 类型定义
internal/api/router/router.go        — 注册 backtest 路由组
web/src/api/strategies.ts            — 新增 backtest API 方法 + TypeScript 类型
web/src/components/BacktestPage.vue  — 接入真实 API + 新增规则配置 UI 组件
web/src/components/StrategyBuilder.vue — [P2] 新增卖出规则编辑入口
```

**数据库变更:**
```sql
ALTER TABLE strategies
    ADD COLUMN exit_rules     JSON,
    ADD COLUMN position_rules JSON;

CREATE TABLE backtest_runs      ( ... );
CREATE TABLE backtest_trades    ( ... );
CREATE TABLE daily_snapshots    ( ... );
```

### B. 关键类型依赖关系

```
Strategy (Model)
  ├── Conditions: []SignalConfig   (现有, 买入)
  ├── ExitRules: ExitRules         (新增, 卖出)
  └── PositionRules: PositionRules (新增, 仓位)

BacktestRun (回测记录)
  ├── exit_rules: ExitRules        (快照)
  └── position_rules: PositionRules(快照)

Engine (引擎)
  ├── uses indicator.Engine         → 评估买入信号
  ├── uses ExitChecker chain        → 评估卖出规则
  └── uses K-line adapters          → 获取 OHLCV 数据
```

### C. 指标计算公式

| 指标 | 公式 |
|------|------|
| 累计收益率 | `(finalEquity - capital) / capital × 100` |
| 年化收益率 | `(finalEquity/capital)^(252/tradingDays) - 1` × 100 |
| 最大回撤 | `min( (equity[t] - max(equity[0..t])) / max(equity[0..t]) )` |
| 夏普比率 | `(mean(dailyReturns) - rf) / std(dailyReturns) × sqrt(252)` |
| 胜率 | `winTrades / totalTrades × 100` |
| 盈亏比 | `sum(profits) / abs(sum(losses))` |
