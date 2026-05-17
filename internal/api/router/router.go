package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/config"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// SubscriptionServiceRef 保存路由层创建的订阅服务引用，供 main.go 注入 Scheduler
var SubscriptionServiceRef *service.SubscriptionService

// SetupRouter 创建路由并返回 gin.Engine 实例
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 健康检查
	r.GET("/health", handler.HealthCheck)

	// 初始化服务
	cfg := config.Get()
	authSvc := service.NewAuthService(cfg.Auth.JWTSecret)
	screenSvc := service.NewScreenService()
	subSvc := service.NewSubscriptionService()

	// 保存订阅服务引用，供 main.go 注入 Scheduler
	SubscriptionServiceRef = subSvc

	// API v1 路由组
	apiV1 := r.Group("/api/v1")
	{
		RegisterAuthRoutes(apiV1, authSvc)
		RegisterStockRoutes(apiV1)
		RegisterCollectorRoutes(apiV1)
		RegisterStrategyRoutes(apiV1, authSvc)
		RegisterIndicatorRoutes(apiV1, authSvc, screenSvc)
		RegisterKLineRoutes(apiV1)
		RegisterBotRoutes(apiV1, authSvc)
		RegisterPortfolioRoutes(apiV1, authSvc)
		RegisterSubscriptionRoutes(apiV1, authSvc, subSvc)
	}

	return r
}
