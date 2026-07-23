# AI Stock Picker

智能选股与策略回测平台。支持自然语言选股、技术指标筛选、多数据源同步、可插拔回测引擎。

## 功能特性

- **AI 自然语言选股** — 用中文描述选股条件，AI 自动翻译为筛选规则
- **条件选股** — 基于技术指标（均线、MACD、KDJ 等）的多条件组合筛选
- **策略回测** — 完整的回测引擎，支持可插拔卖出规则和仓位分配策略
- **多数据源** — 同花顺、东方财富、腾讯股票三路数据源互备
- **K 线管理** — 日/周/月/年多周期 K 线同步与补全
- **MCP Server** — 对外暴露 Model Context Protocol 接口，可被 AI Agent 调用
- **Web 前端** — Vue 3 + ECharts 可视化界面

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25+ / Gin / GORM / MySQL / Wire (DI) |
| 前端 | Vue 3.5+ / TypeScript / Vite 8 / ECharts 6 |
| 数据源 | 同花顺 / 东方财富 / 腾讯股票 |
| 协议 | REST API + MCP (stdio/SSE) |

## 项目结构

```
ai-stock-picker/
├── cmd/                      # 入口程序
│   ├── server/               #   HTTP 服务 + MCP Server
│   ├── sync-kline/           #   K 线同步任务 (init/daily/fill)
│   ├── run-indicator/        #   技术指标计算
│   ├── strategy-backtest/    #   策略回测 CLI
│   └── ...                   #   其他工具命令
├── internal/
│   ├── adapter/              #   数据源适配器 (eastmoney/ths/tencent)
│   ├── api/                  #   HTTP handler + router + middleware
│   ├── backtest/             #   回测引擎 (types/exit/alloc 三层分离)
│   ├── config/               #   配置加载 (Viper)
│   ├── db/                   #   数据库访问层
│   ├── model/                #   数据模型
│   ├── service/              #   业务逻辑层
│   ├── mcp/                  #   MCP Server 实现
│   └── ...                   #   holiday/notifier/subscription 等
├── web/                      # Vue 3 前端
├── sql/                      # 数据库迁移脚本
├── docs/                     # 设计文档
└── config.yaml.example       # 配置示例
```

## 快速开始

### 环境要求

- Go 1.21+
- MySQL 5.7+ / 8.0+
- Node.js 18+ (前端开发)

### 1. 克隆与配置

```bash
git clone <repo-url>
cd ai-stock-picker
cp config.yaml.example config.yaml   # 编辑数据库连接等信息
```

### 2. 初始化数据库

```bash
# 执行 SQL 迁移脚本 (sql/ 目录下的 .sql 文件)
mysql -u root -p your_database < sql/schema.sql

# 可选: 生成模拟数据
go run cmd/server/main.go -config config.yaml -init-data
```

### 3. 启动后端服务

```bash
# 开发模式
make dev

# 或构建后运行
make build && make run
```

### 4. 启动前端 (可选)

```bash
cd web
npm install
npm run dev
```

## 常用命令

| 命令 | 说明 |
|------|------|
| `make dev` | 开发模式启动后端 |
| `make build` | 构建二进制文件 |
| `make test` | 运行测试 |
| `make lint` | 代码格式化 + 静态检查 |
| `make sync-kline-init` | 全量拉取 K 线骨架数据 (首次) |
| `make sync-kline-daily` | 每日增量同步 K 线 (定时任务) |
| `make sync-kline-fill` | 补全缺失金额字段 (低频运行) |

## API 接口

### 选股

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/stocks/filter` | 条件选股 |
| POST | `/api/v1/stocks/ai-query` | AI 自然语言选股 |
| GET | `/api/v1/stocks/hot-topics` | 热门题材 |

### 股票

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/stocks/:code` | 股票详情 |
| GET | `/api/v1/stocks/:code/prices` | 价格历史 |

### 回测

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/strategies/:id/backtest` | 发起回测 |
| GET | `/api/v1/backtest/runs/:id/status` | 查询回测状态 |
| GET | `/api/v1/backtest/runs/:id` | 获取回测结果 |
| GET | `/api/v1/backtest/runs/:id/trades` | 交易记录 |
| GET | `/api/v1/backtest/runs/:id/snapshots` | 净值曲线 |

### 系统

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |

## 配置说明

编辑 `config.yaml`:

```yaml
server:
  port: 8080
  mode: debug                    # debug / release

database:
  host: localhost
  port: 3306
  user: root
  password: "your_password"
  dbname: stock

# 数据源 Cookie (从浏览器 F12 复制或设置环境变量)
data_sources:
  - name: eastmoney
    enabled: true
    provider: eastmoney
    cookie: ""                   # 或 EM_COOKIE 环境变量
  - name: ths
    enabled: true
    provider: ths
    cookie: ""                   # 或 THS_COOKIE 环境变量

mcp:
  enabled: true
  transport: stdio               # stdio / sse

auth:
  jwt_secret: "change-me"        # 生产环境务必修改
  jwt_expire_days: 7
```

## 回测引擎

回测引擎采用 **异步执行 + 状态轮询** 模式，支持可插拔的卖出规则和仓位分配策略。

### 内置卖出规则

| 规则 | 说明 | 参数示例 |
|------|------|---------|
| stop_loss | 固定止损 | `threshold_pct: -8` |
| take_profit | 固定止盈 | `threshold_pct: 20` |
| time_exit | 到期退出 | `hold_days: 60` |
| trailing_stop | 移动止盈 | `trail_pct: 5, activation_pct: 10` |
| segment_profit | 分段止盈 | `levels: [{threshold_pct: 10, sell_ratio: 0.5}]` |

### 内置仓位分配器

| 分配器 | 说明 |
|--------|------|
| equal | 等权分配 |
| signal_weighted | 信号评分加权 |
| volatility_weighted | 波动率倒数加权 |
| risk_parity | 风险平价 (1/vol²) |
| custom_weight | 手动权重 |

### 绩效指标

总收益率 / 年化收益率 / 最大回撤 / 夏普比率 / 胜率 / 盈亏比

> 详细设计文档见 [docs/backtest-engine.md](docs/backtest-engine.md)

## 开发规范

- Go 编码遵循项目 `.codebuddy/rules/` 中的规范
- 函数不超过 80 行，单文件不超过 500 行
- 所有 error 必须处理，禁止 `_` 忽略
- 导出函数必须有 GoDoc 注释
- 测试覆盖正常路径、空值、错误路径、边界值

## License

MIT
