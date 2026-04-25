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

			// 采集单只股票K线
			collector.POST("/kline/:code", dataHandler.RunKLineData)

			// 全量采集所有股票K线
			collector.POST("/kline-batch", dataHandler.RunKLineBatch)

			// --- 基本面/财务面采集 ---
			fundamental := collector.Group("/fundamental")
			{
				// 单只
				fundamental.POST("/:code/performance", dataHandler.RunPerformanceReports)
				fundamental.POST("/:code/shareholder", dataHandler.RunShareholderCounts)
				fundamental.POST("/:code/share-change", dataHandler.RunShareChanges)
			}
			fundamentalBatch := collector.Group("/fundamental-batch")
			{
				// 全量
				fundamentalBatch.POST("/performance", dataHandler.RunPerformanceReportsBatch)
				fundamentalBatch.POST("/shareholder", dataHandler.RunShareholderCountsBatch)
				fundamentalBatch.POST("/share-change", dataHandler.RunShareChangesBatch)
			}
		}
		// --- K线同步接口（多周期三模式） ---
		syncHandler := handler.NewKLineSyncHandler()
		syncKline := apiV1.Group("/sync-kline")
		{
			syncKline.POST("/init", syncHandler.RunInit)   // 初始化：同花顺全量骨架
			syncKline.POST("/daily", syncHandler.RunDaily) // 每日增量：同花顺GetToday
			syncKline.POST("/fill", syncHandler.RunFill)   // 补全金额：东财补amount=0
			syncKline.POST("/debug", syncHandler.Debug)    // 调试
		}
	}

	return r
}
