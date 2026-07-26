package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
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
	applog "stock-ai/internal/log"
	"stock-ai/internal/service"
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
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	// 初始化结构化日志（slog）
	applog.Init()

	// 服务启动 trace_id，标识本次启动的后台流程；后续启动日志统一携带
	startupTraceID := applog.NewTraceID()
	startupCtx := applog.WithTraceID(context.Background(), startupTraceID)
	logger := slog.With("trace_id", startupTraceID)
	logger.Info("服务启动中")

	// ====================================================================
	//  2. 初始化数据库
	// ====================================================================
	if err := db.Init(&cfg.Database); err != nil {
		logger.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.AutoMigrate(); err != nil {
		logger.Error("数据库迁移失败", "error", err)
		os.Exit(1)
	}

	// ====================================================================
	//  3. 加载法定节假日
	// ====================================================================
	if err := holiday.GetProvider().Load(startupCtx); err != nil {
		logger.Warn("节假日数据加载失败", "error", err)
	}

	// ====================================================================
	//  4. 初始化数据采集内置任务
	// ====================================================================
	if err := db.InitDataCollectTasks(startupCtx); err != nil {
		logger.Warn("初始化数据采集任务失败", "error", err)
	} else {
		logger.Info("数据采集任务初始化完成")
	}

	// ====================================================================
	//  5. 注册数据源适配器（动态流程，wire 无法处理）
	// ====================================================================
	registry := adapter.GetRegistry()
	var emAdapter *eastmoney.Adapter
	for _, dsCfg := range cfg.DataSources {
		if !dsCfg.Enabled {
			logger.Info("跳过未启用的数据源", "name", dsCfg.Name)
			continue
		}

		var ds adapter.DataSource
		switch dsCfg.Provider {
		case eastmoney.AdapterName:
			ds = eastmoney.New()
			emAdapter = ds.(*eastmoney.Adapter)
			initConfig := map[string]interface{}{
				"cookie": dsCfg.Cookie,
			}
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
				logger.Error("初始化数据源失败", "name", dsCfg.Name, "error", err)
				continue
			}
		case ths.AdapterName:
			ds = ths.New()
			if err := ds.Init(nil); err != nil {
				logger.Error("初始化数据源失败", "name", dsCfg.Name, "error", err)
				continue
			}
		case tencentstock.AdapterName:
			ds = tencentstock.New()
			if err := ds.Init(nil); err != nil {
				logger.Error("初始化数据源失败", "name", dsCfg.Name, "error", err)
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
				logger.Error("初始化数据源失败", "name", dsCfg.Name, "error", err)
				continue
			}
		default:
			logger.Warn("未知的数据源类型", "name", dsCfg.Name, "provider", dsCfg.Provider)
			continue
		}

		if err := registry.Register(ds); err != nil {
			logger.Error("注册数据源失败", "name", dsCfg.Name, "error", err)
		} else {
			logger.Info("已注册数据源", "display_name", ds.DisplayName(), "name", dsCfg.Name)
		}
	}
	logger.Info("已注册数据源", "names", registry.Names())

	// ====================================================================
	//  5.5 应用 admin 用户在个人主页保存的软件配置（覆盖配置文件中的 cookie）
	// ====================================================================
	softConfigSvc := service.NewSoftwareConfigService()
	if err := softConfigSvc.LoadAndApplyAdminConfigs(); err != nil {
		logger.Warn("应用 admin 软件配置失败", "error", err)
	} else {
		logger.Info("admin 软件配置已应用")
	}

	// ====================================================================
	//  6. Wire: 构建完整组件图
	// ====================================================================
	app, err := InitializeApp(cfg)
	if err != nil {
		logger.Error("初始化组件失败", "error", err)
		os.Exit(1)
	}

	// 注入东财适配器到回测 Handler（用于自选股一键加入等）
	if emAdapter != nil {
		app.BtHandler.SetEMAdapter(emAdapter)
	}

	// ====================================================================
	//  7. 启动运行时组件
	// ====================================================================
	if app.QuoteCache != nil {
		app.QuoteCache.Start()
		logger.Info("QuoteCache 已启动")

		// 立即初始化优先级，避免组件启动后首个调度周期使用默认 Normal
		if app.WatchlistManager != nil {
			app.WatchlistManager.ReloadAll()
		}
	}

	if app.SubRunner != nil {
		logger.Info("Runner 已创建")
	}

	if app.SubScheduler != nil {
		if err := app.SubScheduler.Start(); err != nil {
			logger.Error("Subscription Scheduler 启动失败（订阅功能不可用）", "error", err)
		} else {
			logger.Info("Subscription Scheduler 已启动")
		}
	}

	if app.DCScheduler != nil {
		if err := app.DCScheduler.Start(); err != nil {
			logger.Error("DataCollect Scheduler 启动失败", "error", err)
		} else {
			logger.Info("DataCollect Scheduler 已启动")
		}
	}

	if app.Monitor != nil {
		if err := app.Monitor.Start(); err != nil {
			logger.Error("Monitor 启动失败", "error", err)
		} else {
			logger.Info("Monitor 已启动")
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

	// 盯盘助手: NotifyChange 回调
	if router.MonitorConfigServiceRef != nil && app.Monitor != nil {
		router.MonitorConfigServiceRef.SetNotifyChange(func(ct monitor.ChangeType, id uint) {
			app.Monitor.NotifyChange(monitor.ConfigChange{Type: ct, ConfigID: id})
		})
	}

	// 关注列表管理器: 注入到持仓/盯盘/订阅 Service
	if app.WatchlistManager != nil {
		if router.PortfolioServiceRef != nil {
			router.PortfolioServiceRef.SetWatchlistManager(app.WatchlistManager)
		}
		if router.MonitorConfigServiceRef != nil {
			router.MonitorConfigServiceRef.SetWatchlistManager(app.WatchlistManager)
		}
		if router.SubscriptionServiceRef != nil {
			router.SubscriptionServiceRef.SetWatchlistManager(app.WatchlistManager)
		}
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

	// 持仓管理: 注入持仓变动回调 → 通知 Monitor 重算 ScopeHeld
	if router.PortfolioServiceRef != nil && app.Monitor != nil {
		router.PortfolioServiceRef.SetNotifyHoldingChanged(app.Monitor.NotifyHoldingChanged)
	}

	// ====================================================================
	//  8. 注册前端静态文件（SPA fallback）
	// ====================================================================
	serveStatic(app.Router, cfg.Server.StaticDir)

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
		logger.Info("正在关闭服务...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 优先停止 Scheduler
		if app.SubScheduler != nil {
			logger.Info("正在停止 Subscription Scheduler...")
			app.SubScheduler.Stop()
		}

		// 停止 DataCollect Scheduler
		if app.DCScheduler != nil {
			logger.Info("正在停止 DataCollect Scheduler...")
			app.DCScheduler.Stop()
		}

		// 停止 QuoteCache
		if app.QuoteCache != nil {
			logger.Info("正在停止 QuoteCache...")
			app.QuoteCache.Stop()
		}

		// 停止 Monitor
		if app.Monitor != nil {
			logger.Info("正在停止 Monitor...")
			app.Monitor.Stop()
		}

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("HTTP 服务关闭错误", "error", err)
		}

		// 关闭数据源适配器
		logger.Info("正在关闭数据源连接...")
		registry.CloseAll()

		logger.Info("服务已关闭")
	}()

	// 启动服务
	logger.Info("HTTP 服务启动", "url", fmt.Sprintf("http://localhost:%d", cfg.Server.Port))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP 服务启动失败", "error", err)
		os.Exit(1)
	}
}
