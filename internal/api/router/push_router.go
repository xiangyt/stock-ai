package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterPushRoutes 注册推送配置相关路由 /api/v1/push-configs/*
func RegisterPushRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService) {
	pushHandler := handler.NewPushHandler()
	authMid := middleware.AuthRequired(authSvc)

	push := apiV1.Group("/push-configs")
	push.Use(authMid)
	{
		push.GET("", pushHandler.List)
		push.POST("", pushHandler.Create)
		push.PUT("/:id", pushHandler.Update)
		push.DELETE("/:id", pushHandler.Delete)
		push.PUT("/:id/status", pushHandler.ToggleStatus)
		push.POST("/:id/test", pushHandler.Test)
	}
}
