package router

import (
	"stock-ai/internal/api/handler"

	"github.com/gin-gonic/gin"
)

// RegisterCollectorRoutes 注册数据采集相关路由（含 K线同步 + 估值快照）
func RegisterCollectorRoutes(apiV1 *gin.RouterGroup) {
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

		// 基本面/财务面采集 - 单只
		fundamental := collector.Group("/fundamental")
		{
			fundamental.POST("/:code/performance", dataHandler.RunPerformanceReports)
			fundamental.POST("/:code/shareholder", dataHandler.RunShareholderCounts)
			fundamental.POST("/:code/share-change", dataHandler.RunShareChanges)
		}

		// 基本面/财务面采集 - 全量
		fundamentalBatch := collector.Group("/fundamental-batch")
		{
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
		syncKline.POST("/daily", syncHandler.RunDaily)  // 每日增量：同花顺 GetToday
		syncKline.POST("/fill", syncHandler.RunFill)    // 补全金额：东财补 amount=0
		syncKline.POST("/debug", syncHandler.Debug)     // 调试
	}

	// --- 每日估值快照接口 ---
	snapHandler := handler.NewSnapshotHandler()
	apiV1.POST("/snapshot/calc", snapHandler.Calc)
}
