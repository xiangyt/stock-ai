package main

import (
	"stock-ai/internal/adapter"
	"stock-ai/internal/backtest"
	"stock-ai/internal/config"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/model"
	"stock-ai/internal/subscription/monitor"
	"stock-ai/internal/notifier"
	"stock-ai/internal/subscription/quotecache"
	"stock-ai/internal/subscription/runner"
	subsched "stock-ai/internal/subscription/scheduler"
	"stock-ai/internal/subscription/watchlist"

	"github.com/gin-gonic/gin"
)

// App 聚合所有运行时组件，由 wire 自动构建依赖关系。
//
// 字段按生命周期分组:
//   Config       — 配置（由 main 加载后传入 wire）
//   Registry     — 数据源注册中心（单例）
//   QuoteCache → StockProvider → SubRunner → SubScheduler  — 策略订阅链路
//   QuoteCache → QuoteSub → Monitor                        — 盯盘助手链路
//   AllBuiltins → IndicatorReg → Engine                    — 指标引擎链路
//   Notifier     — 推送通知（被 Runner / Monitor 共用）
//   DCRunner → DCScheduler                                 — 数据采集链路
//   BtService → BtHandler                                  — 回测链路
//   Router       — HTTP 路由（注入 BacktestHandlerRef）
type App struct {
	Config        *config.Config

	// 数据源
	Registry *adapter.Registry

	// 行情缓存链路
	QuoteCache      quotecache.QuoteCache
	WatchlistManager *watchlist.Manager
	StockProvider   runner.StockSourceProvider
	QuoteSub      model.QuoteSubscriber

	// 指标引擎
	IndicatorReg *indicator.Registry
	Engine       *indicator.Engine

	// 公共通知
	Notifier notifier.Notifier

	// 策略订阅
	SubRunner    *runner.SubscriptionRunner
	SubScheduler subsched.Scheduler

	// 数据采集
	DCRunner    *datacollect.DataCollectRunner
	DCScheduler datacollect.Scheduler

	// 回测
	BtFactory *backtest.EngineFactory
	BtHandler *backtest.Handler

	// 盯盘助手
	Monitor *monitor.Monitor

	// HTTP 路由
	Router *gin.Engine
}
