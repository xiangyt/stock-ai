package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterBotRoutes 注册推送配置相关路由 /api/v1/bots/*
func RegisterBotRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService) {
	botHandler := handler.NewBotHandler()
	authMid := middleware.AuthRequired(authSvc)

	push := apiV1.Group("/bots")
	push.Use(authMid)
	{
		push.GET("", botHandler.List)
		push.POST("", botHandler.Create)
		push.PUT("/:id", botHandler.Update)
		push.DELETE("/:id", botHandler.Delete)
		push.PUT("/:id/status", botHandler.ToggleStatus)
		push.POST("/:id/test", botHandler.Test)
	}
}
