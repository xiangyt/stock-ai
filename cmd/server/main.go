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

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/adapter/tencentstock"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/adapter/ths2"
	"stock-ai/internal/api/router"
	"stock-ai/internal/config"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/db"
	"stock-ai/internal/holiday"
	"stock-ai/internal/subscription/monitor"
subsched "stock-ai/internal/subscription/scheduler"
)

func main() {
	// ====================================================================
	//  1. 加载配置
	// ====================================================================
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// ====================================================================
	//  2. 初始化数据库
	// ====================================================================
	if err := db.Init(&cfg.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// ====================================================================
	//  3. 加载法定节假日
	// ====================================================================
	if err := holiday.GetProvider().Load(); err != nil {
		log.Printf("⚠️ 节假日数据加载失败: %v", err)
	}

	// ====================================================================
	//  4. 初始化数据采集内置任务
	// ====================================================================
	if err := db.InitDataCollectTasks(); err != nil {
		log.Printf("⚠️ 初始化数据采集任务失败: %v", err)
	} else {
		log.Println("✅ 数据采集任务初始化完成")
	}

	// ====================================================================
	//  5. 注册数据源适配器（动态流程，wire 无法处理）
	// ====================================================================
	registry := adapter.GetRegistry()
	for _, dsCfg := range cfg.DataSources {
		if !dsCfg.Enabled {
			log.Printf("跳过未启用的数据源: %s", dsCfg.Name)
			continue
		}

		var ds adapter.DataSource
		switch dsCfg.Provider {
		case eastmoney.AdapterName:
			ds = eastmoney.New()
			initConfig := map[string]interface{}{
				"cookie": dsCfg.Cookie,
			}
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
				log.Printf("初始化 %s 失败: %v", dsCfg.Name, err)
				continue
			}
		case ths.AdapterName:
			ds = ths.New()
			if err := ds.Init(nil); err != nil {
				log.Printf("初始化 %s 失败: %v", dsCfg.Name, err)
				continue
			}
		case tencentstock.AdapterName:
			ds = tencentstock.New()
			if err := ds.Init(nil); err != nil {
				log.Printf("初始化 %s 失败: %v", dsCfg.Name, err)
				continue
			}
		case ths2.AdapterName:
			ds = ths2.New()
			initConfig := map[string]interface{}{
				"cookie": dsCfg.Cookie,
			}
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
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

	// ====================================================================
	//  6. Wire: 构建完整组件图
	// ====================================================================
	app, err := InitializeApp(cfg)
	if err != nil {
		log.Fatalf("初始化组件失败: %v", err)
	}

	// ====================================================================
	//  7. 启动运行时组件
	// ====================================================================
	if app.QuoteCache != nil {
		app.QuoteCache.Start()
		log.Println("✅ QuoteCache 已启动")
	}

	if app.SubRunner != nil {
		log.Println("✅ Runner 已创建")
	}

	if app.SubScheduler != nil {
		if err := app.SubScheduler.Start(); err != nil {
			log.Printf("⚠️ Subscription Scheduler 启动失败: %v（订阅功能不可用）", err)
		} else {
			log.Println("✅ Subscription Scheduler 已启动")
		}
	}

	if app.DCScheduler != nil {
		if err := app.DCScheduler.Start(); err != nil {
			log.Printf("⚠️ DataCollect Scheduler 启动失败: %v", err)
		} else {
			log.Println("✅ DataCollect Scheduler 已启动")
		}
	}

	if app.Monitor != nil {
		if err := app.Monitor.Start(); err != nil {
			log.Printf("⚠️ Monitor 启动失败: %v", err)
		} else {
			log.Println("✅ Monitor 已启动")
		}
	}

	// ====================================================================
	//  8. 注入回调（router 内部创建 Service → 通过 package 变量暴露引用）
	// ====================================================================

	// 策略订阅: SetRunner + SetNotifyChange
	if router.SubscriptionServiceRef != nil && app.SubRunner != nil {
		router.SubscriptionServiceRef.SetRunner(app.SubRunner)
	}
	if router.SubscriptionServiceRef != nil && app.SubScheduler != nil {
		router.SubscriptionServiceRef.SetNotifyChange(func(ct subsched.ChangeType, id uint) {
			app.SubScheduler.NotifyChange(subsched.SubscriptionChange{Type: ct, ID: id})
		})
	}

	// 盯盘监控: NotifyChange 回调
	if router.MonitorConfigServiceRef != nil && app.Monitor != nil {
		router.MonitorConfigServiceRef.SetNotifyChange(func(ct monitor.ChangeType, id uint) {
			app.Monitor.NotifyChange(monitor.ConfigChange{Type: ct, ConfigID: id})
		})
	}

	// 数据采集: NotifyChange 回调
	if router.DataCollectServiceRef != nil && app.DCScheduler != nil {
		router.DataCollectServiceRef.SetNotifyChange(func(ct datacollect.ChangeType, id uint) {
			app.DCScheduler.NotifyChange(datacollect.TaskChange{Type: ct, TaskID: id})
		})
	}

	// 持仓管理: 注入 QuoteCache 用于获取现价
	if router.PortfolioServiceRef != nil && app.QuoteCache != nil {
		router.PortfolioServiceRef.SetQuoteCache(app.QuoteCache)
	}

	// ====================================================================
	//  9. 启动 HTTP 服务
	// ====================================================================
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      app.Router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// 优雅关闭
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("正在关闭服务...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 优先停止 Scheduler
		if app.SubScheduler != nil {
			log.Println("正在停止 Subscription Scheduler...")
			app.SubScheduler.Stop()
		}

		// 停止 DataCollect Scheduler
		if app.DCScheduler != nil {
			log.Println("正在停止 DataCollect Scheduler...")
			app.DCScheduler.Stop()
		}

		// 停止 QuoteCache
		if app.QuoteCache != nil {
			log.Println("正在停止 QuoteCache...")
			app.QuoteCache.Stop()
		}

		// 停止 Monitor
		if app.Monitor != nil {
			log.Println("正在停止 Monitor...")
			app.Monitor.Stop()
		}

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP 服务关闭错误: %v", err)
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
