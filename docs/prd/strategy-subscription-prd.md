# 策略订阅模块 — 产品需求文档（PRD）

> 项目：stock-ai · 策略订阅模块
> 版本：v1.0
> 作者：产品经理
> 日期：2025-07-09

---

## 1. 产品目标

### 一句话

为用户提供策略自动化监控能力——按设定频率定期执行选股，并将结果主动推送到即时通讯机器人，实现"订阅即忘"的智能盯盘体验。

### 背景

现有策略系统已支持用户手动创建和执行选股策略，但每次都需要用户主动触发并查看结果，无法满足"长期盯盘、及时通知"的场景需求。用户希望对已保存的策略配置自动监控，在满足条件时第一时间收到通知，减少盯盘时间成本。

---

## 2. 用户故事

| # | 用户故事 |
|---|---------|
| US-1 | 作为**个人投资者**，我希望能对我关注的策略设置自动定期执行，这样我不用每天手动运行选股，节省盯盘时间。 |
| US-2 | 作为**持仓用户**，我希望订阅仅针对我持仓股票的监控，这样我能第一时间发现持仓股触发了卖出/加仓信号。 |
| US-3 | 作为**活跃交易者**，我希望将选股结果推送到钉钉/企微/飞书群，这样我和我的交易团队能同时收到信号通知。 |
| US-4 | 作为**策略研究员**，我希望能自定义通知模板，控制消息的格式和内容，让通知信息更贴合团队阅读习惯。 |
| US-5 | 作为**管理员**，我希望查看每次订阅执行的日志和统计，这样我能监控系统运行状况并排查异常。 |

---

## 3. 需求池

### P0 — Must Have（首期交付核心）

| ID | 需求 | 说明 |
|----|------|------|
| P0-01 | 订阅 CRUD | 创建/编辑/启用/禁用/删除订阅，关联策略 + 监控范围 + 调度频率 + 通知机器人 |
| P0-02 | 监控范围配置 | 支持 all（全部A股权重分批）、held（持仓股）、custom（自选股票代码列表）三种范围 |
| P0-03 | 调度引擎 | 预设频率模式（15min/30min/1h/日开盘/日收盘/每日两次/每周一次）+ 自定义 cron + trading_hours_only 开关 |
| P0-04 | 实时行情缓存模块（QuoteCache） | 进程内统一行情缓存，持仓股 3min 刷新（缓存 TTL 5min），其他股票 10min 刷新（缓存 TTL 15min），chan-based worker pool |
| P0-05 | 通知机器人关联 | M2M 关联表 `subscription_bots`，支持关联多个推送机器人（上限 5 个），列表页展示机器人名称 |
| P0-06 | 通知推送 | 订阅执行完成后，将匹配结果格式化并通过关联的推送机器人发送 |
| P0-07 | 默认通知模板 | 内置默认模板，预注入变量：strategy_name, subscription_name, run_time, total_scanned, match_count, stock_list, duration_ms, error_msg |
| P0-08 | 执行日志 | subscription_logs 表记录每次执行的时间、扫描数、匹配数、耗时、状态、错误信息 |
| P0-09 | 全量扫描分批优化 | scope=all 时 500 只/批，批内并发 50，避免瞬时资源压力 |

### P1 — Should Have（二期迭代）

| ID | 需求 | 说明 |
|----|------|------|
| P1-01 | 自定义通知模板 | 用户可编辑通知消息模板，支持预注入变量替换 |
| P1-02 | 执行历史页面 | 前端展示订阅执行日志列表，支持按时间、状态筛选，查看匹配股票明细 |
| P1-03 | 空结果不通知开关 | 用户可选择"无匹配时不发送通知"，减少噪音 |
| P1-04 | 订阅执行统计 | 展示订阅运行次数、平均耗时、最近匹配时间等统计指标 |
| P1-05 | 监控范围扩展 | 支持 watchlist（自选股）、portfolio（组合）、industry（行业）范围类型 |

### P2 — Nice to Have（远期规划）

| ID | 需求 | 说明 |
|----|------|------|
| P2-01 | 条件触发推送 | 除定时执行外，支持"信号出现时才推送"的事件驱动模式 |
| P2-02 | 推送去重/合并 | 短时间内同一股票被多个订阅命中时，合并为一条消息 |
| P2-03 | 订阅模板市场 | 预置行业/场景化订阅模板（如"放量突破""均线多头"），一键创建 |
| P2-04 | 推送限流 | 单个机器人每小时最大推送数限制，防止消息轰炸 |
| P2-05 | Webhook 回调 | 除即时通讯外，支持通用 Webhook 推送 |

---

## 4. 功能规格

### 4.1 订阅管理（Subscription CRUD）

**创建订阅流程：**

1. 用户选择关联策略（从 strategies 表已有策略中选取）
2. 配置监控范围（scope + custom_stocks）
3. 选择调度频率（preset_type 或自定义 cron）+ trading_hours_only
4. 关联通知机器人（从 push_bots 表中选取，最多 5 个）
5. 可选：自定义通知模板
6. 保存并默认启用

**订阅状态机：**

```
创建 → 启用(active) ⇄ 暂停(paused) → 删除(deleted)
```

**编辑规则：**
- 运行中的订阅修改后，调度器需热更新（下次调度时使用新配置）
- 删除为软删除，保留执行日志

### 4.2 监控范围（Monitor Scope）

| scope 值 | 说明 | 股票来源 |
|----------|------|---------|
| `all` | 全部 A 股 | 数据源全量股票池（约 5000 只），分批扫描 |
| `held` | 用户持仓 | positions 表关联的股票代码 |
| `custom` | 自选股票 | custom_stocks 字段存储（6 位纯数字代码 JSON 数组，如 `["000001","600036"]`） |

**数据模型概要：**

```
Subscription
├── id                  PK
├── name                订阅名称（用户自定义）
├── strategy_id         FK → strategies.id
├── scope               ENUM('all','held','custom')
├── custom_stocks       JSON（仅 scope=custom 时有值，存 6 位代码数组）
├── preset_type         ENUM('every_15min','every_30min','every_hour','daily_open','daily_close','daily_twice','weekly')
├── cron_expr           STRING（自定义 cron 表达式，preset_type=custom 时使用）
├── trading_hours_only  BOOLEAN（默认 true）
├── is_active           BOOLEAN（启用/暂停）
├── template            TEXT（用户自定义模板，NULL 时使用默认模板）
├── silent_on_empty     BOOLEAN（无匹配时不通知，默认 false）【P1】
├── created_at / updated_at / deleted_at
```

### 4.3 通知机器人关联（M2M）

**数据模型概要：**

```
subscription_bots
├── id               PK
├── subscription_id  FK → subscriptions.id
├── bot_id           FK → push_bots.id
├── created_at
```

**约束：**
- 同一订阅下同一 bot_id 不可重复
- 每个订阅最多关联 5 个机器人（应用层校验）
- 级联删除：订阅删除时清理关联记录

### 4.4 实时行情缓存模块（QuoteCache）

**设计要点：**
- 独立进程内模块，为所有订阅/调度器提供统一行情缓存服务
- 优先级分层机制：

| 优先级 | 刷新频率 | 缓存 TTL | 适用场景 |
|--------|---------|---------|---------|
| 高（highChan） | 3 分钟 | 5 分钟 | 持仓股（scope=held 涉及的股票自动提升优先级） |
| 普通（normalChan） | 10 分钟 | 15 分钟 | 全量扫描中的其他股票 |

**架构：**
- 两个 channel（highChan + normalChan），worker 通过 `select` 优先消费 highChan
- 两个 Ticker 分别驱动高/低优先级刷新周期
- **不内置限速器**：底层 adapter.DataSource 已有令牌桶限速，QuoteCache 不重复限速
- 数据组装：执行时构建 `CachedStock`（嵌入 DBStock + QuoteCache），历史数据从 DB 取，今日行情从缓存取

### 4.5 调度引擎

**预设频率模式：**

| preset_type | cron 表达式（参考） | 说明 |
|-------------|---------------------|------|
| every_15min | `*/15 9-14 * * 1-5` | 每 15 分钟 |
| every_30min | `*/30 9-14 * * 1-5` | 每 30 分钟 |
| every_hour | `0 9-14 * * 1-5` | 每小时 |
| daily_open | `30 9 * * 1-5` | 日开盘（9:30） |
| daily_close | `0 15 * * 1-5` | 日收盘（15:00） |
| daily_twice | `30 9,13 * * 1-5` | 每日两次（9:30 + 13:00） |
| weekly | `30 9 * * 1` | 每周一开盘 |
| custom | 用户自定义 | 自定义 cron 表达式 |

**trading_hours_only 开关：**
- 开启时（默认）：仅在交易日 9:15-15:00 期间执行，非交易时段跳过
- A 股交易日判断：排除周末 + 法定节假日（需维护交易日历）
- 关闭时：按 cron 表达式严格执行，不限交易日

**技术选型：** robfig/cron/v3

**热更新：** 订阅配置变更后，调度器在下次调度时自动加载最新配置，无需重启服务。

### 4.6 全量扫描分批优化

当 scope=all 时：
- 全量股票池按 500 只分批
- 批内并发 50（复用 indicator.Engine 的并发能力）
- 批次间串行执行，避免资源过载
- 单次扫描总体超时保护（建议 10 分钟）

### 4.7 通知推送

**执行流程：**

```
订阅触发 → 获取监控范围股票 → 执行选股引擎 → 格式化结果 → 遍历关联机器人 → 发送通知 → 记录日志
```

**预注入模板变量：**

| 变量 | 类型 | 说明 |
|------|------|------|
| `strategy_name` | string | 关联策略名称 |
| `subscription_name` | string | 订阅名称 |
| `run_time` | string | 执行时间（格式：YYYY-MM-DD HH:mm:ss） |
| `total_scanned` | int | 扫描股票总数 |
| `match_count` | int | 匹配股票数 |
| `stock_list` | string | 服务端预渲染的匹配股票列表文本 |
| `duration_ms` | int | 执行耗时（毫秒） |
| `error_msg` | string | 错误信息（无错误时为空） |

**默认通知模板和消息格式详见第 6 节。**

### 4.8 执行日志

**数据模型概要：**

```
SubscriptionLog
├── id               PK
├── subscription_id  FK → subscriptions.id
├── run_time         DATETIME（执行触发时间）
├── finished_at      DATETIME（执行完成时间）
├── scope            ENUM（执行时的监控范围）
├── total_scanned    INT（扫描股票数）
├── match_count      INT（匹配股票数）
├── match_stocks     JSON（匹配的股票代码+名称列表）
├── duration_ms      INT（耗时毫秒）
├── status           ENUM('success','partial','failed')
├── error_msg        TEXT（错误信息）
├── push_status      JSON（各机器人推送结果：bot_id → success/failed/reason）
├── created_at
```

**status 说明：**
- `success`：选股执行成功，通知发送完成
- `partial`：选股执行成功，但部分机器人推送失败
- `failed`：选股执行失败或全部推送失败

---

## 5. API 设计概要

### 5.1 订阅管理

#### 创建订阅

```
POST /api/v1/subscriptions

Request:
{
  "name": "均线多头突破",
  "strategy_id": 1,
  "scope": "custom",
  "custom_stocks": ["000001", "600036", "300750"],
  "preset_type": "every_30min",
  "trading_hours_only": true,
  "bot_ids": [1, 3]
}

Response (201):
{
  "code": 0,
  "data": {
    "id": 1,
    "name": "均线多头突破",
    "strategy_id": 1,
    "scope": "custom",
    "preset_type": "every_30min",
    "trading_hours_only": true,
    "is_active": true,
    "bots": [
      {"id": 1, "name": "交易群-钉钉", "type": "dingtalk"},
      {"id": 3, "name": "个人-企微", "type": "wecom"}
    ],
    "created_at": "2025-07-09T10:00:00Z"
  }
}
```

#### 更新订阅

```
PUT /api/v1/subscriptions/:id

Request: 同创建（所有字段可选）
Response (200): 同创建响应
```

#### 获取订阅列表

```
GET /api/v1/subscriptions?page=1&page_size=20&is_active=true

Response (200):
{
  "code": 0,
  "data": {
    "total": 15,
    "items": [
      {
        "id": 1,
        "name": "均线多头突破",
        "strategy_name": "MA金叉策略",
        "scope": "custom",
        "preset_type": "every_30min",
        "is_active": true,
        "bots": ["交易群-钉钉"],
        "last_run_time": "2025-07-09T10:30:00Z",
        "last_match_count": 3,
        "created_at": "2025-07-09T10:00:00Z"
      }
    ]
  }
}
```

**说明：** 列表页 JOIN strategies 和 push_bots 表，展示策略名称和机器人名称，避免前端二次查询。

#### 获取订阅详情

```
GET /api/v1/subscriptions/:id

Response (200):
{
  "code": 0,
  "data": {
    "id": 1,
    "name": "均线多头突破",
    "strategy_id": 1,
    "strategy_name": "MA金叉策略",
    "scope": "custom",
    "custom_stocks": ["000001", "600036", "300750"],
    "preset_type": "every_30min",
    "trading_hours_only": true,
    "is_active": true,
    "template": null,
    "bots": [
      {"id": 1, "name": "交易群-钉钉", "type": "dingtalk"}
    ],
    "created_at": "2025-07-09T10:00:00Z",
    "updated_at": "2025-07-09T10:00:00Z"
  }
}
```

#### 启用/暂停订阅

```
PATCH /api/v1/subscriptions/:id/active

Request:
{
  "is_active": false
}

Response (200):
{
  "code": 0,
  "message": "ok"
}
```

#### 删除订阅（软删除）

```
DELETE /api/v1/subscriptions/:id

Response (200):
{
  "code": 0,
  "message": "ok"
}
```

#### 手动触发执行

```
POST /api/v1/subscriptions/:id/run

Response (200):
{
  "code": 0,
  "data": {
    "log_id": 123,
    "status": "success",
    "total_scanned": 3,
    "match_count": 1,
    "duration_ms": 1250
  }
}
```

### 5.2 执行日志

#### 查询执行日志

```
GET /api/v1/subscriptions/:id/logs?page=1&page_size=20&status=success

Response (200):
{
  "code": 0,
  "data": {
    "total": 58,
    "items": [
      {
        "id": 123,
        "run_time": "2025-07-09T10:30:00Z",
        "finished_at": "2025-07-09T10:30:01Z",
        "scope": "custom",
        "total_scanned": 3,
        "match_count": 1,
        "match_stocks": [
          {"code": "000001", "name": "平安银行"}
        ],
        "duration_ms": 1250,
        "status": "success",
        "error_msg": ""
      }
    ]
  }
}
```

### 5.3 通知机器人关联

> 机器人的 CRUD 已在现有 push_bots 模块中实现，此处仅需关联操作。

#### 更新关联机器人

```
PUT /api/v1/subscriptions/:id/bots

Request:
{
  "bot_ids": [1, 2, 5]
}

Response (200):
{
  "code": 0,
  "message": "ok"
}
```

**行为：** 全量替换当前关联关系（先清除旧关联，再批量插入新关联）。应用层校验 bot_ids 上限 5 个且全部合法。

---

## 6. 通知消息格式

### 6.1 默认模板

**有匹配结果时：**

```
📊 策略订阅通知

📌 订阅：{subscription_name}
📋 策略：{strategy_name}
🕐 时间：{run_time}

✅ 命中 {match_count} 只股票（共扫描 {total_scanned} 只）

{stock_list}

⏱ 耗时：{duration_ms}ms
```

**stock_list 预渲染格式（服务端生成）：**

```
1. 平安银行(000001) — 现价 12.35 涨幅 +2.15%
2. 贵州茅台(600519) — 现价 1688.00 涨幅 +0.82%
3. 宁德时代(300750) — 现价 198.50 涨幅 +3.41%
```

### 6.2 空结果模板

**无匹配结果时：**

```
📊 策略订阅通知

📌 订阅：{subscription_name}
📋 策略：{strategy_name}
🕐 时间：{run_time}

📭 本次未命中任何股票（共扫描 {total_scanned} 只）

⏱ 耗时：{duration_ms}ms
```

### 6.3 执行失败模板

```
📊 策略订阅通知

📌 订阅：{subscription_name}
📋 策略：{strategy_name}
🕐 时间：{run_time}

❌ 执行失败：{error_msg}

⏱ 耗时：{duration_ms}ms
```

### 6.4 自定义模板规则（P1）

- 用户可编辑纯文本模板
- 使用 `{{variable_name}}` 双花括号语法引用变量
- `stock_list` 变量由服务端预渲染为格式化文本，不支持用户自定义 stock_list 内部格式
- 变量未定义时替换为空字符串
- 模板预览：前端提供变量值模拟 + 渲染预览功能

---

## 7. 非功能性需求

### 7.1 性能

| 指标 | 目标 |
|------|------|
| 订阅调度精度 | 误差 < 30 秒（取决于 cron 精度） |
| 单次选股执行（scope=held/custom ≤ 50 只） | < 5 秒 |
| 单次全量扫描（scope=all，~5000 只） | < 10 分钟 |
| 行情缓存查询延迟 | < 1ms（内存读取） |
| 通知推送延迟 | < 3 秒（选股完成到消息发送） |
| API 响应时间（列表/详情） | < 200ms |

### 7.2 可靠性

| 指标 | 目标 |
|------|------|
| 调度引擎可用性 | 99.9%（月度） |
| 通知送达率 | > 95%（排除第三方服务故障） |
| 失败重试 | 单次推送失败自动重试 1 次 |
| 数据一致性 | 执行日志必须写入成功后才返回成功状态 |
| 并发安全 | QuoteCache 读写需 thread-safe（sync.Map 或 RWMutex） |

### 7.3 可观测性

- 执行日志结构化存储，支持按订阅、时间、状态检索
- 调度引擎关键事件输出日志（订阅启动/停止、执行超时、推送失败）
- 行情缓存命中率统计（可选，用于调优刷新频率）

### 7.4 安全性

- 订阅操作需鉴权（复用现有用户认证体系）
- cron 表达式需服务端校验合法性，防止注入
- custom_stocks 输入需校验为合法 6 位数字代码

---

## 8. 待确认问题

| # | 问题 | 影响 | 建议 |
|---|------|------|------|
| Q1 | 交易日历数据来源？是内置静态表还是从数据源动态获取？ | trading_hours_only 的准确性 | 建议内置维护，每年更新一次，避免运行时依赖外部服务 |
| Q2 | 多用户场景下，QuoteCache 是全局共享还是按用户隔离？ | 缓存设计复杂度和内存占用 | 建议全局共享（股票行情与用户无关），简化实现 |
| Q3 | 推送机器人的消息格式适配（Markdown/Text/Card）是否需要按机器人类型区分？ | 通知展示效果 | 建议首期统一用纯文本，后续按需适配各平台富文本格式 |
| Q4 | 订阅数量上限？单用户/全局最大订阅数？ | 资源规划 | 建议单用户上限 20 个，全局无上限（受调度引擎性能制约） |
| Q5 | 是否需要订阅暂停原因说明字段？ | 用户排查问题 | 建议首期不加，P1 通过日志排查 |
| Q6 | scope=all 时全量股票池的数据来源？是否复用 adapter.DataSource 的接口？ | 全量扫描的股票列表获取方式 | 建议复用 adapter 层已有接口，如有全量列表接口直接调用 |
