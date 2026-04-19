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

	"stock-ai/internal/api/router"
	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/adapter"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	// "stock-ai/internal/mcp" // TODO: 待升级到新版 mcpkit API 后启用
	"stock-ai/internal/service"
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

	for _, dsCfg := range cfg.DataSources {
		if !dsCfg.Enabled {
			log.Printf("跳过未启用的数据源: %s", dsCfg.Name)
			continue
		}

		var ds adapter.DataSource
		switch dsCfg.Provider {
		case "eastmoney":
			ds = eastmoney.New()
			initConfig := map[string]interface{}{
				"cookie": dsCfg.Cookie,
			}
			// 合并 extra 参数
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
				log.Printf("初始化 %s 失败: %v", dsCfg.Name, err)
				continue
			}
		case "ths":
			ds = ths.New()
			if err := ds.Init(nil); err != nil {
				log.Printf("初始化 %s 失败: %v", dsCfg.Name, err)
				continue
			}
		default:
			log.Printf("未知的数据源类型: %s (provider=%s)", dsCfg.Name, dsCfg.Provider)
			continue
		}

		if err := registry.Register(ds); err != nil {
			log.Printf("注册数据源 %s 失败: %v", dsCfg.Name, err)
		} else {
			log.Printf("✅ 已注册数据源: %s (%s)", ds.DisplayName(), dsCfg.Name)
		}
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
	// TODO: 待升级到新版 mcpkit API 后启用
	// var mcpServer *mcp.StockMCPServer
	// if cfg.MCP.Enabled {
	// 	mcpServer = mcp.NewStockMCPServer(cfg.MCP.Name, cfg.MCP.Version)
	// 	go func() {
	// 		log.Printf("MCP Server 启动: %s v%s", cfg.MCP.Name, cfg.MCP.Version)
	// 		if err := mcpServer.Start(); err != nil {
	// 			log.Printf("MCP Server 错误: %v", err)
	// 		}
	// 	}()
	// }

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

		// TODO: 待升级到新版 mcpkit API 后启用
		// if mcpServer != nil {
		// 	if err := mcpServer.Stop(); err != nil {
		// 		log.Printf("MCP Server 关闭错误: %v", err)
		// 	}
		// }

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
