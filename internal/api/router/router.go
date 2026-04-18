package router

import (
	"stock-ai/internal/api/handler"

	"github.com/gin-gonic/gin"
)

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

	// ========== API v1 路由组 ==========
	apiV1 := r.Group("/api/v1")
	{
		// --- 股票选股相关接口 ---
		stockHandler := handler.NewStockHandler()
		stocks := apiV1.Group("/stocks")
		{
			stocks.POST("/filter", stockHandler.FilterStocks)
			stocks.POST("/ai-query", stockHandler.AIQuery)
			stocks.GET("/hot-topics", stockHandler.GetHotTopics)
			stocks.GET("/:code", stockHandler.GetStockDetail)
			stocks.GET("/:code/prices", stockHandler.GetStockPrices)
		}

		// --- 数据采集相关接口 ---
		dataHandler := handler.NewDataCollectorHandler()
		collector := apiV1.Group("/collector")
		{
			// 采集股票列表（外部定时调用）
			collector.POST("/stock-list", dataHandler.RunStockList)

			// 采集单只股票详情
			collector.POST("/stock-detail/:code", dataHandler.RunPriceData)
		}
	}

	return r
}
