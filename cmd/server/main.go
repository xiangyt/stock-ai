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
	"stock-ai/internal/api/handler"
	"stock-ai/internal/api/router"
	"stock-ai/internal/backtest"
	"stock-ai/internal/config"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/db"
	"stock-ai/internal/holiday"
	"stock-ai/internal/indicator"
	"stock-ai/internal/notifier"
	"stock-ai/internal/subscription/quotecache"
	"stock-ai/internal/subscription/runner"
	"stock-ai/internal/subscription/scheduler"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
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

	// 加载法定节假日数据到内存
	if err := holiday.GetProvider().Load(); err != nil {
		log.Printf("⚠️ 节假日数据加载失败: %v", err)
	}

	// 初始化数据采集内置任务
	if err := db.InitDataCollectTasks(); err != nil {
		log.Printf("⚠️ 初始化数据采集任务失败: %v", err)
	} else {
		log.Println("✅ 数据采集任务初始化完成")
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
		case eastmoney.AdapterName:
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
	//  初始化策略订阅模块
	// ====================================================================

	// 1. 创建 QuoteCache（注入 registry，运行时动态获取数据源）
	var quoteCache quotecache.QuoteCache
	if len(registry.Names()) > 0 {
		quoteCache = quotecache.NewQuoteCache(registry)
		quoteCache.Start()
		log.Println("✅ QuoteCache 已启动")
	} else {
		log.Println("⚠️ 无可用数据源，QuoteCache 未启动")
	}

	// 2. 创建 StockSourceProvider（CachedQuoteProvider 组合 QuoteCache 实现 Runner 接口）
	var provider runner.StockSourceProvider
	if quoteCache != nil {
		provider = quotecache.NewCachedQuoteProvider(quoteCache)
	}

	// 3. 创建指标注册表
	reg := indicator.NewRegistry(handler.AllBuiltins())

	// 4. 创建 Notifier
	ntf := notifier.NewNotifier()

	// 5. 创建 SubscriptionRunner（单例）
	var r *runner.SubscriptionRunner
	if provider != nil {
		r = runner.NewSubscriptionRunner(provider, reg.Engine(), ntf)
		log.Println("✅ Runner 已创建")
	} else {
		log.Println("⚠️ 无数据源，Runner 未创建")
	}

	// 6. 创建 Scheduler（只需 Runner）
	var sch scheduler.Scheduler
	if r != nil {
		sch = scheduler.NewScheduler(r)
		if err := sch.Start(); err != nil {
			log.Printf("⚠️ Scheduler 启动失败: %v（订阅功能不可用）", err)
		} else {
			log.Println("✅ Scheduler 已启动")
		}
	}

	// 8. 初始化数据采集运行器（供前端手动触发执行 + 调度器使用）
	dcRunner := datacollect.NewDataCollectRunner()

	// 9. 初始化回测服务
	var backtestHandler *backtest.Handler
	btSvc := backtest.NewService(reg.Engine())
	backtestHandler = backtest.NewHandler(btSvc)
	log.Println("✅ 回测服务已初始化")
	router.BacktestHandlerRef = backtestHandler

	// 10. 设置 Router（router 内部创建 subSvc，通过 SubscriptionServiceRef 获取）
	rtr := router.SetupRouter(dcRunner)

	// 注入 Runner 和 NotifyChange 回调到 SubscriptionService
	if router.SubscriptionServiceRef != nil {
		router.SubscriptionServiceRef.SetRunner(r)
		if sch != nil {
			router.SubscriptionServiceRef.SetNotifyChange(func(ct scheduler.ChangeType, id uint) {
				sch.NotifyChange(scheduler.SubscriptionChange{Type: ct, ID: id})
			})
		}
	}

	// ====================================================================
	//  初始化数据采集调度模块
	// ====================================================================

	var dcSch datacollect.Scheduler
	dcSch = datacollect.NewScheduler(dcRunner)
	if err := dcSch.Start(); err != nil {
		log.Printf("⚠️ DataCollect Scheduler 启动失败: %v", err)
	} else {
		log.Println("✅ DataCollect Scheduler 已启动")
	}

	// 注入 NotifyChange 回调到 DataCollectService
	if router.DataCollectServiceRef != nil {
		router.DataCollectServiceRef.SetNotifyChange(func(ct datacollect.ChangeType, id uint) {
			dcSch.NotifyChange(datacollect.TaskChange{Type: ct, TaskID: id})
		})
	}

	// 启动 HTTP 服务
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      rtr,
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
		if sch != nil {
			log.Println("正在停止 Scheduler...")
			sch.Stop()
		}

		// 停止 DataCollect Scheduler
		if dcSch != nil {
			log.Println("正在停止 DataCollect Scheduler...")
			dcSch.Stop()
		}

		// 停止 QuoteCache
		if quoteCache != nil {
			log.Println("正在停止 QuoteCache...")
			quoteCache.Stop()
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
