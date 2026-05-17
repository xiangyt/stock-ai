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
	"stock-ai/internal/api/handler"
	"stock-ai/internal/api/router"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/indicator"
	"stock-ai/internal/service"
	"stock-ai/internal/subscription"
	"stock-ai/internal/adapter/ths"
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
	var thsInstance *ths.Adapter // 保留THS引用，用于注入快照服务

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
		case "ths":
			thsInstance = ths.New()
			ds = thsInstance
			if err := ds.Init(nil); err != nil {
				log.Printf("初始化 %s 失败: %v", dsCfg.Name, err)
				thsInstance = nil // 初始化失败则清空
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

	// ====================================================================
	//  初始化策略订阅模块
	// ====================================================================

	// 1. 创建 QuoteCache（注入 registry，运行时动态获取数据源）
	var quoteCache subscription.QuoteCache
	if len(registry.Names()) > 0 {
		quoteCache = subscription.NewQuoteCache(registry, 3)
		quoteCache.Start()
		log.Println("✅ QuoteCache 已启动")
	} else {
		log.Println("⚠️ 无可用数据源，QuoteCache 未启动")
	}

	// 3. 创建指标注册表
	reg := indicator.NewRegistry(handler.AllBuiltins())

	// 4. 创建 Notifier
	notifier := subscription.NewNotifier()

	// 5. 创建 SubscriptionRunner
	runner := subscription.NewSubscriptionRunner(quoteCache, reg.Engine(), notifier)

	// 6. 创建 Scheduler
	scheduler := subscription.NewScheduler(runner)

	// 7. 设置 SubscriptionLoader（解耦 scheduler 对 db 的直接依赖）
	subscription.SetSubscriptionLoader(&dbSubscriptionLoader{})

	// 8. 启动 Scheduler
	if err := scheduler.Start(); err != nil {
		log.Printf("⚠️ Scheduler 启动失败: %v（订阅功能不可用）", err)
	} else {
		log.Println("✅ Scheduler 已启动")
	}

	// 9. 设置 Router（router 内部创建 subSvc，通过 SubscriptionServiceRef 获取）
	r := router.SetupRouter()

	// 注入 Scheduler 到 router 层创建的 SubscriptionService
	if router.SubscriptionServiceRef != nil {
		router.SubscriptionServiceRef.SetScheduler(scheduler)
	}

	// 启动 HTTP 服务
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
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
		log.Println("正在停止 Scheduler...")
		scheduler.Stop()

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

// ============================================================================
//  dbSubscriptionLoader 实现 subscription.SubscriptionLoader 接口
// ============================================================================

type dbSubscriptionLoader struct{}

// LoadActive 加载所有活跃订阅
func (l *dbSubscriptionLoader) LoadActive() ([]subscription.SubscriptionLoadResult, error) {
	subs, err := db.GetActiveSubscriptions()
	if err != nil {
		return nil, err
	}
	results := make([]subscription.SubscriptionLoadResult, len(subs))
	for i, sub := range subs {
		results[i] = subscription.SubscriptionLoadResult{
			ID:               sub.ID,
			UID:              sub.UID,
			Name:             sub.Name,
			StrategyID:       sub.StrategyID,
			Scope:            string(sub.Scope),
			CustomStocks:     sub.CustomStocks,
			PresetType:       string(sub.PresetType),
			CronExpr:         sub.CronExpr,
			TradingHoursOnly: sub.TradingHoursOnly,
			IsActive:         sub.IsActive,
			Template:         sub.Template,
		}
	}
	return results, nil
}

// LoadByID 根据 ID 加载订阅
func (l *dbSubscriptionLoader) LoadByID(id uint) (*subscription.SubscriptionLoadResult, error) {
	sub, err := db.GetSubscriptionByIDForScheduler(id)
	if err != nil {
		return nil, err
	}
	return &subscription.SubscriptionLoadResult{
		ID:               sub.ID,
		UID:              sub.UID,
		Name:             sub.Name,
		StrategyID:       sub.StrategyID,
		Scope:            string(sub.Scope),
		CustomStocks:     sub.CustomStocks,
		PresetType:       string(sub.PresetType),
		CronExpr:         sub.CronExpr,
		TradingHoursOnly: sub.TradingHoursOnly,
		IsActive:         sub.IsActive,
		Template:         sub.Template,
	}, nil
}
