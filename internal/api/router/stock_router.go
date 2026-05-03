package router

import (
	"stock-ai/internal/api/handler"

	"github.com/gin-gonic/gin"
)

// RegisterStockRoutes 注册股票选股相关路由 /api/v1/stocks/*
func RegisterStockRoutes(apiV1 *gin.RouterGroup) {
	stockHandler := handler.NewStockHandler()
	stocks := apiV1.Group("/stocks")
	{
		stocks.POST("/filter", stockHandler.FilterStocks)
		stocks.POST("/ai-query", stockHandler.AIQuery)
		stocks.GET("/hot-topics", stockHandler.GetHotTopics)
		stocks.GET("/:code", stockHandler.GetStockDetail)
		stocks.GET("/:code/prices", stockHandler.GetStockPrices)
	}
}
