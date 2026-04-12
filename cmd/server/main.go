package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-stock-picker/internal/api/router"
	"ai-stock-picker/internal/adapter/eastmoney"
	"ai-stock-picker/internal/adapter/ths"
	"ai-stock-picker/internal/adapter"
	"ai-stock-picker/internal/config"
	"ai-stock-picker/internal/db"
	"ai-stock-picker/internal/mcp"
	"ai-stock-picker/internal/service"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	initData := flag.Bool("init-data", false, "初始化模拟数据")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	if err := db.Init(&cfg.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 自动迁移表结构
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// 注册数据源适配器
	registry := adapter.GetRegistry()

	// 注册东方财富
	emAdapter := eastmoney.New()
	if err := emAdapter.Init(nil); err != nil {
		log.Printf("初始化东方财富适配器失败: %v", err)
	} else if err := registry.Register(emAdapter); err != nil {
		log.Printf("注册东方财富失败: %v", err)
	} else {
		log.Println("✅ 已注册数据源: 东方财富 (eastmoney)")
	}

	// 注册同花顺（骨架）
	thsAdapter := ths.New()
	if err := registry.Register(thsAdapter); err != nil {
		log.Printf("注册同花顺失败: %v", err)
	} else {
		log.Println("✅ 已注册数据源: 同花顺 (ths) [骨架]")
	}

	log.Printf("已注册数据源: %v", registry.Names())

	// 初始化模拟数据
	if *initData {
		stockService := service.NewStockService()
		if err := stockService.InitMockData(); err != nil {
			log.Fatalf("初始化模拟数据失败: %v", err)
		}
		log.Println("模拟数据初始化完成")
		return
	}

	// 创建 HTTP 路由
	r := router.SetupRouter()

	// 启动 HTTP 服务
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// 启动 MCP Server (如果启用)
	var mcpServer *mcp.StockMCPServer
	if cfg.MCP.Enabled {
		mcpServer = mcp.NewStockMCPServer(cfg.MCP.Name, cfg.MCP.Version)
		go func() {
			log.Printf("MCP Server 启动: %s v%s", cfg.MCP.Name, cfg.MCP.Version)
			if err := mcpServer.Start(); err != nil {
				log.Printf("MCP Server 错误: %v", err)
			}
		}()
	}

	// 优雅关闭
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("正在关闭服务...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP 服务关闭错误: %v", err)
		}

		if mcpServer != nil {
			if err := mcpServer.Stop(); err != nil {
				log.Printf("MCP Server 关闭错误: %v", err)
			}
		}

		// 关闭数据源适配器
		log.Println("正在关闭数据源连接...")
		registry.CloseAll()

		log.Println("服务已关闭")
	}()

	// 启动服务
	log.Printf("HTTP 服务启动: http://localhost:%d", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP 服务启动失败: %v", err)
	}
}
