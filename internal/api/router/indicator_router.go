package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterIndicatorRoutes 注册指标路由 /api/v1/indicators/*
func RegisterIndicatorRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService, screenSvc *service.ScreenService) {
	h := handler.NewIndicatorHandler(screenSvc)
	authMiddleware := middleware.AuthRequired(authSvc)

	// 保存指标注册表引用，供 main.go 注入 MCP 复用到选股工具
	IndicatorRegistryRef = h.Registry()

	indicators := apiV1.Group("/indicators")
	indicators.Use(authMiddleware)
	{
		indicators.GET("", h.ListIndicators)       // 全量指标元数据
		indicators.GET("/:id", h.GetIndicatorByID) // 单个指标详情
		indicators.POST("/execute", h.Execute)     // 执行选股筛选
	}
}
