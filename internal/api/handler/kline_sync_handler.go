package handler

import (
	"log"
	"net/http"
	"strings"

	"stock-ai/internal/db"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// KLineSyncHandler K线同步处理器
type KLineSyncHandler struct {
	service *service.SyncKLineService
}

// NewKLineSyncHandler 创建 K 线同步处理器
func NewKLineSyncHandler() *KLineSyncHandler {
	return &KLineSyncHandler{
		service: service.NewSyncKLineService(),
	}
}

// SyncKLineRequest K线同步请求
type SyncKLineRequest struct {
	Periods string `json:"periods"` // 逗号分隔: daily,weekly,monthly,yearly（默认全部）
}

// parsePeriods 解析 periods 参数，返回周期列表
func parsePeriods(periodsStr string) []db.KLinePeriod {
	if periodsStr == "" {
		return service.AllPeriods
	}
	var result []db.KLinePeriod
	for _, p := range strings.Split(periodsStr, ",") {
		p = strings.TrimSpace(p)
		switch p {
		case "daily":
			result = append(result, db.KLinePeriodDaily)
		case "weekly":
			result = append(result, db.KLinePeriodWeekly)
		case "monthly":
			result = append(result, db.KLinePeriodMonthly)
		case "yearly":
			result = append(result, db.KLinePeriodYearly)
		}
	}
	if len(result) == 0 {
		return service.AllPeriods
	}
	return result
}

// RunInit 初始化同步：同花顺全量拉取骨架数据
// POST /api/v1/sync-kline/init
func (h *KLineSyncHandler) RunInit(c *gin.Context) {
	var req SyncKLineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	periods := parsePeriods(req.Periods)

	go func() {
		results := h.service.InitAllStocks(c.Request.Context(), periods)
		logSyncResults("init", results)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "K线初始化已启动",
	})
}

// RunDaily 每日增量同步：同花顺 GetToday 获取当期数据
// POST /api/v1/sync-kline/daily
func (h *KLineSyncHandler) RunDaily(c *gin.Context) {
	var req SyncKLineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	periods := parsePeriods(req.Periods)

	go func() {
		results := h.service.SyncDailyForAll(c.Request.Context(), periods)
		logSyncResults("daily", results)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "每日增量同步已启动",
	})
}

// RunFill 补全金额：东财拉取补 amount=0 的记录
// POST /api/v1/sync-kline/fill
func (h *KLineSyncHandler) RunFill(c *gin.Context) {
	var req SyncKLineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	periods := parsePeriods(req.Periods)

	go func() {
		results := h.service.FillMissingAmount(c.Request.Context(), periods)
		logSyncResults("fill", results)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "金额补全已启动",
	})
}

// logSyncResults 打印批量同步结果摘要
func logSyncResults(mode string, results []service.SyncBatchResult) {
	for _, r := range results {
		log.Printf("[sync-%s] 完成: 成功=%d 跳过=%d 失败=%d 耗时=%.1fs",
			mode, r.Success, r.SkipNoDelta, r.Fail, r.CostSeconds)
	}
}
