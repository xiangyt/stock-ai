# AI Stock Picker Backend Makefile

.PHONY: build build-web build-all run dev sync-kline-init sync-kline-daily sync-kline-fill init-db clean test lint

# 默认配置路径
CONFIG_FILE ?= config.yaml

# 前端源码目录
WEB_DIR ?= web

# 构建后端（不含前端，仅用于纯 API 场景）
build:
	go build -o bin/server ./cmd/server/

# 构建前端
build-web:
	cd $(WEB_DIR) && npm ci && npm run build

# 全量构建（前端 + 嵌入前端的后端）
build-all: build-web
	@# 清理旧的嵌套 dist 目录（如果存在）
	@rm -rf cmd/server/dist/dist
	@# 将 web/dist/ 的内容复制到 cmd/server/dist/（注意末尾 / 复制内容而非目录本身）
	cp -r $(WEB_DIR)/dist/ cmd/server/dist/
	go build -o bin/server ./cmd/server/

# 运行（使用内嵌前端，需要先 build-all）
run: build-all
	./bin/server -config $(CONFIG_FILE)

# 开发模式运行（后端独立，前端另行 npm run dev）
dev:
	go run ./cmd/server/ -config $(CONFIG_FILE)

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
	go run ./cmd/server/ -config $(CONFIG_FILE) -init-data

# 清理
clean:
	@# 保留 .gitkeep，删构建产物
	@find cmd/server/dist -not -name '.gitkeep' -not -path 'cmd/server/dist' -delete 2>/dev/null || true
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
	@echo "  make build              - 仅构建后端二进制（不含前端）"
	@echo "  make build-web           - 仅构建前端静态文件"
	@echo "  make build-all           - 全量构建（前端+嵌入前端的后端）"
	@echo "  make run                 - 全量构建并运行服务"
	@echo "  make dev                 - 开发模式运行后端（前端需单独 npm run dev）"
	@echo "  make sync-kline-init     - 初始化: 同花顺全量拉取骨架数据"
	@echo "  make sync-kline-daily    - 每日增量: 同花顺GetToday获取当期(定时任务)"
	@echo "  make sync-kline-fill     - 补全金额: 东财补amount=0的记录(低频)"
	@echo "  make init-db             - 初始化数据库和模拟数据"
	@echo "  make clean               - 清理构建文件"
	@echo "  make test                - 运行测试"
	@echo "  make lint                - 代码格式化检查"
	@echo "  make deps                - 下载依赖"
