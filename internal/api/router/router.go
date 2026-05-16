package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/config"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

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
	}

	return r
}
