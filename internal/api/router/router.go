package router

import (
	"log/slog"

	"stock-ai/internal/api/handler"
	"stock-ai/internal/backtest"
	"stock-ai/internal/config"
	"stock-ai/internal/datacollect"
	applog "stock-ai/internal/log"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// SubscriptionServiceRef 保存路由层创建的订阅服务引用，供 main.go 注入 Scheduler
var SubscriptionServiceRef *service.SubscriptionService

// DataCollectServiceRef 保存路由层创建的数据采集服务引用，供 main.go 注入 Scheduler
var DataCollectServiceRef *service.DataCollectTaskService

// MonitorConfigServiceRef 保存路由层创建的监控配置服务引用，供 main.go 注入 Monitor
var MonitorConfigServiceRef *service.MonitorConfigService

// PortfolioServiceRef 保存路由层创建的持仓服务引用，供 main.go 注入 QuoteCache
var PortfolioServiceRef *service.PortfolioService

// BacktestHandlerRef 保存路由层创建的回测 Handler 引用，供 main.go 注入依赖
var BacktestHandlerRef *backtest.Handler

// SetupRouter 创建路由并返回 gin.Engine 实例
func SetupRouter(runner *datacollect.DataCollectRunner) *gin.Engine {
	// 将 gin 的 debug/错误输出接入 slog，统一日志格式与染色
	if config.Get().Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	gin.DefaultWriter = applog.NewSlogLineWriter(slog.LevelDebug)
	gin.DefaultErrorWriter = applog.NewSlogLineWriter(slog.LevelError)

	r := gin.New()
	r.Use(gin.Recovery())

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// TraceID 链路追踪 + 结构化访问日志（在 CORS 之后、业务路由之前注册）
	r.Use(middleware.TraceID())
	r.Use(middleware.GinLogger())

	// 健康检查
	r.GET("/health", handler.HealthCheck)

	// 初始化服务
	cfg := config.Get()
	authSvc := service.NewAuthService(cfg.Auth.JWTSecret)
	screenSvc := service.NewScreenService()
	subSvc := service.NewSubscriptionService()
	dcSvc := service.NewDataCollectTaskService()
	monitorCfgSvc := service.NewMonitorConfigService()
	portfolioSvc := service.NewPortfolioService()

	// 保存服务引用，供 main.go 注入依赖
	SubscriptionServiceRef = subSvc
	DataCollectServiceRef = dcSvc
	MonitorConfigServiceRef = monitorCfgSvc
	PortfolioServiceRef = portfolioSvc

	// API v1 路由组
	apiV1 := r.Group("/api/v1")
	{
		RegisterAuthRoutes(apiV1, authSvc)
		RegisterStrategyRoutes(apiV1, authSvc)
		RegisterIndicatorRoutes(apiV1, authSvc, screenSvc)
		RegisterKLineRoutes(apiV1)
		RegisterBotRoutes(apiV1, authSvc)
		RegisterPortfolioRoutes(apiV1, authSvc, portfolioSvc)
		RegisterSubscriptionRoutes(apiV1, authSvc, subSvc)
		RegisterDataCollectRoutes(apiV1, authSvc, dcSvc, runner)
		RegisterMonitorConfigRoutes(apiV1, authSvc, monitorCfgSvc)

		// 回测路由（通过 BacktestHandlerRef 注入）
		if BacktestHandlerRef != nil {
			RegisterBacktestRoutes(apiV1, authSvc, BacktestHandlerRef)
		}
	}

	return r
}
