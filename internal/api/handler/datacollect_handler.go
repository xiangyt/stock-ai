package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// ============================================================================
//  DataCollectHandler — 统一的数据采集 Handler
//  包含：采集执行、快照计算、K线同步、定时任务管理（前端配置页）
// ============================================================================

// DataCollectHandler 数据采集 HTTP Handler
type DataCollectHandler struct {
	taskSvc *service.DataCollectTaskService
	runner  *datacollect.DataCollectRunner
}

// NewDataCollectHandler 创建数据采集 Handler
func NewDataCollectHandler(taskSvc *service.DataCollectTaskService, runner *datacollect.DataCollectRunner) *DataCollectHandler {
	return &DataCollectHandler{
		taskSvc: taskSvc,
		runner:  runner,
	}
}

// ============================================================================
//  通用辅助方法
// ============================================================================

// adminRequired 校验当前用户是否为管理员
func (h *DataCollectHandler) adminRequired(c *gin.Context) (uint, bool) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return 0, false
	}
	user, err := db.GetUserByID(userID)
	if err != nil || user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "需要管理员权限"})
		return 0, false
	}
	return userID, true
}

// ============================================================================
//  定时任务管理接口（前端配置页，需要管理员权限）
// ============================================================================

// List 获取所有数据采集任务
// GET /api/v1/datacollect
func (h *DataCollectHandler) List(c *gin.Context) {
	if _, ok := h.adminRequired(c); !ok {
		return
	}

	items, err := h.taskSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": items})
}

// GetByID 获取任务详情
// GET /api/v1/datacollect/:id
func (h *DataCollectHandler) GetByID(c *gin.Context) {
	if _, ok := h.adminRequired(c); !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
		return
	}

	detail, err := h.taskSvc.GetByID(uint(id))
	if err != nil {
		errMsg := err.Error()
		code := 500
		if errMsg == "任务不存在" {
			code = 404
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
}

// Update 更新任务配置
// PUT /api/v1/datacollect/:id
func (h *DataCollectHandler) Update(c *gin.Context) {
	if _, ok := h.adminRequired(c); !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
		return
	}

	var req model.DCUpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.taskSvc.UpdateTask(uint(id), &req)
	if err != nil {
		errMsg := err.Error()
		code := 500
		switch errMsg {
		case "任务不存在":
			code = 404
		case "cron 表达式不能为空", "参数格式无效，必须是合法 JSON":
			code = 400
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
}

// UpdateBots 更新任务关联的机器人
// PUT /api/v1/datacollect/:id/bots
func (h *DataCollectHandler) UpdateBots(c *gin.Context) {
	if _, ok := h.adminRequired(c); !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
		return
	}

	var req model.DCUpdateBotsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if err := h.taskSvc.UpdateBots(uint(id), req.BotIDs); err != nil {
		errMsg := err.Error()
		code := 500
		switch {
		case errMsg == "任务不存在" ||
			(strings.Contains(errMsg, "机器人") && strings.Contains(errMsg, "不存在")):
			code = 404
		case strings.Contains(errMsg, "机器人") && strings.Contains(errMsg, "已禁用"):
			code = 400
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// Execute 立即执行一次数据采集任务
// POST /api/v1/datacollect/:id/execute
func (h *DataCollectHandler) Execute(c *gin.Context) {
	if _, ok := h.adminRequired(c); !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的任务 ID"})
		return
	}

	// 加载任务
	task, err := db.GetDataCollectTaskByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "任务不存在"})
		return
	}

	// 加载关联机器人
	bots, _ := db.GetDataCollectBots(uint(id))

	// 异步执行，立即返回
	go func() {
		ctx := context.Background()
		if err := h.runner.Run(ctx, task, bots); err != nil {
			log.Printf("[DataCollect] 手动触发执行失败 task=%d(%s): %v", task.ID, task.Name, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": fmt.Sprintf("任务 [%s] 已触发执行", task.Name),
	})
}

// ============================================================================
//  采集执行接口 — 股票列表 & 详情
// ============================================================================

// StockListCollectRequest 股票列表采集请求
type StockListCollectRequest struct {
	Source string `json:"source" binding:"required"` // 数据源名称: eastmoney / ths
}

// RunStockList 运行股票列表采集
// POST /api/v1/collector/stock-list
func (h *DataCollectHandler) RunStockList(c *gin.Context) {
	var req StockListCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx := context.Background()
		adp, err := datacollect.ResolveAdapter(adapter.GetRegistry(), req.Source)
		if err != nil {
			log.Printf("[collector] 获取数据源失败: %v", err)
			return
		}

		allStocks, err := adp.GetStockList(ctx)
		if err != nil {
			log.Printf("[collector] 获取股票列表失败: %v", err)
			return
		}

		total, newCount, updCount := len(allStocks), 0, 0
		for i, stock := range allStocks {
			detail, detailErr := adp.GetStockDetail(ctx, stock.Code)
			if detailErr != nil {
				log.Printf("[collector] 获取详情失败 [%s]: %v", stock.Code, detailErr)
				continue
			}
			if db.UpsertStock(datacollect.ToStockModel(stock.Code, detail)) == 0 {
				newCount++
			} else {
				updCount++
			}
			if (i+1)%100 == 0 || i == total-1 {
				log.Printf("[collector] 详情进度: %d/%d (新增=%d, 更新=%d)", i+1, total, newCount, updCount)
			}
		}
		log.Printf("[collector] 采集完成: total=%d, new=%d, upd=%d", total, newCount, updCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("股票列表采集已启动, 数据源=%s", req.Source),
	})
}

// StockDetailCollectRequest 单只股票详情采集请求
type StockDetailCollectRequest struct {
	Source string `json:"source"` // 数据源名称(可选, 默认 eastmoney)
}

// RunPriceData 运行单只股票详情采集
// POST /api/v1/collector/stock-detail/:code
func (h *DataCollectHandler) RunPriceData(c *gin.Context) {
	code := c.Param("code")

	var req StockDetailCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx := context.Background()
		adp, err := datacollect.ResolveAdapter(adapter.GetRegistry(), req.Source)
		if err != nil {
			log.Printf("[collector] 获取数据源失败: %v", err)
			return
		}
		detail, err := adp.GetStockDetail(ctx, code)
		if err != nil {
			log.Printf("[collector] 详情采集失败 [%s]: %v", code, err)
			return
		}
		s := datacollect.ToStockModel(code, detail)
		db.UpsertStock(s)
		log.Printf("[collector] 详情采集成功 [%s]: %s", code, s.Name)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("股票详情采集已启动: %s (源=%s)", code, req.Source),
	})
}

// ============================================================================
//  采集执行接口 — K线
// ============================================================================

// KLineCollectRequest K线采集请求
type KLineCollectRequest struct {
	Source    string `json:"source"`             // 数据源名称(可选, 默认 eastmoney)
	KLineType string `json:"kline_type"`         // 周期: daily / weekly / monthly / yearly
	AdjType   string `json:"adj_type,omitempty"` // 复权类型(可选, 默认前复权 qfq)
}

// RunKLineData 运行单只股票K线采集
// POST /api/v1/collector/kline/:code
func (h *DataCollectHandler) RunKLineData(c *gin.Context) {
	code := c.Param("code")

	var req KLineCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.KLineType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kline_type 必填 (daily/weekly/monthly/yearly)"})
		return
	}

	period := klineTypeToPeriod(req.KLineType)

	go func() {
		err := datacollect.GetSyncKLineService().DebugSyncSingle(
			context.Background(),
			[]db.KLinePeriod{period},
			code,
			"daily",
		)
		if err != nil {
			log.Printf("[collector] K线采集失败 [%s/%s]: %v", code, req.KLineType, err)
			return
		}
		log.Printf("[collector] K线采集完成 [%s/%s]", code, req.KLineType)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("K线采集已启动: %s, 周期=%s, 源=%s", code, req.KLineType, req.Source),
	})
}

// RunKLineBatch 运行全量股票K线采集
// POST /api/v1/collector/kline-batch
func (h *DataCollectHandler) RunKLineBatch(c *gin.Context) {
	var req KLineCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.KLineType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kline_type 必填 (daily/weekly/monthly/yearly)"})
		return
	}

	period := klineTypeToPeriod(req.KLineType)

	go func() {
		results := datacollect.GetSyncKLineService().SyncDailyForAll(
			context.Background(),
			[]db.KLinePeriod{period},
		)
		logSyncResults("kline_batch", results)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("全量%sK线采集已启动, 数据源=%s", req.KLineType, req.Source),
	})
}

// klineTypeToPeriod 将字符串周期映射为 db.KLinePeriod
func klineTypeToPeriod(kt string) db.KLinePeriod {
	switch kt {
	case "daily":
		return db.KLinePeriodDaily
	case "weekly":
		return db.KLinePeriodWeekly
	case "monthly":
		return db.KLinePeriodMonthly
	case "yearly":
		return db.KLinePeriodYearly
	default:
		return db.KLinePeriodDaily
	}
}

// ============================================================================
//  采集执行接口 — 基本面/财务面
// ============================================================================

// FundamentalCollectRequest 基本面采集请求
type FundamentalCollectRequest struct {
	Source string `json:"source"` // 数据源名称(可选, 默认 eastmoney)
}

// RunPerformanceReports 运行单只股票财报采集
// POST /api/v1/collector/fundamental/:code/performance
func (h *DataCollectHandler) RunPerformanceReports(c *gin.Context) {
	code := c.Param("code")
	var req FundamentalCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx := context.Background()
		adp, err := datacollect.ResolveAdapter(adapter.GetRegistry(), req.Source)
		if err != nil {
			log.Printf("[collector] 获取数据源失败: %v", err)
			return
		}
		result, err := datacollect.RunPerformanceReports(ctx, adp, code)
		if err != nil {
			log.Printf("[collector] 财报采集失败 [%s]: %v", code, err)
			return
		}
		log.Printf("[collector] 财报采集完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("财报采集已启动: %s (源=%s)", code, req.Source),
	})
}

// RunPerformanceReportsBatch 运行全量股票财报采集
// POST /api/v1/collector/fundamental-batch/performance
func (h *DataCollectHandler) RunPerformanceReportsBatch(c *gin.Context) {
	var req FundamentalCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx := context.Background()
		adp, err := datacollect.ResolveAdapter(adapter.GetRegistry(), req.Source)
		if err != nil {
			log.Printf("[collector] 获取数据源失败: %v", err)
			return
		}
		result, err := datacollect.RunPerformanceReportsBatch(ctx, adp)
		if err != nil {
			log.Printf("[collector] 全量财报采集失败: %v", err)
			return
		}
		log.Printf("[collector] 全量财报采集完成: total=%d, new=%d, upd=%d, fail=%d",
			result.Total, result.NewCount, result.UpdCount, result.FailCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "全量财报采集已启动",
	})
}

// RunShareholderCounts 运行单只股票股东户数采集
// POST /api/v1/collector/fundamental/:code/shareholder
func (h *DataCollectHandler) RunShareholderCounts(c *gin.Context) {
	code := c.Param("code")
	var req FundamentalCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx := context.Background()
		adp, err := datacollect.ResolveAdapter(adapter.GetRegistry(), req.Source)
		if err != nil {
			log.Printf("[collector] 获取数据源失败: %v", err)
			return
		}
		result, err := datacollect.RunShareholderCounts(ctx, adp, code)
		if err != nil {
			log.Printf("[collector] 股东户数采集失败 [%s]: %v", code, err)
			return
		}
		log.Printf("[collector] 股东户数采集完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("股东户数采集已启动: %s (源=%s)", code, req.Source),
	})
}

// RunShareholderCountsBatch 运行全量股东户数采集
// POST /api/v1/collector/fundamental-batch/shareholder
func (h *DataCollectHandler) RunShareholderCountsBatch(c *gin.Context) {
	var req FundamentalCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx := context.Background()
		adp, err := datacollect.ResolveAdapter(adapter.GetRegistry(), req.Source)
		if err != nil {
			log.Printf("[collector] 获取数据源失败: %v", err)
			return
		}
		result, err := datacollect.RunShareholderCountsBatch(ctx, adp)
		if err != nil {
			log.Printf("[collector] 全量股东户数采集失败: %v", err)
			return
		}
		log.Printf("[collector] 全量股东户数采集完成: total=%d, new=%d, upd=%d, fail=%d",
			result.Total, result.NewCount, result.UpdCount, result.FailCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "全量股东户数采集已启动",
	})
}

// RunShareChanges 运行单只股票股本变动采集
// POST /api/v1/collector/fundamental/:code/share-change
func (h *DataCollectHandler) RunShareChanges(c *gin.Context) {
	code := c.Param("code")
	var req FundamentalCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx := context.Background()
		adp, err := datacollect.ResolveAdapter(adapter.GetRegistry(), req.Source)
		if err != nil {
			log.Printf("[collector] 获取数据源失败: %v", err)
			return
		}
		result, err := datacollect.RunShareChanges(ctx, adp, code)
		if err != nil {
			log.Printf("[collector] 股本变动采集失败 [%s]: %v", code, err)
			return
		}
		log.Printf("[collector] 股本变动采集完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("股本变动采集已启动: %s (源=%s)", code, req.Source),
	})
}

// RunShareChangesBatch 运行全量股本变动采集
// POST /api/v1/collector/fundamental-batch/share-change
func (h *DataCollectHandler) RunShareChangesBatch(c *gin.Context) {
	var req FundamentalCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx := context.Background()
		adp, err := datacollect.ResolveAdapter(adapter.GetRegistry(), req.Source)
		if err != nil {
			log.Printf("[collector] 获取数据源失败: %v", err)
			return
		}
		result, err := datacollect.RunShareChangesBatch(ctx, adp)
		if err != nil {
			log.Printf("[collector] 全量股本变动采集失败: %v", err)
			return
		}
		log.Printf("[collector] 全量股本变动采集完成: total=%d, new=%d, upd=%d, fail=%d",
			result.Total, result.NewCount, result.UpdCount, result.FailCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "全量股本变动采集已启动",
	})
}

// ============================================================================
//  快照计算接口
// ============================================================================

// SnapshotRequest 快照计算请求
type SnapshotRequest struct {
	Code string `json:"code"` // 股票代码（空=所有股票）
}

// Calc 快照计算入口
// POST /api/v1/collector/snapshot/calc
func (h *DataCollectHandler) Calc(c *gin.Context) {
	var req SnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		result := datacollect.GetSnapshotService().Calc(context.Background(), strings.TrimSpace(req.Code))
		log.Printf("[snapshot] 全部完成! 股票:成功=%d 失败=%d | 快照:写入成功=%d 写入失败=%d | 耗时=%.1fs",
			result.SuccessStocks, result.FailStocks,
			result.SuccessSnapshots, result.FailSnapshots,
			result.CostSeconds)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "快照计算已启动",
	})
}

// ============================================================================
//  K线同步接口
// ============================================================================

// SyncKLineRequest K线同步请求
type SyncKLineRequest struct {
	Periods string `json:"periods"` // 逗号分隔: daily,weekly,monthly,yearly（默认全部）
	Mode    string `json:"mode"`    // 执行类型
	Code    string `json:"code"`    // 股票代码
}

// parsePeriods 解析 periods 参数，返回周期列表
func parsePeriods(periodsStr string) []db.KLinePeriod {
	if periodsStr == "" {
		return datacollect.AllPeriods
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
		return datacollect.AllPeriods
	}
	return result
}

// RunInit 初始化同步：同花顺全量拉取骨架数据
// POST /api/v1/sync-kline/init
func (h *DataCollectHandler) RunInit(c *gin.Context) {
	var req SyncKLineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	periods := parsePeriods(req.Periods)

	go func() {
		results := datacollect.GetSyncKLineService().InitAllStocks(context.Background(), periods)
		logSyncResults("init", results)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "K线初始化已启动",
	})
}

// RunDaily 每日增量同步：同花顺 GetToday 获取当期数据
// POST /api/v1/sync-kline/daily
func (h *DataCollectHandler) RunDaily(c *gin.Context) {
	var req SyncKLineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	periods := parsePeriods(req.Periods)

	go func() {
		results := datacollect.GetSyncKLineService().SyncDailyForAll(context.Background(), periods)
		logSyncResults("daily", results)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "每日增量同步已启动",
	})
}

// RunFill 补全金额：东财拉取补 amount=0 的记录
// POST /api/v1/sync-kline/fill
func (h *DataCollectHandler) RunFill(c *gin.Context) {
	var req SyncKLineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	periods := parsePeriods(req.Periods)

	go func() {
		results := datacollect.GetSyncKLineService().FillMissingAmount(context.Background(), periods)
		logSyncResults("fill", results)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "金额补全已启动",
	})
}

// Debug 调试接口
// POST /api/v1/sync-kline/debug
func (h *DataCollectHandler) Debug(c *gin.Context) {
	var req SyncKLineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	periods := parsePeriods(req.Periods)

	err := datacollect.GetSyncKLineService().DebugSyncSingle(c.Request.Context(), periods, req.Code, req.Mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "调试同步已启动",
	})
}

// ============================================================================
//  辅助函数
// ============================================================================

// logSyncResults 打印批量同步结果摘要
func logSyncResults(mode string, results []datacollect.SyncBatchResult) {
	for _, r := range results {
		log.Printf("[sync-%s] 完成: 成功=%d 跳过=%d 失败=%d 耗时=%.1fs",
			mode, r.Success, r.SkipNoDelta, r.Fail, r.CostSeconds)
	}
}

// ============================================================================
//  全局健康检查（包级函数，供 router.go 直接引用）
// ============================================================================

// HealthCheck 健康检查
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"version":   "1.0.0",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
