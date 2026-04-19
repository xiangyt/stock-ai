# AI Stock Picker Backend Makefile

.PHONY: build run dev sync-kline-init sync-kline-daily sync-kline-fill init-db clean test lint

# 默认配置路径
CONFIG_FILE ?= config.yaml

# 构建
build:
	go build -o bin/server cmd/server/main.go

# 运行
run: build
	./bin/server -config $(CONFIG_FILE)

# 开发模式运行
dev:
	go run cmd/server/main.go -config $(CONFIG_FILE)

# ========== K线同步（多周期三模式） ==========
# 周期通过 -periods 指定，默认 daily,weekly,monthly,yearly

# 初始化：同花顺全量拉取骨架数据
sync-kline-init:
	go run cmd/sync-kline/main.go init -config $(CONFIG_FILE)

# 每日增量：同花顺 GetToday 等接口获取当期数据（建议定时任务每天跑）
sync-kline-daily:
	go run cmd/sync-kline/main.go daily -config $(CONFIG_FILE)

# 补全金额：东财全量拉取补 amount=0 的记录（建议每周低频跑）
sync-kline-fill:
	go run cmd/sync-kline/main.go fill -periods daily -config $(CONFIG_FILE)

# 初始化数据库(生成模拟数据)
init-db:
	go run cmd/server/main.go -config $(CONFIG_FILE) -init-data

# 清理
 clean:
	rm -rf bin/
	go clean

# 测试
test:
	go test -v ./...

# 代码检查
lint:
	gofmt -w .
	go vet ./...

# 下载依赖
deps:
	go mod download
	go mod tidy

# 生成 Swagger 文档(如需要)
swag:
	which swag || go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/server/main.go

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build            - 构建二进制文件"
	@echo "  make run              - 构建并运行服务"
	@echo "  make dev              - 开发模式运行(不编译)"
	@echo "  make sync-kline-init   - 初始化: 同花顺全量拉取骨架数据"
	@echo "  make sync-kline-daily  - 每日增量: 同花顺GetToday获取当期(定时任务)"
	@echo "  make sync-kline-fill   - 补全金额: 东财补amount=0的记录(低频)"
	@echo "  make init-db           - 初始化数据库和模拟数据"
	@echo "  make clean             - 清理构建文件"
	@echo "  make test              - 运行测试"
	@echo "  make lint              - 代码格式化检查"
	@echo "  make deps              - 下载依赖"
