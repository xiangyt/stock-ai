# AI Stock Picker Backend Makefile

.PHONY: build run dev init-db clean test lint

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
	@echo "  make build    - 构建二进制文件"
	@echo "  make run      - 构建并运行服务"
	@echo "  make dev      - 开发模式运行(不编译)"
	@echo "  make init-db  - 初始化数据库和模拟数据"
	@echo "  make clean    - 清理构建文件"
	@echo "  make test     - 运行测试"
	@echo "  make lint     - 代码格式化检查"
	@echo "  make deps     - 下载依赖"
