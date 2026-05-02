package router

import (
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/api/handler"
	"stock-ai/internal/config"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// SetupRouter 创建路由（无THS适配器，快照全日期路径不可用）
func SetupRouter() *gin.Engine {
	return SetupRouterWithTHS(nil)
}

// SetupRouterWithTHS 创建路由并注入 THS 适配器
func SetupRouterWithTHS(thsAdapter *ths.Adapter) *gin.Engine {
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

	// ========== 初始化认证服务和中间件 ==========
	cfg := config.Get()
	authSvc := service.NewAuthService(cfg.Auth.JWTSecret)
	authHandler := handler.NewAuthHandler(authSvc)
	authMiddleware := middleware.AuthRequired(authSvc)

	// ========== API v1 路由组 ==========
	apiV1 := r.Group("/api/v1")
	{
		// --- 认证接口（无需登录） ---
		auth := apiV1.Group("/auth")
		{
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

		// --- 每日估值快照接口（统一入口） ---
		snapHandler := handler.NewSnapshotHandler()
		apiV1.POST("/snapshot/calc", snapHandler.Calc) // code/date 为空=全部

		// --- 策略管理接口（需要登录） ---
		strategyHandler := handler.NewStrategyHandler()
		strategies := apiV1.Group("/strategies")
		strategies.Use(authMiddleware)
		{
			strategies.GET("", strategyHandler.List)          // 列表（搜索+分页）
			strategies.POST("", strategyHandler.Create)       // 创建
			strategies.GET("/:id", strategyHandler.GetByID)    // 详情
			strategies.PUT("/:id", strategyHandler.Update)     // 更新
			strategies.DELETE("/:id", strategyHandler.Delete)  // 删除单个
			strategies.PUT("/:id/rename", strategyHandler.Rename) // 重命名
			strategies.DELETE("/batch", strategyHandler.BatchDelete) // 批量删除
		}
	}

	return r
}
