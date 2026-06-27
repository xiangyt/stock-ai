package backtest

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 回测 HTTP 处理器。
type Handler struct {
	engine Engine
	dao    *DAO
}

// NewHandler 创建回测 Handler。
func NewHandler(engine Engine) *Handler {
	return &Handler{engine: engine, dao: NewDAO()}
}

// ============================================================================
//  P0 API
// ============================================================================

// Initiate POST /api/v1/strategies/:id/backtest
func (h *Handler) Initiate(c *gin.Context) {
	var req struct {
		StockPool      []string `json:"stock_pool" binding:"required"`
		StartDate      string   `json:"start_date" binding:"required"`
		EndDate        string   `json:"end_date" binding:"required"`
		InitialCapital float64  `json:"initial_capital" binding:"required"`
		EntryConfigs   []SignalConfig `json:"entry_configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}
	_, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid strategy id"})
		return
	}
	runRequest := RunRequest{
		StockPool:      req.StockPool,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		InitialCapital: req.InitialCapital,
		EntryConfigs:   req.EntryConfigs,
	}
	runID, err := h.engine.Initiate(c.Request.Context(), runRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"run_id": runID, "status": "pending"}})
}

// GetRun GET /api/v1/backtest/runs/:id
func (h *Handler) GetRun(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid run id"})
		return
	}
	run, err := h.dao.GetRun(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": run})
}

// GetRunStatus GET /api/v1/backtest/runs/:id/status
func (h *Handler) GetRunStatus(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid run id"})
		return
	}
	run, err := h.dao.GetRun(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"status": run.Status, "progress_pct": run.ProgressPct}})
}

// GetTrades GET /api/v1/backtest/runs/:id/trades
func (h *Handler) GetTrades(c *gin.Context) {
	runID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	trades, total, err := h.dao.GetTradesByRun(runID, (page-1)*pageSize, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"total": total, "items": trades}})
}

// GetSnapshots GET /api/v1/backtest/runs/:id/snapshots
func (h *Handler) GetSnapshots(c *gin.Context) {
	runID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	snapshots, err := h.dao.GetSnapshotsByRun(runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"snapshots": snapshots}})
}

// GetRuns GET /api/v1/strategies/:id/backtest/runs
func (h *Handler) GetRuns(c *gin.Context) {
	strategyID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	runs, err := h.dao.GetRunsByStrategy(strategyID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": runs})
}

// DeleteRun DELETE /api/v1/backtest/runs/:id
func (h *Handler) DeleteRun(c *gin.Context) {
	runID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.dao.DeleteRun(runID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"deleted": true}})
}
