package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterStrategyRoutes 注册策略管理路由 /api/v1/strategies/*
func RegisterStrategyRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService) {
	strategyHandler := handler.NewStrategyHandler()
	authMiddleware := middleware.AuthRequired(authSvc)

	strategies := apiV1.Group("/strategies")
	strategies.Use(authMiddleware)
	{
		strategies.GET("", strategyHandler.List)           // 列表（搜索+分页）
		strategies.POST("", strategyHandler.Create)        // 创建
		strategies.GET("/:id", strategyHandler.GetByID)    // 详情
		strategies.PUT("/:id", strategyHandler.Update)     // 更新
		strategies.DELETE("/:id", strategyHandler.Delete)  // 删除单个
		strategies.PUT("/:id/rename", strategyHandler.Rename) // 重命名
		strategies.PUT("/:id/public", strategyHandler.SetPublic) // 切换公开/私有
		strategies.POST("/:id/copy", strategyHandler.Copy)        // 复制策略
		strategies.DELETE("/batch", strategyHandler.BatchDelete) // 批量删除
	}
}
