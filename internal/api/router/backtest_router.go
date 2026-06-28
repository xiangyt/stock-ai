package router

import (
	"stock-ai/internal/backtest"
	"stock-ai/internal/middleware"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterBacktestRoutes 注册回测路由
func RegisterBacktestRoutes(rg *gin.RouterGroup, authSvc *service.AuthService, h *backtest.Handler) {
	rg.Use(middleware.AuthRequired(authSvc))

	// 策略回测
	rg.POST("/strategies/:id/backtest", h.Initiate)
	rg.GET("/strategies/:id/backtest/runs", h.GetRuns)

	// 回测详情
	rg.GET("/backtest/runs/:id", h.GetRun)
	rg.GET("/backtest/runs/:id/status", h.GetRunStatus)
	rg.GET("/backtest/runs/:id/trades", h.GetTrades)
	rg.GET("/backtest/runs/:id/snapshots", h.GetSnapshots)

	// 回测管理
	rg.POST("/backtest/runs/:id/stop", h.Stop)
	rg.DELETE("/backtest/runs/:id", h.DeleteRun)

	// 自选股
	rg.POST("/backtest/batch/favorites", h.BatchAddToFavorites)
}
