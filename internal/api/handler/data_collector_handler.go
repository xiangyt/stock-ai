package handler

import (
	"net/http"
	"strconv"
	"time"

	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// DataCollectorHandler 数据采集处理器
type DataCollectorHandler struct {
	service *service.DataCollectService
}

// NewDataCollectorHandler 创建数据采集处理器
func NewDataCollectorHandler() *DataCollectorHandler {
	return &DataCollectorHandler{
		service: service.NewDataCollectService(),
	}
}

// CreateTaskRequest 创建采集任务请求
type CreateTaskRequest struct {
	Type       string   `json:"type" binding:"required"`        // stock_list/price/hot_topic/all
	Name       string   `json:"name"`                           // 任务名称
	Codes      []string `json:"codes"`                          // 指定股票代码(可选)
	Schedule   string   `json:"schedule"`                      // 定时表达式(cron)(可选)
	Enabled    bool     `json:"enabled"`                       // 是否启用
}

// TaskResponse 任务响应
type TaskResponse struct {
	Success bool          `json:"success"`
	Task   interface{}    `json:"task"`
	Message string        `json:"message"`
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	Success bool            `json:"success"`
	Tasks   []interface{}   `json:"tasks"`
	Total   int             `json:"total"`
}

// CollectorStatusResponse 采集状态响应
type CollectorStatusResponse struct {
	Success      bool   `json:"success"`
	Running      bool   `json:"running"`
	LastRunTime  string `json:"last_run_time"`
	NextRunTime  string `json:"next_run_time"`
	TotalTasks   int    `json:"total_tasks"`
	TotalStocks  int    `json:"total_stocks"`
	TotalRecords int64  `json:"total_records"`
}

// CollectorLogsResponse 采集日志响应
type CollectorLogsResponse struct {
	Success bool           `json:"success"`
	Logs    []interface{}  `json:"logs"`
	Total   int            `json:"total"`
}

// DataSourceResponse 数据源配置响应
type DataSourceResponse struct {
	Success   bool                   `json:"success"`
	Sources  []interface{}          `json:"sources"`
}

// CreateTask 创建采集任务
func (h *DataCollectorHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.service.CreateTask(service.CreateTaskParams{
		Type:     req.Type,
		Name:     req.Name,
		Codes:    req.Codes,
		Schedule: req.Schedule,
		Enabled:  req.Enabled,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, TaskResponse{
		Success: true,
		Task:    task,
		Message: "采集任务创建成功",
	})
}

// ListTasks 列出所有采集任务
func (h *DataCollectorHandler) ListTasks(c *gin.Context) {
	page := 1
	pageSize := 20
	
	if p, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil && ps > 0 && ps <= 100 {
		pageSize = ps
	}

	tasks, total, err := h.service.ListTasks(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, TaskListResponse{
		Success: true,
		Tasks:   tasks,
		Total:   total,
	})
}

// GetTask 获取采集任务详情
func (h *DataCollectorHandler) GetTask(c *gin.Context) {
	id := c.Param("id")
	
	task, err := h.service.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, TaskResponse{
		Success: true,
		Task:    task,
	})
}

// DeleteTask 删除采集任务
func (h *DataCollectorHandler) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	
	if err := h.service.DeleteTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "任务删除成功",
	})
}

// RunStockList 运行股票列表采集
func (h *DataCollectorHandler) RunStockList(c *gin.Context) {
	go func() {
		h.service.RunStockListCollection()
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "股票列表采集任务已启动",
	})
}

// RunPriceData 运行价格数据采集
func (h *DataCollectorHandler) RunPriceData(c *gin.Context) {
	code := c.Param("code")
	daysStr := c.DefaultQuery("days", "30")
	days, _ := strconv.Atoi(daysStr)
	
	go func() {
		h.service.RunPriceCollection(code, days)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "价格数据采集已启动",
	})
}

// RunAllPrices 运行全量价格采集
func (h *DataCollectorHandler) RunAllPrices(c *gin.Context) {
	go func() {
		h.service.RunAllPricesCollection()
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "全量价格采集已启动",
	})
}

// CollectorStatus 获取采集器状态
func (h *DataCollectorHandler) CollectorStatus(c *gin.Context) {
	status, err := h.service.GetCollectorStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// CollectorLogs 获取采集日志
func (h *DataCollectorHandler) CollectorLogs(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil && l <= 500 {
		limit = l
	}

	logs, err := h.service.GetLogs(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, CollectorLogsResponse{
		Success: true,
		Logs:    logs,
		Total:   len(logs),
	})
}

// GetDataSources 获取数据源配置
func (h *DataCollectorHandler) GetDataSources(c *gin.Context) {
	sources, err := h.service.GetDataSources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, DataSourceResponse{
		Success: true,
		Sources: sources,
	})
}

// UpdateDataSource 更新数据源配置
func (h *DataCollectorHandler) UpdateDataSource(c *gin.Context) {
	name := c.Param("name")
	var req map[string]interface{}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.UpdateDataSource(name, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "数据源更新成功",
	})
}

// HealthCheck 健康检查 (移到此处作为公共方法)
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"version":   "1.0.0",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
