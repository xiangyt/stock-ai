package router

import (
	"stock-ai/internal/backtest"

	"github.com/gin-gonic/gin"
)

// RegisterBacktestRoutes 注册回测路由
func RegisterBacktestRoutes(rg *gin.RouterGroup, h *backtest.Handler) {
	// 策略回测
	rg.POST("/strategies/:id/backtest", h.Initiate)
	rg.GET("/strategies/:id/backtest/runs", h.GetRuns)

	// 回测详情
	rg.GET("/backtest/runs/:id", h.GetRun)
	rg.GET("/backtest/runs/:id/status", h.GetRunStatus)
	rg.GET("/backtest/runs/:id/trades", h.GetTrades)
	rg.GET("/backtest/runs/:id/snapshots", h.GetSnapshots)

	// 回测管理
	rg.DELETE("/backtest/runs/:id", h.DeleteRun)
}
