# AI Stock Picker Backend

AI选股平台后端服务

## 技术栈

- Go 1.21+
- Gin
- GORM
- MySQL
- mcpkit

## 开发

```bash
# 下载依赖
go mod download

# 初始化数据库
go run cmd/server/main.go -init-data

# 开发模式运行
make dev

# 或使用自定义配置
go run cmd/server/main.go -config ./config.yaml
```

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/stocks/filter | 条件选股 |
| POST | /api/v1/stocks/ai-query | AI自然语言选股 |
| GET | /api/v1/stocks/hot-topics | 热门题材 |
| GET | /api/v1/stocks/:code | 股票详情 |
| GET | /api/v1/stocks/:code/prices | 价格历史 |
| GET | /health | 健康检查 |

## 配置

编辑 `config.yaml`:

```yaml
server:
  port: 8080

database:
  host: localhost
  port: 3306
  user: root
  password: ""
  dbname: ai_stock_picker
```
