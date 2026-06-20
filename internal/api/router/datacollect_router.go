package router

import (
	"stock-ai/internal/api/handler"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterDataCollectRoutes 注册数据采集相关路由（含采集执行 + K线同步 + 快照 + 定时任务管理）
func RegisterDataCollectRoutes(apiV1 *gin.RouterGroup, authSvc *service.AuthService, dcSvc *service.DataCollectTaskService, runner *datacollect.DataCollectRunner) {
	dcHandler := handler.NewDataCollectHandler(dcSvc, runner)

	// --- 采集执行接口（无需鉴权，供外部/内部定时调用） ---
	collector := apiV1.Group("/collector")
	{
		collector.POST("/stock-list", dcHandler.RunStockList)
		collector.POST("/stock-detail/:code", dcHandler.RunPriceData)
		collector.POST("/kline/:code", dcHandler.RunKLineData)
		collector.POST("/kline-batch", dcHandler.RunKLineBatch)

		fundamental := collector.Group("/fundamental")
		{
			fundamental.POST("/:code/performance", dcHandler.RunPerformanceReports)
			fundamental.POST("/:code/shareholder", dcHandler.RunShareholderCounts)
			fundamental.POST("/:code/share-change", dcHandler.RunShareChanges)
			fundamental.POST("/:code/dividend", dcHandler.RunDividendHistory)
			fundamental.POST("/:code/name-change", dcHandler.RunNameChanges)
		}

		fundamentalBatch := collector.Group("/fundamental-batch")
		{
			fundamentalBatch.POST("/performance", dcHandler.RunPerformanceReportsBatch)
			fundamentalBatch.POST("/shareholder", dcHandler.RunShareholderCountsBatch)
			fundamentalBatch.POST("/share-change", dcHandler.RunShareChangesBatch)
			fundamentalBatch.POST("/dividend", dcHandler.RunDividendHistoryBatch)
			fundamentalBatch.POST("/name-change", dcHandler.RunNameChangesBatch)
		}

		snapshot := collector.Group("/snapshot")
		snapshot.POST("/calc", dcHandler.Calc)
	}

	// --- K线同步接口 ---
	syncKline := apiV1.Group("/sync-kline")
	{
		syncKline.POST("/init", dcHandler.RunInit)
		syncKline.POST("/daily", dcHandler.RunDaily)
		syncKline.POST("/fill", dcHandler.RunFill)
		syncKline.POST("/dividend", dcHandler.RunDividend)
		syncKline.POST("/debug", dcHandler.Debug)
	}

	// --- 定时任务管理接口（需鉴权 + 管理员权限） ---
	authMid := middleware.AuthRequired(authSvc)

	dc := apiV1.Group("/datacollect")
	dc.Use(authMid)
	{
		dc.GET("", dcHandler.List)
		dc.GET("/:id", dcHandler.GetByID)
		dc.PUT("/:id", dcHandler.Update)
		dc.PUT("/:id/bots", dcHandler.UpdateBots)
		dc.POST("/:id/execute", dcHandler.Execute)
	}
}
