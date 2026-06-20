//go:build wireinject
// +build wireinject

package main

import (
	"stock-ai/internal/adapter"
	"stock-ai/internal/api/handler"
	"stock-ai/internal/api/router"
	"stock-ai/internal/backtest"
	"stock-ai/internal/config"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
	"stock-ai/internal/monitor"
	"stock-ai/internal/notifier"
	"stock-ai/internal/subscription/quotecache"
	"stock-ai/internal/subscription/runner"
	subsched "stock-ai/internal/subscription/scheduler"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

// ============================================================================
//  Provider 辅助函数 — 将方法调用 / 单例获取包装为 wire 可用的函数
// ============================================================================

func provideRegistry() *adapter.Registry {
	return adapter.GetRegistry()
}

func provideQuoteSubscriber(cache quotecache.QuoteCache) model.QuoteSubscriber {
	return cache.Subscriber()
}

func provideEngine(reg *indicator.Registry) *indicator.Engine {
	return reg.Engine()
}

func provideRouter(dcRunner *datacollect.DataCollectRunner, btHandler *backtest.Handler) *gin.Engine {
	router.BacktestHandlerRef = btHandler
	return router.SetupRouter(dcRunner)
}

// ============================================================================
//  Wire 注入器入口
// ============================================================================

// InitializeApp 由 wire 生成完整组件图。
// cfg 由 main() 加载后传入，wire 将其注入到 App.Config 字段。
func InitializeApp(cfg *config.Config) (*App, error) {
	wire.Build(
		// --- 辅助 provider ---
		provideRegistry,
		provideQuoteSubscriber,
		provideEngine,
		provideRouter,

		// --- 行情缓存链路 ---
		quotecache.NewQuoteCache,
		quotecache.NewCachedQuoteProvider,

		// --- 指标引擎 ---
		handler.AllBuiltins,
		indicator.NewRegistry,

		// --- 通知器 ---
		notifier.NewNotifier,

		// --- 策略订阅 ---
		runner.NewSubscriptionRunner,
		subsched.NewScheduler,

		// --- 数据采集 ---
		datacollect.NewDataCollectRunner,
		wire.Bind(new(datacollect.TaskRunner), new(*datacollect.DataCollectRunner)),
		datacollect.NewScheduler,

		// --- 回测 ---
		backtest.NewService,
		backtest.NewHandler,

		// --- 盯盘监控 ---
		monitor.NewMonitor,

		// --- 聚合到 App 结构体 ---
		wire.Struct(new(App), "*"),
	)
	return &App{}, nil
}
