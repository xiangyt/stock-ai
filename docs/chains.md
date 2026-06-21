# 全模块链路文档

> 生成时间: 2026-06-21

---

## 模块总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                           main.go（组装层）                           │
│                                                                     │
│  QuoteCache.Start() ─→ ReloadAll() ─→ 各组件启动 ─→ 回调注入           │
└─────────────────────────────────────────────────────────────────────┘
         │                  │                 │              │
    ┌────▼─────┐     ┌──────▼──────┐   ┌──────▼─────┐ ┌────▼─────┐
    │quotecache│     │ watchlist   │   │  Monitor   │ │ Portfolio│
    │ 行情缓存  │◄────│ 关注列表管理  │   │  盯盘助手    │ │ 持仓管理  │
    └────┬─────┘     └─────────────┘   └──────┬─────┘ └──────────┘
         │                                    │
         └─────── Subscribe / Notify ─────────┘
```

---

## 链 A：持仓生命周期

```
POST /api/v1/positions
  │
  ▼
PortfolioHandler.OpenPosition
  │
  ▼
PortfolioService.OpenPosition(req, uid)
  ├─ db.CreatePosition(position)                    ← 写入 positions 表
  ├─ db.CreateTrade(trade)                         ← 写入 position_trades 表
  │
  ├─ watchlistMgr.OnPositionOpened(uid, code)       ← ① 同步关注列表
  │   └─ userPositions[uid] += code
  │   └─ incRefLocked(code): refCount 0→1
  │       └─ cache.SetPriority([code], PriorityHigh)  ← quotecache 标记
  │
  └─ notifyHoldingChanged()                         ← ② 通知 Monitor
      └─ Monitor.NotifyHoldingChanged()
          └─ changeCh ← ConfigChange{HoldingChanged}
              └─ configLoop → syncWorkers(unionCodes())
                  └─ subscribe(new codes) via quotecache

  ═══════════════ 后续：quotecache 实时刷新 ═══════════════

quotecache scheduler（每 1 分钟 :05）
  │
  ├─ refreshPriority(PriorityHigh, 0)  ← 全并发
  │   └─ for each code:
  │       └─ refreshOne(code)
  │           ├─ collector.Collect(ctx, code)        ← 请求腾讯股票 API
  │           │   └─ adapter.GetIntraday(code)
  │           │       └─ HTTP → IntradayData（分时bar+盘口+PE/PB）
  │           ├─ 更新 items[code].Intraday
  │           ├─ data.Invalidate()  ← 清空日线缓存
  │           └─ notifySubscribers(code, data)
  │               └─ buildEvent(data) → QuoteEvent
  │                   └─ for each subscriber ch → ch ← QuoteEvent

Monitor.codeWorker（独立 goroutine，每 code 一个）
  │
  └─ quotaEvent ← ch
      └─ processEvent(event)
          ├─ utils.IsTradingHours() ? continue : 丢弃
          ├─ codeConfigs[code] → 获取该 code 的所有监控配置
          ├─ for each config:
          │   ├─ cfg.ParseRule()             → MonitorRule（涨跌幅/急拉急跌/量比/封板）
          │   ├─ cfg.ParseCooldown()         → 冷却配置
          │   └─ checker.Check(rule, data)   → []Alert
          │       └─ for each alert:
          │           ├─ cooldown.ShouldAlert() ? 检查冷却
          │           ├─ push.Build(cfg, alert, data)  → 消息文本
          │           └─ db.GetMonitorConfigBots(cfg.ID)
          │               └─ notifier.Send(bot, msg)   → 钉钉/飞书/企微推送

  ═══════════════ 清仓 / 删除 ═══════════════

PortfolioService.ClosePosition(id, uid)
  ├─ db.UpdatePosition → quantity=0, status=closed
  ├─ db.CreateTrade(trade)
  │
  ├─ watchlistMgr.OnPositionClosed(uid, code)
  │   └─ userPositions[uid] -= code
  │   └─ decRefLocked(code): refCount 1→0
  │       └─ cache.SetPriority([code], PriorityLow)   ← 降级
  │
  └─ notifyHoldingChanged()
      └─ Monitor.NotifyHoldingChanged()
          └─ syncWorkers(unionCodes())  ← 移除不再监控的 code

PortfolioService.BuyMore(id, uid)   ← 加仓：只设 PriorityHigh，不改变监控范围
PortfolioService.SellPartial(id)    ← 减仓不归零：不触发任何变更
```

---

## 链 B：盯盘配置生命周期

```
POST /api/v1/monitor-configs
  │
  ▼
MonitorConfigHandler.Create
  │
  ▼
MonitorConfigService.Create(req, uid)
  ├─ db.CreateMonitorConfig(cfg)                     ← 写入 monitor_configs
  ├─ db.SetMonitorConfigBots(cfg.ID, botIDs)         ← 写入 monitor_config_bots
  │
  ├─ s.notifyChange(ChangeCreated, cfg.ID)            ← ① 通知 Monitor 引擎
  │   └─ main.go 闭包
  │       └─ Monitor.NotifyChange(ConfigChange{Created, cfgID})
  │           └─ changeCh
  │               └─ configLoop
  │                   ├─ 重新从 DB 加载配置
  │                   ├─ 更新 m.configs / m.codeConfigs 索引
  │                   └─ syncWorkers(unionCodes())
  │                       ├─ 移除旧 worker，取消 subscribe
  │                       └─ 新增 worker:
  │                           ├─ ch, unsub = m.subscribe([code])
  │                           │   └─ quotecache.Subscribe([code])
  │                           │       └─ 注册到 codeToSubs 反向索引
  │                           └─ go processCode(code, ch)  ← 启动监听
  │
  └─ s.notifyWatchlist(uid)                           ← ② 同步关注列表
      └─ watchlistMgr.OnMonitorChanged(uid)
          ├─ old = copySet(userMonitors[uid])
          ├─ new = loadUserMonitorCodes(uid)          ← 从 DB 加载当前 code
          │   └─ db.GetAllActiveMonitorConfigs().filter(uid)
          │       └─ resolveScopeCodes(scope, uid, stocksJSON)
          │           ├─ ScopeHeld → db.GetAllActiveStockCodes(uid)
          │           └─ ScopeCustom → json.Unmarshal(Stocks)
          └─ diffUpdate(old, new)
              ├─ 移除的 code → decRefLocked → 0 时 PriorityLow
              └─ 新增的 code → incRefLocked → 0→1 时 PriorityHigh

PUT /api/v1/monitor-configs/:id     → Update → ChangeUpdated   + notifyWatchlist
DELETE /api/v1/monitor-configs/:id  → Delete → ChangeDeleted   + notifyWatchlist
PATCH .../:id/active                → SetActive → ChangeEnabled/Disabled + notifyWatchlist
PUT .../:id/bots                    → UpdateBots → 不触发任何变更
```

---

## 链 C：策略订阅生命周期

```
POST /api/v1/subscriptions
  │
  ▼
SubscriptionHandler.Create
  │
  ▼
SubscriptionService.Create(req, uid)
  ├─ db.CreateSubscription(sub)                      ← 写入 strategy_subscriptions
  ├─ db.SetSubscriptionBots(subID, botIDs)           ← 写入 subscription_bots
  │
  ├─ s.notifyChange(ChangeCreated, sub.ID)            ← ① 通知 Scheduler
  │   └─ main.go 闭包
  │       └─ Scheduler.NotifyChange(SubscriptionChange{Created, subID})
  │           └─ changeCh
  │               └─ changeLoop
  │                   └─ addTask(task) → cron.AddFunc(expr, runTask)
  │
  └─ s.notifyWatchlist(uid)                           ← ② 同步关注列表
      └─ watchlistMgr.OnSubscriptionChanged(uid)
          ├─ old = copySet(userSubscriptions[uid])
          ├─ new = loadUserSubscriptionCodes(uid)
          └─ diffUpdate → incRef/decRef → PriorityHigh/Low

  ═══════════════ Cron 触发 ═══════════════

Scheduler cron（如 "0 */30 * * * *" 每 30 分钟）
  │
  └─ runTask(subID)
      ├─ db.GetDataCollectTaskByID → 检查 isActive
      ├─ db.GetDataCollectBots(taskID)
      └─ runner.Run(ctx, task, bots)
          │
          ▼
SubscriptionRunner.Run(ctx, sub)
  ├─ resolveStockCodes(sub.Scope, sub.UID, sub.CustomStocks)
  │   ├─ ScopeAll → db.LoadAllStockCodes()               ← 全市场
  │   ├─ ScopeHeld → db.ListPositions(uid, "holding")    ← 持仓股
  │   └─ ScopeCustom → json.Unmarshal(CustomStocks)      ← 自选股
  │
  ├─ provider.BuildStockSources(ctx, codes)
  │   └─ CachedStock per code
  │       └─ GetDailyKline / GetDailySnapshot
  │           └─ cache.Get(ctx, code)                     ← 从 quotecache 拉取
  │               ├─ 命中 → 返回缓存数据
  │               └─ 未命中 → fetch(code) → collector.Collect
  │                   └─ 写入缓存 → 返回
  │
  ├─ engine.Execute(strategyID, stockSources)              ← 执行指标计算
  │   └─ 指标信号 → 排序/筛选 → 选股结果
  │
  └─ notifier.Send(bots, result)                          ← 推送结果
```

---

## 链 D：每日优先级重置

```
DataCollect cron "0 0 8 * * *"（每日 8:00）
  │
  ▼
DataCollectRunner.handleReloadHighPriority
  │
  └─ watchlistMgr.ReloadAll()
      ├─ mu.Lock()
      ├─ posUsers = allTrackedUserIDs()         ← 快照当前用户
      ├─ cache.ResetPriorities()                ← 清空 quotecache priority map
      ├─ 重置 userPositions / userMonitors / userSubscriptions / refCount
      │
      ├─ 加载全平台活跃 MonitorConfig → resolveScopeCodes → 重建 userMonitors
      ├─ 加载全平台活跃 Subscription   → resolveScopeCodes → 重建 userSubscriptions
      ├─ 遍历所有用户 → db.GetAllActiveStockCodes(uid) → 重建 userPositions
      │
      └─ 遍历 refCount → cnt > 0 的 code → SetPriority(highCodes, PriorityHigh)
          └─ mu.Unlock()
```

---

## 链 E：quotecache 调度引擎

```
scheduler()  goroutine（每分钟 :05 秒对齐）
  │
  ├─ IsActive() == false（非交易时段）
  │   └─ 9:00 →
  │       ├─ lowExecuted = 重置
  │       └─ cleanStale()  ← 清空 items 缓存（priority map 保留）
  │
  ├─ IsActive() == true（交易时段 9:15-11:30, 13:00-15:00）
  │   │
  │   ├─ 每分钟：refreshPriority(PriorityHigh, 0)     ← 全并发，无限制
  │   │   └─ getCodesByPriority(High)
  │   │       ├─ priority map 中 PriorityHigh 的 code
  │   │       └─ items 中 priority 字段 = High 的 code（兜底）
  │   │   └─ for each code → go refreshOne(code)      ← 全并发
  │   │
  │   ├─ 每 5 分钟（min%5==0）：refreshPriority(PriorityNormal, 50)
  │   │   └─ semaphore(50) 并发控制
  │   │
  │   └─ 定时 3 次：refreshPriority(PriorityLow, 100)
  │       ├─ 10:30（minOfDay=630）
  │       ├─ 12:00（minOfDay=720）
  │       └─ 14:00（minOfDay=840）
  │       └─ semaphore(100)，每天只执行一次
  │
  └─ 9:00 → 重置 lowExecuted 标记

  ═══════════════ refreshOne(code) 细节 ═══════════════

refreshOne(code)
  ├─ 检查 TTL：time.Since(fetchedAt) < TTL ? return
  │   ├─ PriorityHigh TTL = 60s
  │   ├─ PriorityNormal TTL = 300s
  │   └─ PriorityLow TTL = 900s
  │
  ├─ collector.Collect(ctx, code)  ← 请求数据源
  │   └─ collectorAdapter.GetIntraday(code)
  │       ├─ 优先：adapter.GetIntraday(code) → IntradayData{Bars, Depth, PE, ...}
  │       └─ 降级：adapter.GetDaily(code) → 构造单 bar Intraday
  │
  ├─ 更新 items[code]：
  │   ├─ data.Intraday = new Intraday
  │   ├─ data.Invalidate()  ← 清空日线缓存
  │   └─ fetchedAt = now
  │
  └─ notifySubscribers(code, data)
      └─ codeToSubs[code] → 反向索引 O(1)
          └─ for each subscriber ch:
              └─ buildEvent(data) → QuoteEvent
                  ├─ Daily() ← Lazy 聚合分时 bar
                  │   └─ computeDailyFromBars(bars, preClose)
                  │       ├─ Open = bars[0].Price
                  │       ├─ Close = bars[-1].Price
                  │       ├─ High = max(bars[*].Price)
                  │       └─ Low = min(bars[*].Price)
                  └─ QuoteData{Code, Name, Price(元), ChangePct, Minutes, Depth, PE, PB, ...}
              └─ ch ← QuoteEvent（非阻塞，channel 满则丢弃）
```

---

## 链 F：启动流程

```
main()
  │
  ├─ 1. 加载配置 config.yaml
  ├─ 2. 注册数据源适配器（tencentstock, eastmoney, ths）
  ├─ 3. 初始化数据库 + AutoMigrate
  ├─ 4. 初始化法定节假日
  ├─ 5. 初始化初始数据采集任务
  ├─ 6. Wire 构建依赖图（app）
  │
  ├─ 7. 启动运行时组件
  │   ├─ QuoteCache.Start()         ← ① 先启动（idle，无优先级）
  │   │   └─ ReloadAll()            ← ② 立即初始化优先级（夹在中间）
  │   │       └─ 从 DB 加载所有用户的持仓/Monitor/Subscription
  │   │       └─ SetPriority(highCodes, PriorityHigh)
  │   ├─ SubScheduler.Start()       ← ③ 加载活跃订阅，注册 cron
  │   ├─ DCScheduler.Start()        ← ④ 加载活跃采集任务，注册 cron
  │   └─ Monitor.Start()            ← ⑤ 加载活跃监控，subscribe 行情
  │       └─ syncWorkers(unionCodes()) → 此时 quotecache 已有 PriorityHigh
  │
  ├─ 8. 注入回调
  │   ├─ SubscriptionService.SetRunner(app.SubRunner)
  │   ├─ SubscriptionService.SetNotifyChange(→ Scheduler)
  │   ├─ MonitorConfigService.SetNotifyChange(→ Monitor)
  │   ├─ SetWatchlistManager → PortfolioService / MonitorConfigService / SubscriptionService
  │   ├─ PortfolioService.SetNotifyHoldingChanged(→ Monitor)
  │   ├─ DataCollectService.SetNotifyChange(→ DCScheduler)
  │   └─ PortfolioService.SetQuoteCache(app.QuoteCache)
  │
  └─ 9. 启动 HTTP 服务 → 监听请求 → 优雅关闭
```

---

## 链 G：数据采集任务

```
DataCollect Scheduler cron（12 个内置任务）
  │
  ├─ Task 1:  股票详情同步        cron="0 0 0 1 * *"
  ├─ Task 2:  每日增量同步K线     cron="0 0 18 ? * 1-5"
  ├─ Task 3:  东财数据全量补全    cron="0 */30 * * * *"
  ├─ Task 4:  同步全量财报        cron="0 5 1 ? * 2-6"
  ├─ Task 5:  同步股本变化        cron="0 40 1 ? * 2-6"
  ├─ Task 6:  同步股东人数变化    cron="0 5 2 ? * 2-6"
  ├─ Task 7:  同步每日快照        cron="0 0 3 ? * 2-6"
  ├─ Task 8:  每周增量同步K线     cron="0 0 1 * * 6"
  ├─ Task 9:  每月增量同步K线     cron="0 30 1 1 * ?"
  ├─ Task 10: 同步分红历史        cron="0 5 0 ? * 1-5"
  ├─ Task 11: 除权K线同步         cron="0 0 22 * * *"
  ├─ Task 12: 同步名称变更        cron="0 10 2 ? * 1-5"
  └─ Task 13: 重置高优先级        cron="0 0 8 * * *"   ← 每日 8:00
      └─ watchlistMgr.ReloadAll()
```

---

## watchlist 内部数据结构

```
Manager {
    cache quotecache.QuoteCache
    
    userPositions:     map[uid]→{code1:true, code2:true, ...}  ← 持仓股
    userMonitors:      map[uid]→{code1:true, code2:true, ...}  ← 盯盘
    userSubscriptions: map[uid]→{code1:true, code2:true, ...}  ← 订阅
    
    refCount: map[code]→int   ← 跨来源聚合：几个用户关注这只股
}

优先级规则：refCount[code] > 0  → PriorityHigh（1 分钟刷新）
          refCount[code] == 0 → PriorityLow（外部降级）
```

---

## 优先级 TTL 与刷新表

| 优先级 | TTL | 刷新频率 | 并发 | 触发条件 |
|--------|-----|---------|------|---------|
| PriorityHigh | 60s | 每 1 分钟 | 全并发 | refCount > 0 |
| PriorityNormal | 300s | 每 5 分钟 | semaphore(50) | 默认（无标记） |
| PriorityLow | 900s | 10:30/12:00/14:00 | semaphore(100) | 已清仓/删除，无人关注 |
