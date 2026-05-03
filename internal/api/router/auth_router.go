package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterAuthRoutes 注册认证相关路由 /api/v1/auth/*
func RegisterAuthRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService) {
	authHandler := handler.NewAuthHandler(authSvc)
	authMiddleware := middleware.AuthRequired(authSvc)

	auth := apiV1.Group("/auth")
	{
		// 无需登录
		auth.POST("/login", authHandler.Login)
		auth.POST("/register", authHandler.Register)

		// 需要登录的 auth 接口
		me := auth.Group("")
		me.Use(authMiddleware)
		{
			me.GET("/me", authHandler.GetMe)
			me.PUT("/account", authHandler.UpdateAccount)
		}

		// 管理员接口（需登录 + admin 角色）
		adminAuth := auth.Group("/admin")
		adminAuth.Use(authMiddleware)
		{
			adminAuth.GET("/users", authHandler.ListUsers)
			adminAuth.PUT("/users/:id/status", authHandler.ToggleUserStatus)
			adminAuth.PUT("/users/:id/password", authHandler.ResetUserPassword)
		}
	}
}
