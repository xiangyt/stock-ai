package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterMonitorConfigRoutes 注册盯盘助手相关路由 /api/v1/monitor-configs/*
func RegisterMonitorConfigRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService, monitorCfgSvc *service.MonitorConfigService) {
	monitorHandler := handler.NewMonitorConfigHandler(monitorCfgSvc)
	authMid := middleware.AuthRequired(authSvc)

	cfgs := apiV1.Group("/monitor-configs")
	cfgs.Use(authMid)
	{
		cfgs.GET("", monitorHandler.List)
		cfgs.POST("", monitorHandler.Create)
		cfgs.GET("/:id", monitorHandler.GetByID)
		cfgs.PUT("/:id", monitorHandler.Update)
		cfgs.DELETE("/:id", monitorHandler.Delete)
		cfgs.PATCH("/:id/active", monitorHandler.SetActive)
		cfgs.PUT("/:id/bots", monitorHandler.UpdateBots)
	}
}
