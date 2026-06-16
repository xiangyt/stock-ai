# 策略回测 — 后端全流程文档

> 从 HTTP 请求到回测完成的完整链路

---

## 目录

- [1. 总览架构](#1-总览架构)
- [2. HTTP 层 — Handler](#2-http-层--handler)
- [3. 服务层 — Service.Initiate](#3-服务层--serviceinitiate)
- [4. 回测主循环 — run](#4-回测主循环--run)
  - [4.1 交易日计算](#41-交易日计算)
  - [4.2 策略解析](#42-策略解析)
  - [4.3 股票池初始化](#43-股票池初始化)
  - [4.4 逐日循环](#44-逐日循环)
    - [Step 1: 加载当日行情 (loadDayBars)](#step-1-加载当日行情-loaddaybars)
    - [Step 2: 检查卖出 (exitCheckerChain)](#step-2-检查卖出-exitcheckerchain)
    - [Step 3: 执行卖出 (executeSell)](#step-3-执行卖出-executesell)
    - [Step 4: 检查买入 (checkBuySignals)](#step-4-检查买入-checkbuysignals)
    - [Step 5: 记录快照](#step-5-记录快照)
  - [4.5 强制清仓](#45-强制清仓)
  - [4.6 绩效计算与持久化 (finalize)](#46-绩效计算与持久化-finalize)
- [5. 卖出规则引擎](#5-卖出规则引擎)
- [6. 仓位与费用管理](#6-仓位与费用管理)
- [7. 前端轮询配合](#7-前端轮询配合)
- [8. 数据模型一览](#8-数据模型一览)

---

## 1. 总览架构

```
POST /api/v1/strategies/:id/backtest
         │
         ▼
   Handler.Initiate  (解析参数 → 调用 Service.Initiate)
         │
         ▼
   Service.Initiate  (加载策略/规则 → 写 DB 记录 → 启动 goroutine)
         │
         ▼
   Service.run       (goroutine — 异步回测主循环)
         │
         ├── getTradingDays       (计算交易日列表)
         ├── parseStrategyRules   (解析买入/卖出/仓位规则)
         ├── [逐日循环]
         │     ├── loadDayBars         (从 daily_kline 表批量加载日K)
         │     ├── exitCheckerChain    (检查止损/止盈/到期)
         │     ├── executeSell         (卖出: 费用计算 + 盈亏计算)
         │     ├── checkBuySignals     (信号评估 + 买入)
         │     └── 快照记录             (每日净值快照，实时写入 DB)
         ├── executeForceClose    (最后一天强制清仓)
         └── finalize             (CalcMetrics + 写 trades + 更新 run)
```

**关键设计决策：**

| 决策 | 说明 |
|------|------|
| 异步执行 | `Initiate` 立即返回 runID，实际回测在 goroutine 中运行 |
| 前端轮询 | 每 2 秒调 `GET /backtest/runs/:id/status` 获取进度 |
| K线数据源 | 直接从 `daily_kline` 表批量查，不经过采集器 API |
| 手续费 | 佣金（万分之2.5，可配不免五）+ 卖出印花税（0.05%） |
| 卖出优先级 | 止损 > 止盈 > 到期退出（可插拔链式调用） |
| 信号评估 | 通过 `indicator.Engine.Execute` 批量评估，并发度 10 |
| 快照写入 | 每个交易日计算完立即写入 DB（`s.dao.CreateSnapshot`），前端可实时轮询 |

---

## 2. HTTP 层 — Handler

**文件：** `internal/backtest/handler.go`

### 2.1 Initiate（发起回测）

```
POST /api/v1/strategies/:id/backtest
```

请求体：

```json
{
  "stock_pool": ["600519", "000858"],   // 股票代码列表，可为空（空=全市场）
  "start_date": "2025-01-01",
  "end_date": "2026-06-15",
  "initial_capital": 100000,
  "exit_rules_override": {             // 可选，覆盖策略默认卖出规则
    "stop_loss": { "enabled": true, "threshold_pct": -8 },
    "take_profit": { "enabled": true, "threshold_pct": 20 },
    "time_exit": { "enabled": false, "hold_days": 60 },
    "slippage_pct": 0.3
  },
  "position_rules_override": {         // 可选，覆盖仓位规则
    "max_positions": 5,
    "max_single_pct": 20,
    "allocation": "equal"
  }
}
```

处理流程（伪代码）：

```
func (h *Handler) Initiate(c *gin.Context):
    // 1. 解析请求体
    req := JSON.parse(c.Body)

    // 2. 从 URL 提取 strategyID
    strategyID := c.Param("id")

    // 3. 调用 Service.Initiate（异步启动）
    runID, err := h.svc.Initiate(
        ctx, strategyID,
        req.StockPool,
        req.StartDate, req.EndDate, req.InitialCapital,
        req.ExitRulesOverride,    // 可为 nil
        req.PositionRulesOverride  // 可为 nil
    )

    // 4. 立即返回 runID，不等待回测完成
    return JSON { code: 0, data: { run_id: runID, status: "pending" } }
```

### 2.2 其他端点

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/v1/backtest/runs/:id` | GET | 获取单次回测详情（含绩效指标） |
| `/api/v1/backtest/runs/:id/status` | GET | 获取运行状态（轮询用） |
| `/api/v1/backtest/runs/:id/trades` | GET | 获取交易明细（分页） |
| `/api/v1/backtest/runs/:id/snapshots` | GET | 获取每日净值快照 |
| `/api/v1/strategies/:id/backtest/runs` | GET | 获取策略历史回测列表 |
| `/api/v1/backtest/runs/:id` | DELETE | 删除回测记录（级联删除交易+快照） |

---

## 3. 服务层 — Service.Initiate

**文件：** `internal/backtest/service.go`

```
func (s *Service) Initiate(ctx, strategyID, stockPool, startDate, endDate,
                            initialCapital, exitOverride, positionOverride):

    // ===== 阶段 1: 加载策略 =====
    strategy := db.GetStrategyByID(strategyID)
    if strategy == nil:
        return error("策略不存在")

    // ===== 阶段 2: 解析规则 =====
    // 2.1 从策略 DB 字段解析默认规则
    exitRules     := JSON.parse(strategy.ExitRules)     // 可能为空 → 默认值
    positionRules := JSON.parse(strategy.PositionRules) // 可能为空 → 默认值

    // 2.2 如果前端传了 override，覆盖默认规则
    if exitOverride != nil:
        exitRules = *exitOverride
    if positionOverride != nil:
        positionRules = *positionOverride

    // ===== 阶段 3: 加载用户手续费配置 =====
    commissionRate := 2.5   // 默认万分之2.5
    minCommission  := true  // 默认不免五
    user := db.GetUserByID(strategy.UID)
    if user != nil:
        commissionRate = user.CommissionRate  // > 0 时生效
        minCommission  = user.MinCommission

    // ===== 阶段 4: 创建 DB 记录 =====
    run := BacktestRun{
        StrategyID:     strategyID,
        StockPool:      JSON.marshal(stockPool),  // JSON 字符串
        StartDate:      startDate,
        EndDate:        endDate,
        InitialCapital: initialCapital,
        ExitRules:      JSON.marshal(exitRules),
        PositionRules:  JSON.marshal(positionRules),
        Status:         "pending",
    }
    s.dao.CreateRun(run)

    // ===== 阶段 5: 异步启动回测主循环 =====
    go s.run(
        run.ID, strategy,
        stockPool, startDate, endDate, initialCapital,
        &exitRules, &positionRules,
        commissionRate, minCommission,
    )

    return run.ID  // 立即返回
```

**关键点：**
- `Initiate` 同步返回（约 10ms），不阻塞 HTTP 请求
- 实际的回测循环在 `go s.run(...)` 中异步执行
- `exitOverride` 和 `positionOverride` 为 `nil` 时使用策略默认值

---

## 4. 回测主循环 — run

**文件：** `internal/backtest/service.go` — `Service.run()`

```
func (s *Service) run(runID, strategy, stockPool, startDate, endDate,
                      initialCapital, exitRules, positionRules,
                      commissionRate, minCommission):

    // 防止 goroutine panic 导致进程崩溃
    defer:
        if recover():
            s.failRun(runID, "panic: " + errorMsg)

    // ================================================================
    //  阶段 1: 计算交易日列表
    // ================================================================
    tradingDays := getTradingDays(startDate, endDate)
    // 例: 2025-01-01 ~ 2025-03-31 → ["2025-01-02", "2025-01-03", ...]
    // 排除周末 + 法定节假日
    if len(tradingDays) == 0:
        s.failRun("回测区间无交易日")
        return

    // ================================================================
    //  阶段 2: 解析策略条件
    // ================================================================
    sc := parseStrategyRules(strategy)
    // sc.Conditions  = []indicator.SignalConfig  (买入信号列表)
    // sc.LogicalOp   = "and" | "or"
    // sc.ExitRules   = *model.ExitRules
    // sc.PositionRules = *model.PositionRules

    // ================================================================
    //  阶段 3: 股票池初始化
    // ================================================================
    if len(stockPool) == 0:
        // 空股票池 → 使用全市场股票（从 stocks 表加载所有代码）
        allStocks := db.LoadAllStockCodes()
        stockPool = allStocks  // ["000001", "000002", ...]

    // ================================================================
    //  阶段 4: 更新状态为 running
    // ================================================================
    s.dao.UpdateRun({ ID: runID, Status: "running" })

    // ================================================================
    //  阶段 5: 初始化运行时上下文
    // ================================================================
    rc := runContext{
        runID:          runID,
        initialCapital: initialCapital,
        cash:           initialCapital,          // 当前现金
        holdings:       {},                      // map[stockCode] -> HoldingPosition
        dailySnapshots: [],                      // 每日净值快照
        trades:         [],                      // 交易记录（内存）
        commissionRate: commissionRate,          // 手续费率
        minCommission:  minCommission,           // 不免五
    }

    // ================================================================
    //  阶段 6: 创建卖出检查链
    // ================================================================
    exitChain := newExitCheckerChain()
    // = [StopLossChecker, TakeProfitChecker, TimeExitChecker]
    // 按优先级排序：止损(1) > 止盈(2) > 到期(3)

    // ================================================================
    //  阶段 7: 逐日循环 — 回测核心
    // ================================================================
    prevEquity := initialCapital
    total := len(tradingDays)

    for i, date := range tradingDays:
        // ────────────────────────────────────────────────────────
        // Step 1: 加载当日行情
        // ────────────────────────────────────────────────────────
        bars := s.loadDayBars(date, stockPool, rc.holdings)
        // bars = map[stockCode]DayBar{ Open, High, Low, Close }

    if len(bars) == 0:
        // 无行情数据 → 记录快照并立即入库，跳过
        snap := DailySnapshot{ date, prevEquity, ... }
        s.dao.CreateSnapshot(&snap)
        rc.dailySnapshots.append(snap)
        continue

        // ────────────────────────────────────────────────────────
        // Step 2: 检查卖出
        // ────────────────────────────────────────────────────────
        for code, pos := range rc.holdings:
            bar := bars[code]
            if bar == nil:
                continue  // 该股无当日行情，跳过

            decision := exitChain.Check(pos, bar, exitRules)
            // 按优先级链检查：止损 → 止盈 → 到期
            // 返回第一个触发的 ExitDecision{Reason, Price}
            // 全部未触发返回 nil

            if decision != nil:
                s.executeSell(rc, code, pos, bar, decision, date)

        // ────────────────────────────────────────────────────────
        // Step 3: 检查买入
        // ────────────────────────────────────────────────────────
        maxPos := positionRules.MaxPositions
        shouldBuy := maxPos <= 0 || len(rc.holdings) < maxPos
        // maxPos <= 0 表示不限制持仓数

        if shouldBuy:
            // 收集未持仓的候选股
            candidates := stockPool 中不在 rc.holdings 的股票
            if len(candidates) > 0:
                s.checkBuySignals(rc, candidates, bars, sc,
                                  exitRules, positionRules, date)

        // ────────────────────────────────────────────────────────
        // Step 4: 记录每日快照
        // ────────────────────────────────────────────────────────
        marketValue := 0.0
        for code, pos := range rc.holdings:
            bar := bars[code]
            marketValue += pos.Quantity * bar.Close

        totalEquity := rc.cash + marketValue

        dayReturn := (totalEquity - prevEquity) / prevEquity * 100
        cumReturn := (totalEquity - initialCapital) / initialCapital * 100

        // 快照立即写入 DB（前端可实时轮询获取）
        snap := DailySnapshot{ ... }
        s.dao.CreateSnapshot(&snap)
        rc.dailySnapshots = append(rc.dailySnapshots, snap)
        prevEquity := totalEquity

        // ────────────────────────────────────────────────────────
        // Step 5: 进度更新（每 10% 写入 DB）
        // ────────────────────────────────────────────────────────
        progressPct := (i + 1) * 100 / total
        if progressPct % 10 == 0 || i == total - 1:
            s.dao.UpdateRun({ ID: runID, ProgressPct: progressPct })

    // ================================================================
    //  阶段 8: 最后一天强制清仓
    // ================================================================
    lastDate := tradingDays[len(tradingDays)-1]
    lastBars := s.loadDayBars(lastDate, stockPool, rc.holdings)
    for code, pos := range rc.holdings:
        bar := lastBars[code]
        s.executeForceClose(rc, code, pos, bar, lastDate)

    // ================================================================
    //  阶段 9: 计算绩效指标 + 持久化
    // ================================================================
    s.finalize(runID, rc, len(tradingDays))
```

### 4.1 交易日计算

**文件：** `internal/backtest/engine_helpers.go` — `getTradingDays()`

```
func getTradingDays(startDate, endDate):
    holidays := getHolidaySet()   // 硬编码 2025-2026 法定节假日
    days := []

    for d := startDate; d <= endDate; d += 1天:
        if d 是周六或周日:
            continue
        if holidays[d]:
            continue
        days.append(d.Format("2006-01-02"))

    return days
```

节假日列表硬编码在 `getHolidays()` 中，覆盖 2025-2026 年春节/清明/五一/端午/中秋/国庆。

### 4.2 策略解析

**文件：** `internal/backtest/engine_helpers.go` — `parseStrategyRules()`

```
func parseStrategyRules(strategy):
    sc := strategyConditions{}

    // 1. 解析买入信号（从 strategy.Conditions JSON 字段）
    sc.Conditions = JSON.parse(strategy.Conditions)
    // 例如: [{ signal_id: "ma_golden_cross", operator: ">", params: {...} }]

    // 2. 解析卖出规则（从 strategy.ExitRules JSON 字段）
    sc.ExitRules = JSON.parse(strategy.ExitRules)
    // { stop_loss: {...}, take_profit: {...}, time_exit: {...} }

    // 3. 解析仓位规则（从 strategy.PositionRules JSON 字段）
    sc.PositionRules = JSON.parse(strategy.PositionRules)
    // { max_positions: 5, max_single_pct: 20, allocation: "equal" }

    return sc
```

### 4.3 股票池初始化

```go
if len(stockPool) == 0 {
    allStocks := db.LoadAllStockCodes()
    stockPool = make([]string, len(allStocks))
    for i, st := range allStocks {
        stockPool[i] = st.Code
    }
}
```

当前端传空 `stock_pool` 时，从 `stocks` 表加载全市场股票代码。

### 4.4 逐日循环

每一天执行以下 5 步：

#### Step 1: 加载当日行情 (loadDayBars)

```
func loadDayBars(date, stockPool, holdings):

    // 1. 收集所有需要行情数据的股票代码
    allCodes := stockPool ∪ holdings.keys

    // 2. 将日期 "2025-06-15" 转换为 YYYYMMDD int → 20250615
    tradeDate := 20250615

    // 3. 一次 SQL 批量查询
    klines := SELECT * FROM daily_kline
             WHERE stock_code IN (allCodes)
               AND trade_date = tradeDate

    // 4. 价格单位转换：分 → 元
    for k := range klines:
        bars[k.StockCode] = DayBar{
            Open:  k.Open / 100,
            High:  k.High / 100,
            Low:   k.Low / 100,
            Close: k.Close / 100,
        }

    return bars
```

**关键点：**
- 一次 SQL 查询加载所有股票当日 K 线，避免 N+1
- `daily_kline` 表中价格单位是「分」，需 ÷100 转「元」

#### Step 2: 检查卖出 (exitCheckerChain)

```
for code, pos := range rc.holdings:
    bar := bars[code]
    decision := exitChain.Check(pos, bar, exitRules)

    // 链式调用：StopLoss → TakeProfit → TimeExit
    // 返回第一个触发的决策，全部未触发返回 nil

    if decision != nil:
        s.executeSell(rc, code, pos, bar, decision, date)
```

详见 [第 5 节 — 卖出规则引擎](#5-卖出规则引擎)。

#### Step 3: 执行卖出 (executeSell)

```
func executeSell(rc, code, pos, bar, decision, date):

    sellPrice := decision.Price       // 止损价/止盈价/收盘价
    sellAmount := sellPrice * pos.Quantity

    // ─── 费用计算 ───
    commission := calcCommission(sellAmount, rate, minCommission)
    stampTax   := calcStampTax(sellAmount)

    // ─── 更新现金 ───
    rc.cash += sellAmount - commission - stampTax

    // ─── 计算盈亏（含手续费）───
    entryAmount := pos.EntryPrice * pos.Quantity
    profitLoss  := sellAmount - entryAmount - commission - stampTax
    profitLossPct := profitLoss / entryAmount * 100

    // ─── 记录交易 ───
    rc.trades.append(BacktestTrade{
        StockCode:     code,
        TradeType:     2,           // 卖出
        Quantity:      pos.Quantity,
        Price:         sellPrice,
        Amount:        sellAmount,
        Commission:    commission,
        StampTax:      stampTax,
        ExitReason:    &decision.Reason,
        ProfitLoss:    &profitLoss,
        ProfitLossPct: &profitLossPct,
    })

    // ─── 移除持仓 ───
    delete(rc.holdings, code)
```

**盈亏公式：**

```
盈亏金额 = 卖出金额 - 买入金额 - 卖出佣金 - 卖出印花税
盈亏%   = 盈亏金额 / 买入金额 × 100
```

注意：**买入佣金已在买入时从 cash 扣除**，盈亏不再减买入佣金，只减卖出费用。

#### Step 4: 检查买入 (checkBuySignals)

```
func checkBuySignals(rc, candidates, bars, sc, exitRules, posRules, date):

    if len(sc.Conditions) == 0:
        return  // 无买入条件，跳过

    // ─── 1. 过滤无行情数据的候选股 ───
    validCandidates := []
    for code := range candidates:
        if bars[code] != nil:
            validCandidates.append(code)

    // ─── 2. 构建 StockSource（用于信号评估）───
    sources := []
    for code := range validCandidates:
        stock := db.FindStockByCode(code)          // 查 stocks 表
        src   := stocksource.NewDBStock(stock, tradeDate)
        sources.append(src)

    // ─── 3. 调用 Engine.Execute 做信号评估 ───
    results := s.engine.Execute(sources, sc.Conditions, concurrency=10)

    // 过滤出通过的股票
    passed := {}
    for r := range results:
        if r.Result == ResultPassed:
            passed[r.Code] = true

    // ─── 4. 对通过的股票执行买入 ───
    for code := range validCandidates:
        if !passed[code]:
            continue

        bar := bars[code]

        // 4.1 计算总权益（现金 + 持仓市值）
        totalEquity := rc.cash
        for pos := range rc.holdings:
            totalEquity += pos.Quantity * bars[pos.StockCode].Close

        // 4.2 计算分配金额
        allocAmount := calcAllocation(rc.cash, maxPositions,
                                      len(rc.holdings),
                                      maxSinglePct, totalEquity)
        if allocAmount <= 0:
            continue

        // 4.3 计算买入数量（取整百股）
        buyPrice := bar.Close
        buyQty   := int(allocAmount / buyPrice / 100) * 100
        if buyQty < 100:
            continue  // 不够买一手

        // 4.4 计算费用
        buyAmount   := buyPrice * buyQty
        commission  := calcCommission(buyAmount, rate, minCommission)
        totalCost   := buyAmount + commission

        if rc.cash < totalCost:
            continue  // 现金不足

        // 4.5 执行买入
        rc.cash -= totalCost
        rc.trades.append(BacktestTrade{
            StockCode:  code,
            TradeType:  1,  // 买入
            Quantity:   buyQty,
            Price:      buyPrice,
            Amount:     buyAmount,
            Commission: commission,
        })

        // 4.6 创建持仓（计算预止损价/预止盈价）
        preStop, preProfit := calcEntryExitPrices(buyPrice, exitRules, date)

        expiryDate := date
        if exitRules.TimeExit.Enabled && exitRules.TimeExit.HoldDays > 0:
            expiryDate = addTradingDays(date, holdDays)

        rc.holdings[code] = HoldingPosition{
            StockCode:     code,
            Quantity:      buyQty,
            EntryPrice:    buyPrice,
            EntryDate:     date,
            PreStopLoss:   preStop,
            PreTakeProfit: preProfit,
            ExpiryDate:    expiryDate,
        }
```

**信号评估流程：**

```
Engine.Execute(sources, conditions, concurrency)
    │
    ├── 对每只股票 source，遍历所有 conditions
    │     ├── 计算每个信号的指标值
    │     └── 判断是否满足 operator + params
    │
    ├── 按 logicalOp ("and"/"or") 汇总结果
    │     ├── "and": 全部通过才通过
    │     └── "or":  任一通过即通过
    │
    └── 返回 []EvalResult{Code, Result, Details}
```

#### Step 5: 记录快照

```
marketValue := 0.0
for code, pos := range rc.holdings:
    bar := bars[code]
    marketValue += pos.Quantity * bar.Close

totalEquity := rc.cash + marketValue

dayReturn := (totalEquity - prevEquity) / prevEquity * 100
cumReturn  := (totalEquity - initialCapital) / initialCapital * 100

rc.dailySnapshots.append(DailySnapshot{
    SnapDate:         date,
    TotalEquity:      totalEquity,
    Cash:             rc.cash,
    MarketValue:      marketValue,
    PositionCount:    len(rc.holdings),
    DailyReturn:      &dayReturn,
    CumulativeReturn: &cumReturn,
})
```

### 4.5 强制清仓

回测最后一天，所有未卖出的持仓按当日收盘价强制清仓：

```
lastDate := tradingDays[len(tradingDays)-1]
lastBars := s.loadDayBars(lastDate, stockPool, rc.holdings)

for code, pos := range rc.holdings:
    bar := lastBars[code]
    s.executeForceClose(rc, code, pos, bar, lastDate)
```

`executeForceClose` 与 `executeSell` 逻辑一致，区别是 `ExitReason = "force_close"`，执行价为当日 `bar.Close`。

### 4.6 绩效计算与持久化 (finalize)

**文件：** `internal/backtest/service.go` — `finalize()` + `internal/backtest/metrics.go`

```
func finalize(runID, rc, tradingDays):

    // ─── 1. 计算绩效指标 ───
    metrics := CalcMetrics(rc.dailySnapshots, rc.trades,
                           rc.initialCapital, tradingDays)
    // 返回 MetricsResult:
    //   TotalReturn   = (finalEquity - initialCapital) / initialCapital * 100
    //   AnnualReturn  = (finalEquity/initialCapital)^(252/tradingDays) - 1
    //   MaxDrawdown   = max( (equity - peak) / peak )
    //   SharpeRatio   = (mean_daily_return - rf_daily) / std * sqrt(252)
    //   WinRate       = 盈利交易数 / 总卖出交易数 * 100
    //   ProfitFactor  = 总盈利 / 总亏损
    //   TradeCount    = len(trades)

    // ─── 2. 统计卖出原因 ───
    stopLossCount   := count trades where exit_reason == "stop_loss"
    takeProfitCount := count trades where exit_reason == "take_profit"
    timeExitCount   := count trades where exit_reason == "time_exit"

    // ─── 3. 持久化 ───
    s.dao.CreateTrades(rc.trades)  // 交易记录批量写入（分批 500 条）
    // 注意：快照已逐日实时写入 DB，此处不再重复写入

    // ─── 4. 更新回测运行记录 ───
    s.dao.UpdateRun(BacktestRun{
        ID:              runID,
        Status:          "done",
        ProgressPct:     100,
        FinalEquity:     &finalEquity,
        TotalReturn:     &metrics.TotalReturn,
        AnnualReturn:    &metrics.AnnualReturn,
        MaxDrawdown:     &metrics.MaxDrawdown,
        SharpeRatio:     &metrics.SharpeRatio,
        WinRate:         &metrics.WinRate,
        ProfitFactor:    &metrics.ProfitFactor,
        TradeCount:      metrics.TradeCount,
        StopLossCount:   stopLossCount,
        TakeProfitCount: takeProfitCount,
        TimeExitCount:   timeExitCount,
    })
```

**失败处理：**

```
func failRun(runID, errMsg):
    s.dao.UpdateRun({
        ID:           runID,
        Status:       "failed",
        ErrorMessage: errMsg,
    })
```

---

## 5. 卖出规则引擎

**文件：** `internal/backtest/exit_check.go`

### 5.1 接口定义

```
type ExitChecker interface:
    Check(pos, bar, rules) -> ExitDecision | nil
    Priority() int
    Name() string
```

### 5.2 优先级链

```
exitCheckerChain = [StopLossChecker, TakeProfitChecker, TimeExitChecker]
优先级: 1 (止损) > 2 (止盈) > 3 (到期)
```

链式调用逻辑：

```
func (chain) Check(pos, bar, rules):
    for checker in chain:
        decision := checker.Check(pos, bar, rules)
        if decision != nil:
            return decision  // 返回第一个触发的
    return nil               // 全部未触发
```

### 5.3 止损检查 (Priority=1)

```
StopLossChecker.Check(pos, bar, rules):

    if rules.StopLoss == nil or !rules.StopLoss.Enabled:
        return nil

    if bar.Low <= pos.PreStopLoss:
        // 考虑滑点
        execPrice := pos.PreStopLoss * (1 - slippagePct/100)
        if execPrice < bar.Low:
            execPrice = bar.Low  // 不低于最低价
        return ExitDecision{ Reason: "stop_loss", Price: execPrice }

    return nil
```

### 5.4 止盈检查 (Priority=2)

```
TakeProfitChecker.Check(pos, bar, rules):

    if rules.TakeProfit == nil or !rules.TakeProfit.Enabled:
        return nil

    if bar.High >= pos.PreTakeProfit:
        return ExitDecision{ Reason: "take_profit", Price: pos.PreTakeProfit }

    return nil
```

### 5.5 到期检查 (Priority=3)

```
TimeExitChecker.Check(pos, bar, rules):

    if rules.TimeExit == nil or !rules.TimeExit.Enabled:
        return nil

    if bar.Date >= pos.ExpiryDate:
        return ExitDecision{ Reason: "time_exit", Price: bar.Close }

    return nil
```

---

## 6. 仓位与费用管理

**文件：** `internal/backtest/position_mgr.go` + `internal/backtest/engine_helpers.go`

### 6.1 预止损/止盈价计算

```
calcEntryExitPrices(entryPrice, exitRules, date):

    preStopLoss := 0.0
    if exitRules.StopLoss.Enabled:
        preStopLoss := entryPrice * (1 + stopLoss.threshold_pct/100)
        // 例: 10元买入, 止损 -8% → preStopLoss = 9.20

    preTakeProfit := 0.0
    if exitRules.TakeProfit.Enabled:
        preTakeProfit := entryPrice * (1 + takeProfit.threshold_pct/100)
        // 例: 10元买入, 止盈 20% → preTakeProfit = 12.00

    return preStopLoss, preTakeProfit
```

### 6.2 买入金额分配

```
calcAllocation(cash, maxPositions, currentHoldingCount, maxSinglePct, totalEquity):

    availableCash := cash * 0.95            // 保留 5% 缓冲
    maxSlots := maxPositions - currentHoldingCount

    if maxSlots <= 0:
        return 0

    perStockAmount := availableCash / maxPositions

    if maxSinglePct > 0:
        maxAmountPerStock := totalEquity * maxSinglePct / 100
        perStockAmount := min(perStockAmount, maxAmountPerStock)

    return round(perStockAmount, 2)
```

### 6.3 佣金计算

```
calcCommission(amount, commissionRate, minCommission):

    fee := amount * commissionRate / 10000
    // 例: 买入 10000 元, 费率 2.5(万分之2.5)
    //   → fee = 10000 * 2.5 / 10000 = 2.5 元

    if minCommission AND fee < 5.0:
        fee = 5.0  // 不免五，最低 5 元

    return round(fee, 4)
```

### 6.4 印花税

```
calcStampTax(amount):

    tax := amount * 0.0005  // 成交金额 × 0.05%
    // 例: 卖出 15000 元 → tax = 7.5 元

    return round(tax, 4)
```

---

## 7. 前端轮询配合

前端 `BacktestPage.vue` 与后端的配合时序：

```
前端                                    后端
  │
  ├── POST /strategies/:id/backtest ────→ Initiate
  │                                       │
  │ ←── { run_id: 123, status: "pending" }
  │                                       │
  │   setInterval(2s)                     │  go run() 启动
  │   │                                   │  Status → "running"
  │   ├── GET /runs/123/status ──────────→ GetRunStatus
  │   │ ←── { status: "running", progress: 30% }
  │   │                                   │  逐日循环中...
  │   ├── GET /runs/123/status ──────────→
  │   │ ←── { status: "running", progress: 60% }
  │   │                                   │
  │   ├── GET /runs/123/status ──────────→
  │   │ ←── { status: "done", progress: 100% }
  │   │                                   │  finalize() 完成
  │   │  clearInterval()
  │   │                                   │
  │   ├── GET /runs/123 ─────────────────→ GetRun (绩效指标)
  │   ├── GET /runs/123/trades ──────────→ GetTrades (交易明细)
  │   └── GET /runs/123/snapshots ───────→ GetSnapshots (净值曲线)
  │
  └── 渲染结果
```

---

## 8. 数据模型一览

### 8.1 BacktestRun（回测运行记录）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 主键 |
| strategy_id | uint64 | 策略 ID |
| stock_pool | JSON string | 股票池代码列表 |
| start_date | string | 开始日期 (YYYY-MM-DD) |
| end_date | string | 结束日期 |
| initial_capital | float64 | 初始资金 |
| final_equity | float64 | 最终权益 |
| exit_rules | JSON string | 卖出规则 |
| position_rules | JSON string | 仓位规则 |
| status | string | pending/running/done/failed |
| progress_pct | int | 进度 0-100 |
| total_return | float64 | 累计收益率 % |
| annual_return | float64 | 年化收益率 % |
| max_drawdown | float64 | 最大回撤 % |
| sharpe_ratio | float64 | 夏普比率 |
| win_rate | float64 | 胜率 % |
| profit_factor | float64 | 盈亏比 |
| trade_count | int | 总交易次数 |
| stop_loss_count | int | 止损次数 |
| take_profit_count | int | 止盈次数 |
| time_exit_count | int | 到期退出次数 |

### 8.2 BacktestTrade（交易明细）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 主键 |
| run_id | uint64 | 关联 run |
| stock_code | string(6) | 股票代码 |
| stock_name | string | 股票名称（非DB字段，查询时填充） |
| trade_type | int8 | 1=买入 2=卖出 |
| quantity | int | 数量（股） |
| price | float64 | 成交价（元） |
| amount | float64 | 成交金额（元） |
| commission | float64 | 佣金 |
| stamp_tax | float64 | 印花税（仅卖出） |
| trade_date | DateOnly | 交易日期 |
| exit_reason | string | 卖出原因: stop_loss/take_profit/time_exit/force_close |
| profit_loss | float64 | 盈亏金额（含手续费） |
| profit_loss_pct | float64 | 盈亏百分比 |

### 8.3 DailySnapshot（每日净值快照）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 主键 |
| run_id | uint64 | 关联 run |
| snap_date | string | 快照日期 |
| total_equity | float64 | 总权益 |
| cash | float64 | 现金 |
| market_value | float64 | 持仓市值 |
| position_count | int | 持仓股票数 |
| daily_return | float64 | 日收益率 % |
| cumulative_return | float64 | 累计收益率 % |

### 8.4 HoldingPosition（内存持仓对象）

| 字段 | 类型 | 说明 |
|------|------|------|
| StockCode | string | 股票代码 |
| Quantity | int | 持仓数量 |
| EntryPrice | float64 | 买入均价 |
| EntryDate | string | 买入日期 |
| PreStopLoss | float64 | 预止损价 |
| PreTakeProfit | float64 | 预止盈价 |
| ExpiryDate | string | 到期日（时间退出用） |
| HoldDays | int | 已持天数 |

---

## 附录：文件索引

| 文件 | 职责 |
|------|------|
| `internal/backtest/handler.go` | HTTP 层：7 个端点 |
| `internal/backtest/service.go` | 核心逻辑：Initiate、run、executeSell、checkBuySignals、finalize |
| `internal/backtest/exit_check.go` | 卖出规则引擎：ExitChecker 接口 + 3 个检查器 |
| `internal/backtest/engine_helpers.go` | 辅助：交易日计算、策略解析、费用计算 |
| `internal/backtest/position_mgr.go` | 仓位管理：分配金额计算、预止损/止盈价计算 |
| `internal/backtest/metrics.go` | 绩效指标：累计收益、年化、回撤、夏普、胜率、盈亏比 |
| `internal/backtest/model.go` | 数据模型：BacktestRun、BacktestTrade、DailySnapshot、DateOnly |
| `internal/backtest/dao.go` | 数据访问：CRUD + 批量写入 |
