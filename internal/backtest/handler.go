package backtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler 回测 HTTP 处理器。
type Handler struct {
	factory *EngineFactory
	dao     *DAO
}

// NewHandler 创建回测 Handler。
// factory 每次 Initiate 调用时创建独立的 Engine，保证并发安全。
func NewHandler(factory *EngineFactory) *Handler {
	return &Handler{factory: factory, dao: NewDAO()}
}

// ============================================================================
//  P0 API
// ============================================================================

// Initiate POST /api/v1/strategies/:id/backtest
func (h *Handler) Initiate(c *gin.Context) {
	var req struct {
		StockPool             []string       `json:"stock_pool"`
		StartDate             string         `json:"start_date" binding:"required"`
		EndDate               string         `json:"end_date" binding:"required"`
		InitialCapital        float64        `json:"initial_capital" binding:"required"`
		EntryConfigs          []SignalConfig `json:"entry_configs"`
		ExitRulesOverride     *struct {
			Rules       []ExitRule `json:"rules"`
			SlippagePct float64    `json:"slippage_pct"`
		} `json:"exit_rules_override"`
		PositionRulesOverride *PositionRules `json:"position_rules_override"`
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

	// 空 stock_pool 默认使用 start_date 当天有日K数据的所有股票
	if len(req.StockPool) == 0 {
		req.StockPool, err = loadDefaultStockPool(req.StartDate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
			return
		}
	}

	// 空 entry_configs 默认从策略的 conditions 字段解析
	if len(req.EntryConfigs) == 0 {
		req.EntryConfigs, err = loadStrategyEntryConfigs(strategyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
			return
		}
	}

	runRequest := RunRequest{
		StrategyID:     strategyID,
		StockPool:      req.StockPool,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		InitialCapital: req.InitialCapital,
		EntryConfigs:   req.EntryConfigs,
	}
	// 前端可能传 exit_rules_override / position_rules_override
	if req.ExitRulesOverride != nil {
		runRequest.ExitConfigs = req.ExitRulesOverride.Rules
	}
	if req.PositionRulesOverride != nil {
		runRequest.PositionRules = *req.PositionRulesOverride
	}
	if err := validateRunRequest(runRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}
	engine, err := h.factory.NewEngine()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
	runID, err := engine.Initiate(c.Request.Context(), runRequest)
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

// ============================================================================
//  入参校验
// ============================================================================

// conditionItem 策略 conditions JSON 中的单条记录结构。
type conditionItem struct {
	SignalID string         `json:"signal_id"`
	Operator string         `json:"operator"`
	Params   map[string]any `json:"params"`
}

// loadStrategyEntryConfigs 从 DB 加载策略的 conditions JSON，解析为 SignalConfig 列表。
func loadStrategyEntryConfigs(strategyID uint64) ([]SignalConfig, error) {
	var st model.Strategy
	if err := db.GetDB().First(&st, strategyID).Error; err != nil {
		return nil, fmt.Errorf("strategy %d not found: %w", strategyID, err)
	}
	if st.Conditions == "" {
		return nil, fmt.Errorf("strategy %d has no conditions", strategyID)
	}
	var items []conditionItem
	if err := json.Unmarshal([]byte(st.Conditions), &items); err != nil {
		return nil, fmt.Errorf("parse strategy conditions: %w", err)
	}
	cfgs := make([]SignalConfig, 0, len(items))
	for _, item := range items {
		if item.SignalID == "" {
			continue
		}
		cfgs = append(cfgs, SignalConfig{
			SignalID: item.SignalID,
			Operator: item.Operator,
			Params:   item.Params,
		})
	}
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("strategy %d has no valid signals", strategyID)
	}
	return cfgs, nil
}

// loadDefaultStockPool 加载指定日期有日K数据的所有股票代码，作为默认股票池。
func loadDefaultStockPool(dateStr string) ([]string, error) {
	date, err := utilsParseDate(dateStr)
	if err != nil {
		return nil, fmt.Errorf("parse start_date for default stock pool: %w", err)
	}
	tradeDate, err := db.FindNearestDailyKlineDate(date)
	if err != nil {
		return nil, fmt.Errorf("no trading data found near %s: %w", dateStr, err)
	}
	stocks, err := db.LoadStockCodesByTradeDate(tradeDate)
	if err != nil {
		return nil, fmt.Errorf("load stock codes: %w", err)
	}
	codes := make([]string, len(stocks))
	for i, s := range stocks {
		codes[i] = s.Code
	}
	return codes, nil
}

// utilsParseDate 将 YYYYMMDD 整数转为 "2006-01-02" 格式的 time.Time，辅助函数。
func utilsParseDate(dateStr string) (int, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, err
	}
	return t.Year()*10000 + int(t.Month())*100 + t.Day(), nil
}

// validateRunRequest 校验回测请求参数。
func validateRunRequest(req RunRequest) error {
	if len(req.StockPool) == 0 {
		return fmt.Errorf("stock_pool is empty")
	}
	if req.InitialCapital <= 0 {
		return fmt.Errorf("initial_capital must be positive, got %.2f", req.InitialCapital)
	}

	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start_date %q: %w", req.StartDate, err)
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return fmt.Errorf("invalid end_date %q: %w", req.EndDate, err)
	}
	if end.Before(start) {
		return fmt.Errorf("end_date %s is before start_date %s", req.EndDate, req.StartDate)
	}
	return nil
}
