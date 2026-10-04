package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/service"
	"stock-ai/utils"

	"github.com/gin-gonic/gin"
)

// PickDefaultRangeDays 日期为空时，默认复盘的日历天数
const PickDefaultRangeDays = 14

// StrategyPickHandler 策略选股复盘 Handler
type StrategyPickHandler struct {
	svc *service.StrategyPicksService
}

// NewStrategyPickHandler 创建选股复盘 Handler。
//
// 参数:
//   - screenSvc: 股票池构建服务
//   - registry: 指标注册表（提供选股引擎）
func NewStrategyPickHandler(screenSvc *service.ScreenService, registry *indicator.Registry) *StrategyPickHandler {
	return &StrategyPickHandler{
		svc: service.NewStrategyPicksService(screenSvc, registry),
	}
}

// StrategyPickDaysMeta 复盘区间的交易日列表（供前端按天轮询）。
type StrategyPickDaysMeta struct {
	StrategyID   uint     `json:"strategy_id"`
	StrategyName string   `json:"strategy_name"`
	HoldDays     int      `json:"hold_days"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
	Dates        []string `json:"dates"` // 需要回放的交易日，YYYY-MM-DD 升序
}

// PicksDays 返回复盘区间内需要回放的交易日列表（跳过周末与节假日）。
// GET /api/v1/strategies/:id/picks/days
//
// Query 参数:
//   - start_date: 复盘起始日期 YYYY-MM-DD，默认最近 14 个自然日
//   - end_date:   复盘结束日期 YYYY-MM-DD，默认最新交易日
//   - hold_days:  观察窗口（交易日），默认 5
//
// 前端拿到 dates 后逐个调用 PicksDay，自行汇总结果。
func (h *StrategyPickHandler) PicksDays(c *gin.Context) {
	strategy, ok := h.loadStrategy(c)
	if !ok {
		return
	}
	startDate, endDate, ok := h.resolveRange(c)
	if !ok {
		return
	}
	holdDays := queryInt(c.Query("hold_days"), service.DefaultPickHoldDays, service.MaxPickHoldDays)

	dates, err := h.svc.TradingDays(startDate, endDate)
	if err != nil {
		slog.Error("查询复盘交易日失败", "strategy_id", strategy.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询复盘交易日失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": StrategyPickDaysMeta{
		StrategyID:   strategy.ID,
		StrategyName: strategy.Name,
		HoldDays:     holdDays,
		StartDate:    startDate,
		EndDate:      endDate,
		Dates:        dates,
	}})
}

// PicksDay 只回放单个交易日的选股结果，前端按天轮询后汇总。
// GET /api/v1/strategies/:id/picks/day
//
// Query 参数:
//   - date:      交易日 YYYY-MM-DD（必填）
//   - hold_days: 观察窗口（交易日），默认 5
//
// 响应 data 为 service.PickDayResult。
func (h *StrategyPickHandler) PicksDay(c *gin.Context) {
	strategy, ok := h.loadStrategy(c)
	if !ok {
		return
	}
	date := c.Query("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 date 参数"})
		return
	}

	query := service.StrategyPicksQuery{
		Strategy:       strategy,
		HoldDays:       queryInt(c.Query("hold_days"), service.DefaultPickHoldDays, service.MaxPickHoldDays),
		MaxConcurrency: queryInt(c.Query("max_concurrency"), 10, 100),
	}

	day, err := h.svc.AnalyzeDay(c.Request.Context(), query, date)
	if err != nil {
		slog.Error("单日选股复盘失败", "strategy_id", strategy.ID, "date", date, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "单日选股复盘失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": day})
}

// loadStrategy 解析 :id 并加载当前用户可见的策略，失败时已写入响应。
func (h *StrategyPickHandler) loadStrategy(c *gin.Context) (*model.Strategy, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return nil, false
	}
	strategy, err := db.GetVisibleStrategyByID(c.Request.Context(), getUID(c), isAdmin(c), uint(id))
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在或无权访问"})
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询策略失败: " + err.Error()})
		return nil, false
	}
	return strategy, true
}

// resolveRange 解析复盘日期区间，失败时已写入响应。
func (h *StrategyPickHandler) resolveRange(c *gin.Context) (string, string, bool) {
	startDate, endDate, err := resolvePickDateRange(c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return "", "", false
	}
	return startDate, endDate, true
}

// resolvePickDateRange 解析复盘日期区间（返回 YYYY-MM-DD 字符串）。
// 起始日期为空时取结束日期往前 PickDefaultRangeDays 个自然日；结束日期为空时取最新交易日。
func resolvePickDateRange(startRaw, endRaw string) (string, string, error) {
	endDate := endRaw
	if endDate == "" {
		latest, err := db.GetLatestDailyKlineDate()
		if err != nil || latest == 0 {
			return "", "", errors.New("无法确定复盘结束日期，请显式指定 end_date")
		}
		endDate = db.FormatTradeDate(latest)
	}
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return "", "", errors.New("end_date 格式应为 YYYY-MM-DD")
	}

	startDate := startRaw
	if startDate == "" {
		startDate = endTime.AddDate(0, 0, -PickDefaultRangeDays).Format("2006-01-02")
	}
	if _, err := utils.ParseDateToTradeDate(startDate); err != nil {
		return "", "", errors.New("start_date 格式应为 YYYY-MM-DD")
	}
	return startDate, endDate, nil
}

// queryInt 解析查询参数，非法或越界时返回 fallback，并限制在 [1, max] 区间。
func queryInt(raw string, fallback, max int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	if v > max {
		return max
	}
	return v
}
