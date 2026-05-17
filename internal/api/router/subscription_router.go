package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterSubscriptionRoutes 注册策略订阅相关路由 /api/v1/subscriptions/*
func RegisterSubscriptionRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService, subSvc *service.SubscriptionService) {
	subHandler := handler.NewSubscriptionHandler(subSvc)
	authMid := middleware.AuthRequired(authSvc)

	subs := apiV1.Group("/subscriptions")
	subs.Use(authMid)
	{
		subs.GET("", subHandler.List)
		subs.POST("", subHandler.Create)
		subs.GET("/:id", subHandler.GetByID)
		subs.PUT("/:id", subHandler.Update)
		subs.DELETE("/:id", subHandler.Delete)
		subs.PATCH("/:id/active", subHandler.SetActive)
		subs.POST("/:id/run", subHandler.TriggerRun)
		subs.PUT("/:id/bots", subHandler.UpdateBots)
	}
}
