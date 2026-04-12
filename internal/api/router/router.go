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
			// 采集任务管理
			collector.POST("/tasks", dataHandler.CreateTask)
			collector.GET("/tasks", dataHandler.ListTasks)
			collector.GET("/tasks/:id", dataHandler.GetTask)
			collector.DELETE("/tasks/:id", dataHandler.DeleteTask)
			
			// 手动触发采集
			collector.POST("/run/stock-list", dataHandler.RunStockList)
			collector.POST("/run/prices/:code", dataHandler.RunPriceData)
			collector.POST("/run/all-prices", dataHandler.RunAllPrices)
			
			// 采集状态查询
			collector.GET("/status", dataHandler.CollectorStatus)
			collector.GET("/logs", dataHandler.CollectorLogs)
			
			// 数据源配置
			collector.GET("/sources", dataHandler.GetDataSources)
			collector.PUT("/sources/:name", dataHandler.UpdateDataSource)
		}
	}

	return r
}
