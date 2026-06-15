package backtest

import (
	"net/http"
	"strconv"

	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler 回测 HTTP 处理器
type Handler struct {
	svc *Service
	dao *DAO
}

// NewHandler 创建回测 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, dao: NewDAO()}
}

// ============================================================================
//  P0 API
// ============================================================================

// Initiate POST /api/v1/strategies/:id/backtest
func (h *Handler) Initiate(c *gin.Context) {
	var req struct {
		StockPool             []string            `json:"stock_pool" binding:"required"`
		StartDate             string              `json:"start_date" binding:"required"`
		EndDate               string              `json:"end_date" binding:"required"`
		InitialCapital        float64             `json:"initial_capital" binding:"required"`
		ExitRulesOverride     *model.ExitRules    `json:"exit_rules_override"`
		PositionRulesOverride *model.PositionRules `json:"position_rules_override"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}
	strategyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid strategy id"})
		return
	}
	runID, err := h.svc.Initiate(c.Request.Context(), strategyID, req.StockPool, req.StartDate, req.EndDate, req.InitialCapital, req.ExitRulesOverride, req.PositionRulesOverride)
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
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"status":       run.Status,
			"progress_pct": run.ProgressPct,
		},
	})
}

// GetTrades GET /api/v1/backtest/runs/:id/trades
func (h *Handler) GetTrades(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid run id"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	trades, total, err := h.dao.GetTradesByRun(runID, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}

	// 填充 stock_name（批量查 DB，避免 N+1）
	codeSet := make(map[string]bool)
	for i := range trades {
		codeSet[trades[i].StockCode] = true
	}
	for code := range codeSet {
		if stock, err := db.FindStockByCode(code); err == nil {
			for i := range trades {
				if trades[i].StockCode == code {
					trades[i].StockName = stock.Name
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"total": total, "items": trades}})
}

// GetSnapshots GET /api/v1/backtest/runs/:id/snapshots
func (h *Handler) GetSnapshots(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid run id"})
		return
	}
	snapshots, err := h.dao.GetSnapshotsByRun(runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"snapshots": snapshots}})
}

// GetRuns GET /api/v1/strategies/:id/backtest/runs
func (h *Handler) GetRuns(c *gin.Context) {
	strategyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid strategy id"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	runs, err := h.dao.GetRunsByStrategy(strategyID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": runs})
}

// ============================================================================
//  P1 API
// ============================================================================

// DeleteRun DELETE /api/v1/backtest/runs/:id
func (h *Handler) DeleteRun(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "invalid run id"})
		return
	}
	if err := h.dao.DeleteRun(runID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"deleted": true}})
}
