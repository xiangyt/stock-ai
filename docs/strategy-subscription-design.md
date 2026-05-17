# 策略订阅模块设计文档

> 版本: 2.0 | 日期: 2026-05-17 | Subscription Engine 重构

---

## 一、模块概述

策略订阅模块是 ai-stock-picker 的核心自动化组件，允许用户将自定义选股策略绑定到定时调度任务上，系统按配置的频率自动扫描股票池、执行策略筛选，并将匹配结果通过钉钉/飞书/企微等渠道推送给用户。

**核心价值链**: `用户定义策略 → 配置订阅调度 → 引擎定时扫描 → 匹配结果推送`

---

## 二、系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        Frontend (Vue 3)                         │
│  ┌──────────────┐  ┌──────────────────┐  ┌───────────────────┐  │
│  │StrategyList  │  │StrategyBuilder   │  │SubscriptionPage   │  │
│  │  策略列表     │  │  策略编辑器       │  │  订阅管理页        │  │
│  └──────┬───────┘  └───────┬──────────┘  └───────┬───────────┘  │
│         │                  │                      │              │
│         └──────────────────┼──────────────────────┘              │
│                            │ HTTP REST (JWT)                     │
└────────────────────────────┼────────────────────────────────────┘
                             │
┌────────────────────────────┼────────────────────────────────────┐
│                     Backend (Go + Gin)                          │
│                            ▼                                     │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Router (Gin)                          │    │
│  │  /api/v1/strategies/*        /api/v1/subscriptions/*     │    │
│  └─────────┬───────────────────────────────┬───────────────┘    │
│            │                               │                    │
│  ┌─────────▼──────────┐     ┌─────────────▼──────────────┐     │
│  │  StrategyHandler    │     │  SubscriptionHandler       │     │
│  └─────────┬──────────┘     └─────────────┬──────────────┘     │
│            │                               │                    │
│  ┌─────────▼──────────┐     ┌─────────────▼──────────────┐     │
│  │  StrategyService    │     │  SubscriptionService       │     │
│  └─────────┬──────────┘     └─────────────┬──────────────┘     │
│            │                               │                    │
│  ┌─────────▼──────────┐     ┌─────────────▼──────────────┐     │
│  │  StrategyDAO        │     │  SubscriptionDAO           │     │
│  └─────────┬──────────┘     └─────────────┬──────────────┘     │
│            │                               │                    │
│  ┌─────────▼───────────────────────────────▼──────────────┐     │
│  │                    MySQL (GORM)                        │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              Subscription Engine (后台引擎)                │    │
│  │  ┌──────────┐  ┌────────┐  ┌──────────┐  ┌──────────┐  │    │
│  │  │Scheduler │→ │Runner  │→ │Notifier  │  │QuoteCache│  │    │
│  │  │ 调度器   │  │执行器  │  │推送器    │  │行情缓存  │  │    │
│  │  └──────────┘  └────────┘  └──────────┘  └──────────┘  │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 分层职责

| 层级 | 目录 | 职责 |
|------|------|------|
| Model | `internal/model/` | 数据结构定义、枚举、序列化 |
| DAO | `internal/db/` | 数据库 CRUD，GORM 操作 |
| Service | `internal/service/` | 业务逻辑、校验、数据组装、CRUD 后通知 Scheduler |
| Handler | `internal/api/handler/` | HTTP 入参/出参处理、调用 Service |
| Router | `internal/api/router/` | 路由注册、中间件 |
| Engine | `internal/subscription/` | 后台引擎（子包结构，见 2.2） |

### 2.2 Engine 子包结构

```
internal/subscription/
├── runner/              # 订阅执行器（单例）
│   └── runner.go        # StockSourceProvider 接口 + Runner 实现
├── scheduler/           # 调度引擎（独立子包）
│   ├── scheduler.go     # Scheduler + SubscriptionChange 事件 + changeLoop
│   └── trading.go       # 交易日/交易时段判断
├── quotecache/          # 行情缓存（独立子包）
│   ├── cache.go         # QuoteCache 接口 + 双优先级实现（日/周/月/年四周期缓存）
│   └── stock.go         # CachedStock（缓存增强 StockSource）+ CachedQuoteProvider
└── notifier/            # 通知推送（独立子包）
    ├── notifier.go      # Notifier 接口 + 钉钉/飞书/企微实现
    └── template.go      # 通知模板 + 渲染
```

### 2.3 依赖关系

```
service
  └── → subscription/runner       (Runner 单例 + TriggerRun 直调)
  注入: NotifyChange 回调函数（由 main.go 闭包包裹 Scheduler.NotifyChange）

subscription/scheduler
  ├── → subscription/runner       (调用 Runner.Run)
  └── → db                        (内置 DB 加载，直接调用 GetActiveSubscriptions/GetSubscriptionByIDForScheduler)

subscription/runner
  ├── → subscription/notifier     (渲染 + 推送)
  └── → StockSourceProvider       (接口，由 quotecache 实现)

subscription/quotecache
  ├── → subscription/runner       (CachedQuoteProvider 实现 StockSourceProvider 接口)
  ├── → indicator/stocksource     (CachedStock 嵌入 DBStock)
  └── → adapter                   (行情数据源：日/周/月/年四周期)

subscription/notifier
  └── → model                     (PushBot)
```

---

## 三、数据模型

### 3.1 ER 关系图

```
User (users)
  ├── 1:N ──→ Strategy (strategies)
  │              │
  │              └── 1:N ──→ Subscription (strategy_subscriptions)
  │                              │
  │                              ├── N:1 ──→ Strategy (策略引用)
  │                              ├── M:N ──→ PushBot (via subscription_bots)
  │                              └── 1:N ──→ SubscriptionLog (subscription_logs)
  │
  └── 1:N ──→ PushBot (push_bots)
```

### 3.2 表结构详细定义

#### subscriptions (strategy_subscriptions)

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | uint | PK, auto_inc | 主键 |
| uid | uint | INDEX, NOT NULL | 所属用户 ID |
| name | varchar(100) | NOT NULL | 订阅名称 |
| strategy_id | uint | INDEX, NOT NULL | 关联策略 ID |
| scope | varchar(20) | NOT NULL, default "all" | 监控范围: all/held/custom |
| custom_stocks | text | | 自选股票 JSON: ["000001","600036"] |
| preset_type | varchar(20) | NOT NULL, default "every_30min" | 频率预设 |
| cron_expr | varchar(100) | | 自定义 cron 表达式 |
| trading_hours_only | bool | default true | 仅交易时段执行 |
| is_active | bool | default true, INDEX | 启停状态 |
| template | text | | 用户自定义通知模板 |
| last_run_at | timestamp | | 最后执行时间 |
| created_at | timestamp | | 创建时间 |
| updated_at | timestamp | | 更新时间 |
| deleted_at | timestamp | INDEX | 软删除 |

#### subscription_bots

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | uint | PK | 主键 |
| subscription_id | uint | UNIQUE_IDX(sub_bot), NOT NULL | 订阅 ID |
| bot_id | uint | UNIQUE_IDX(sub_bot), NOT NULL | 机器人 ID |
| created_at | timestamp | | 创建时间 |

#### subscription_logs

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | uint | PK | 主键 |
| subscription_id | uint | INDEX, NOT NULL | 订阅 ID |
| run_time | timestamp | NOT NULL | 执行开始时间 |
| finished_at | timestamp | | 执行结束时间 |
| scope | varchar(20) | | 本次执行范围 |
| total_scanned | int | default 0 | 扫描股票总数 |
| match_count | int | default 0 | 匹配股票数 |
| match_stocks | text | | 匹配股票 JSON: [{code,name,price}] |
| duration_ms | int | default 0 | 执行耗时(毫秒) |
| status | varchar(20) | NOT NULL | success/partial/failed |
| error_msg | text | | 错误信息 |
| push_status | text | | 各机器人推送状态 JSON |
| created_at | timestamp | | 创建时间 |

#### strategies

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | uint | PK | 主键 |
| uid | uint | INDEX | 所属用户 |
| name | varchar(100) | NOT NULL | 策略名称 |
| description | varchar(500) | | 策略描述 |
| logical_op | varchar(10) | default "and" | 条件逻辑: and/or |
| conditions_raw | text | NOT NULL | 条件 JSON: [{signal,params,op,value}] |
| backtest_count | int | default 0 | 回测次数 |
| last_run_at | timestamp | | 最后回测时间 |
| is_public | bool | default false | 是否公开 |
| star_count | int | default 0 | 收藏数 |
| created_at / updated_at / deleted_at | timestamp | | 时间戳 |

#### push_bots

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | uint | PK | 主键 |
| user_id | uint | INDEX, NOT NULL | 所属用户 |
| name | varchar(100) | NOT NULL | 机器人名称 |
| channel | varchar(20) | NOT NULL | 渠道: qq/wecom/dingtalk/feishu |
| webhook_url | varchar(500) | | Webhook 地址 |
| token | varchar(255) | | 鉴权 Token |
| secret | varchar(255) | | 签名密钥 |
| status | int | default 1 | 0=禁用 1=启用 |
| created_at / updated_at | timestamp | | 时间戳 |

### 3.3 核心枚举定义

**MonitorScope (监控范围)**:
| 值 | 标签 | 说明 |
|---|---|---|
| all | 全部A股 | 扫描全市场 |
| held | 我的持仓 | 仅扫描用户持仓股 |
| custom | 自选股票 | 扫描用户指定的股票列表 |

**PresetType (频率预设)**:
| 值 | 标签 | Cron 表达式 |
|---|---|---|
| every_15min | 每15分钟 | `*/15 * * * *` |
| every_30min | 每30分钟 | `*/30 * * * *` |
| every_hour | 每小时 | `0 * * * *` |
| daily_open | 每日开盘 | `15 9 * * 1-5` |
| daily_close | 每日收盘 | `0 15 * * 1-5` |
| daily_twice | 每日两次 | `15 9,0 15 * * 1-5` |
| weekly | 每周 | `0 9 * * 1` |
| custom | 自定义 | 用户填写 |

**LogStatus (执行状态)**:
| 值 | 说明 |
|---|---|
| success | 执行成功 |
| partial | 部分成功（有匹配但推送部分失败） |
| failed | 执行失败 |

---

## 四、API 接口设计

### 4.1 策略管理 `/api/v1/strategies`

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/` | 策略列表（搜索+分页） | JWT |
| POST | `/` | 创建策略 | JWT |
| GET | `/:id` | 策略详情 | JWT |
| PUT | `/:id` | 全量更新策略 | JWT |
| DELETE | `/:id` | 删除策略 | JWT |
| PUT | `/:id/rename` | 重命名 | JWT |
| DELETE | `/batch` | 批量删除 | JWT |

### 4.2 订阅管理 `/api/v1/subscriptions`

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/` | 订阅列表（分页+状态过滤） | JWT |
| POST | `/` | 创建订阅 | JWT |
| GET | `/:id` | 订阅详情 | JWT |
| PUT | `/:id` | 更新订阅 | JWT |
| DELETE | `/:id` | 删除订阅 | JWT |
| PATCH | `/:id/active` | 切换启停状态 | JWT |
| POST | `/:id/run` | 手动触发执行 | JWT |
| PUT | `/:id/bots` | 更新关联机器人 | JWT |

### 4.3 关键请求/响应结构

**创建订阅请求**:
```json
{
  "name": "涨停板策略-每15分钟",
  "strategy_id": 1,
  "scope": "all",
  "custom_stocks": ["000001", "600036"],
  "preset_type": "every_15min",
  "cron_expr": "",
  "trading_hours_only": true,
  "template": "",
  "bot_ids": [1, 2]
}
```

**订阅详情响应**:
```json
{
  "id": 1,
  "name": "涨停板策略-每15分钟",
  "strategy_id": 1,
  "strategy_name": "涨停板筛选",
  "scope": "all",
  "preset_type": "every_15min",
  "is_active": true,
  "last_run_at": "2026-05-17T14:30:00+08:00",
  "bots": [
    {"id": 1, "name": "工作群机器人", "channel": "dingtalk"}
  ],
  "created_at": "2026-05-01T10:00:00+08:00"
}
```

---

## 五、核心业务流程

### 5.1 订阅创建流程

```
用户提交创建请求
       │
       ▼
┌──────────────────┐    失败    ┌──────────┐
│ 校验策略归属      │──────────→│ 返回错误  │
│ (必须为当前用户)  │            └──────────┘
└───────┬──────────┘
        │ 通过
        ▼
┌──────────────────┐    失败    ┌──────────┐
│ 校验订阅数量上限   │──────────→│ 返回错误  │
│ (每用户 ≤ 20)    │            │ "已达上限"│
└───────┬──────────┘            └──────────┘
        │ 通过
        ▼
┌──────────────────┐    失败    ┌──────────┐
│ 校验参数          │──────────→│ 返回错误  │
│ scope/cron/stocks │            └──────────┘
└───────┬──────────┘
        │ 通过
        ▼
┌──────────────────┐
│ 创建订阅记录      │
│ 写入 DB          │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 设置关联机器人    │
│ (事务: 先删后插)  │
│ 每订阅 ≤ 5 个     │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 返回订阅详情      │
└──────────────────┘
```

### 5.2 定时执行流程 (Scheduler → Runner)

```
Cron 触发 (按预设/cron 表达式)
       │
       ▼
┌──────────────────┐    否     ┌────────────┐
│ is_active?       │──────────→│ 跳过执行   │
└───────┬──────────┘           └────────────┘
        │ 是
        ▼
┌──────────────────┐    否     ┌────────────┐
│ trading_hours    │──────────→│ 跳过执行   │
│ _only 检查       │           └────────────┘
│ (交易日+交易时段) │
└───────┬──────────┘
        │ 是
        ▼
┌──────────────────┐
│ 加载策略条件      │
│ 解析 Signal JSON  │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 获取股票代码池     │
│ all → 全部A股     │
│ held → 用户持仓   │
│ custom → 自选列表 │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 构建数据源        │
│ 并发构建(20路)    │
│ + 行情缓存增强    │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 执行选股引擎      │
│ 全量: 500/批     │
│ 并发: 50 路      │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 筛选匹配股票      │
│ ResultPassed     │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 加载关联机器人    │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 渲染通知消息      │
│ (模板 + 变量替换) │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 推送通知          │
│ (失败自动重试1次)  │
└───────┬──────────┘
        │
        ▼
┌──────────────────┐
│ 写入执行日志      │
│ 更新 last_run_at  │
└──────────────────┘
```

---

## 六、后台引擎设计

### 6.1 Runner 执行器 (`subscription/runner/`)

- **单例模式**: 全局唯一 Runner 实例，由 main.go 创建
- **两个触发入口**:
  - `Scheduler` 定时触发 → `Runner.Run(ctx, sub)`
  - `HTTP TriggerRun` 直调 → `goroutine { Runner.Run(ctx, sub) }`（不经过 Scheduler）
- **StockSourceProvider 接口**: Runner 不直接依赖 QuoteCache，而是通过注入的 `StockSourceProvider` 接口获取 `[]indicator.StockSource`，由 `quotecache.CachedQuoteProvider` 实现（组合 `QuoteCache` + `DBStock`，K线拼接缓存当期数据）
- **超时控制**: 单次执行 10 分钟超时 (`context.WithTimeout`)，由调用方设置
- **并发策略**:
  - 全量扫描: 每 500 只为一批，最大 50 路并发
  - 持仓/自选: 直接执行，最大 10 路并发
- **状态判定**:
  - 有匹配 + 全部推送成功 → `success`
  - 有匹配 + 部分推送失败 → `partial`
  - 无匹配 → `success`
  - 有匹配但无机器人 → `failed`

### 6.2 Scheduler 调度器 (`subscription/scheduler/`)

- **精度**: 秒级 cron (`cron.WithSeconds()`)
- **热更新**: 每次执行前从 DB 重新加载订阅配置
- **变更通知机制**:
  - 内置 `chan SubscriptionChange`（缓冲 64），Service 层 CRUD 后非阻塞通知
  - `changeLoop` goroutine 消费事件，动态增删 cron job
  - 支持 5 种变更类型: `created`/`updated`/`deleted`/`enabled`/`disabled`
- **依赖注入**: `Runner` 通过构造函数注入；DB 加载内置在 scheduler 包中（直接依赖 `db` 包）
- **启动流程**: `Start()` → 加载活跃订阅 → 注册 cron job → 启动 cron → 启动 changeLoop

### 6.3 QuoteCache 行情缓存 (`subscription/quotecache/`)

| 优先级 | TTL | 刷新频率 | 用途 |
|--------|-----|---------|------|
| high | 5 分钟 | 每 3 分钟 | 持仓股/高频访问 |
| normal | 15 分钟 | 每 10 分钟 | 普通扫描 |

- 3 个 worker goroutine 消费刷新任务
- 支持 `Promote()` / `Demote()` 动态调整优先级
- 缓存未命中时实时调用 API 获取

### 6.4 Notifier 通知推送

| 渠道 | 协议 | 特殊处理 |
|------|------|---------|
| 钉钉 | Webhook | HMAC-SHA256 加签 |
| 飞书 | Webhook | 标准 JSON |
| 企微 | Webhook | 标准 JSON |

- 失败自动重试 1 次 (间隔 500ms)
- 统一 text 消息格式

### 6.5 Template 通知模板

- 3 种默认模板: 有匹配 / 无匹配 / 执行失败
- 用户可自定义模板 (存储于 `Subscription.Template`)
- 变量: `{strategy_name}`, `{scope_label}`, `{total_scanned}`, `{match_count}`, `{duration_ms}`, `{run_time}`, `{match_list}`, `{error_msg}`

---

## 七、前端设计

### 7.1 页面结构

| 组件 | 文件 | 职责 |
|------|------|------|
| SubscriptionPage | `web/src/components/SubscriptionPage.vue` | 订阅管理主页 |
| StrategyList | `web/src/components/StrategyList.vue` | 策略列表(表格/卡片) |
| StrategyBuilder | `web/src/components/StrategyBuilder.vue` | 策略编辑器 |

### 7.2 订阅管理页功能

- **列表展示**: 表格形式展示所有订阅（名称、策略、范围、频率、机器人、状态、最后执行时间）
- **创建/编辑**: 弹窗表单，含策略选择、范围配置、频率设置、机器人多选
- **操作**: 启停切换、手动触发执行、删除（带确认）
- **表单校验**: 必填项、scope=custom 时股票代码必填、preset=custom 时 cron 必填

---

## 八、业务约束与限制

| 约束项 | 限制值 | 说明 |
|--------|--------|------|
| 每用户最大订阅数 | 20 | `maxUserSubscriptions` |
| 每订阅最大机器人数 | 5 | `maxBotsPerSubscription` |
| 单次执行超时 | 10 分钟 | `context.WithTimeout` |
| 全量扫描批次大小 | 500 只/批 | 内存控制 |
| 全量扫描并发度 | 50 路 | | 
| 持仓/自选扫描并发度 | 10 路 | |
| 通知推送重试次数 | 1 次 | 间隔 500ms |
| 交易日判定 | 周末 + 法定节假日 | 硬编码 2025 年节假日 |
| 交易时段 | 9:15 ~ 15:00 | |

---

## 九、潜在优化方向

> 以下为基于代码分析的改进建议，供设计评审参考

### 9.1 性能优化

| 问题 | 现状 | 建议 |
|------|------|------|
| 全量A股扫描耗时 | 每次执行扫描全市场 | 引入增量扫描或分层抽样机制 |
| 行情缓存粒度 | 双优先级 (high/normal) | 可增加 per-subscription 级别缓存 |
| 日志无归档 | subscription_logs 无清理机制 | 增加日志归档/清理策略（保留N天） |
| 启动时全量加载 | Scheduler.Start() 加载所有活跃订阅 | 可优化为延迟加载 |

### 9.2 功能增强

| 功能 | 现状 | 建议 |
|------|------|------|
| 通知渠道 | 仅钉钉/飞书/企微 | 增加 Telegram / 邮件 / Server酱 |
| 消息格式 | 仅 text 类型 | 支持 Markdown / 卡片消息 |
| 策略共享 | is_public 字段已存在但未使用 | 实现策略市场/推荐机制 |
| 订阅分组 | 无分组概念 | 增加订阅分组/标签管理 |
| 执行失败告警 | 仅记录日志 | 连续失败时告警通知用户 |

### 9.3 可靠性提升

| 问题 | 现状 | 建议 |
|------|------|------|
| 节假日硬编码 | 2025 年节假日写死在代码中 | 从配置文件或外部 API 获取 |
| 推送可靠性 | 重试 1 次 | 增加消息队列保证至少一次投递 |
| 并发控制 | 固定并发数 | 基于系统负载动态调整 |
| 分布式 | 单进程 cron | 未来多实例部署需分布式调度 |

### 9.4 代码架构优化

| 问题 | 现状 | 建议 |
|------|------|------|
| Service 全局变量 | `SubscriptionServiceRef` 全局引用 | 已通过 `SetNotifyChange`/`SetRunner` 闭包注入优化 |
| 订阅 SQL 缺 DDL | 依赖 GORM AutoMigrate | 补充手动 DDL 脚本至 sql/ 目录 |
| 前端 API 类型重复 | API 文件和组件中重复定义 | 统一类型导出 |
| List 方法过滤 | 内存中按 isActive 过滤 | 下推到 SQL WHERE 条件 |

---

## 十、文件索引

| 文件路径 | 说明 |
|---------|------|
| `internal/model/subscription.go` | Subscription/SubscriptionBot/SubscriptionLog 模型 |
| `internal/model/indicator.go` | Strategy 模型 (含 Signal/Conditions) |
| `internal/model/bot.go` | PushBot 模型 |
| `internal/db/dao_subscription.go` | 订阅 DAO 层 |
| `internal/db/dao_strategy.go` | 策略 DAO 层 |
| `internal/db/dao_bot.go` | 机器人 DAO 层 |
| `internal/service/subscription_service.go` | 订阅业务逻辑（CRUD + NotifyChange） |
| `internal/service/strategy_service.go` | 策略业务逻辑 |
| `internal/api/handler/subscription_handler.go` | 订阅 HTTP Handler |
| `internal/api/handler/strategy_handler.go` | 策略 HTTP Handler |
| `internal/api/router/subscription_router.go` | 订阅路由定义 |
| `internal/api/router/strategy_router.go` | 策略路由定义 |
| `internal/subscription/runner/runner.go` | 执行器（单例 + StockSourceProvider 接口） |
| `internal/subscription/scheduler/scheduler.go` | 调度引擎（cron + changeLoop） |
| `internal/subscription/scheduler/trading.go` | 交易日/时段判断 |
| `internal/subscription/quotecache/cache.go` | 行情缓存（双优先级，日/周/月/年四周期） |
| `internal/subscription/quotecache/stock.go` | CachedStock（缓存增强 StockSource）+ CachedQuoteProvider |
| `internal/subscription/notifier/notifier.go` | 通知推送（钉钉/飞书/企微） |
| `internal/subscription/notifier/template.go` | 通知模板 + 渲染 |
| `sql/strategy_tables.sql` | 策略表 DDL |
| `sql/bot_tables.sql` | 机器人表 DDL |
| `web/src/api/subscriptions.ts` | 前端订阅 API |
| `web/src/api/strategies.ts` | 前端策略 API |
| `web/src/components/SubscriptionPage.vue` | 订阅管理页 |
| `web/src/components/StrategyList.vue` | 策略列表页 |
| `web/src/components/StrategyBuilder.vue` | 策略编辑器 |
