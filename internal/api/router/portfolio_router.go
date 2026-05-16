package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterPortfolioRoutes 注册持仓管理相关路由
func RegisterPortfolioRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService) {
	portfolioHandler := handler.NewPortfolioHandler()
	authMid := middleware.AuthRequired(authSvc)

	// /api/v1/positions — 持仓管理（需登录）
	positions := apiV1.Group("/positions")
	positions.Use(authMid)
	{
		positions.GET("", portfolioHandler.ListPositions)           // 持仓列表
		positions.GET("/:id", portfolioHandler.GetPositionByID)     // 持仓详情
		positions.POST("", portfolioHandler.OpenPosition)            // 建仓
		positions.POST("/:id/buy", portfolioHandler.BuyMore)        // 加仓
		positions.POST("/:id/sell", portfolioHandler.SellPartial)   // 减仓
		positions.POST("/:id/close", portfolioHandler.ClosePosition) // 清仓
		positions.PUT("/:id", portfolioHandler.UpdatePositionNote) // 更新备注
		positions.DELETE("/:id", portfolioHandler.DeletePosition)   // 删除持仓
	}

	// /api/v1/user/trade-config — 交易配置（需登录）
	user := apiV1.Group("/user")
	user.Use(authMid)
	{
		user.GET("/trade-config", portfolioHandler.GetTradeConfig)    // 获取配置
		user.PUT("/trade-config", portfolioHandler.UpdateTradeConfig) // 更新配置
	}
}
