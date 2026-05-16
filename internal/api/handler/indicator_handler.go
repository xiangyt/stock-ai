package handler

import (
	"net/http"

	"stock-ai/internal/indicator"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// IndicatorHandler 指标 HTTP Handler
type IndicatorHandler struct {
	registry  *indicator.Registry
	screenSvc *service.ScreenService // 用于构建选股/回测引擎输入数据
}

// NewIndicatorHandler 创建指标 Handler（初始化全部内置指标）
func NewIndicatorHandler(screenSvc *service.ScreenService) *IndicatorHandler {
	reg := indicator.NewRegistry(allBuiltins())
	return &IndicatorHandler{registry: reg, screenSvc: screenSvc}
}

// Registry 返回底层指标注册表（供命令行工具等非 HTTP 场景使用）
func (h *IndicatorHandler) Registry() *indicator.Registry {
	return h.registry
}

// ListIndicators 获取全部指标元数据
// GET /api/v1/indicators
func (h *IndicatorHandler) ListIndicators(c *gin.Context) {
	meta := h.registry.ToAPIMeta()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"categories":   meta.Categories,
			"indicators":   meta.Indicators,
			"enum_options": meta.EnumOptions,
		},
	})
}

// GetIndicatorByID 获取单个指标详情
// GET /api/v1/indicators/:id
func (h *IndicatorHandler) GetIndicatorByID(c *gin.Context) {
	id := c.Param("id")
	ind, ok := h.registry.GetIndicatorByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "指标不存在: " + id})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": indicator.ToAPIMeta(ind)})
}

// ExecuteRequest 选股执行请求体
type ExecuteRequest struct {
	Configs        []*indicator.SignalConfig `json:"configs"`         // 信号配置列表
	MaxConcurrency int                       `json:"max_concurrency"` // 最大并发数 (默认10)
	Date           string                    `json:"date"`            // 选股日期
}

// ExecuteResponse 选股执行响应
type ExecuteResponse struct {
	Total    int                        `json:"total"`    // 输入股票总数
	Passed   []indicator.EvaluatedStock `json:"passed"`   // 通过列表
	Rejected []indicator.EvaluatedStock `json:"rejected"` // 未通过列表
}

// Execute 执行选股筛选
// POST /api/v1/indicators/execute
//
// 处理流程:
//  1. 从请求体解析 SignalConfig 列表
//  2. 调用 ScreenService.BuildAll() 获取全量股票数据
//  3. 调用 Engine.Execute() 并发评估
//  4. 按 Passed/Rejected 分组返回
func (h *IndicatorHandler) Execute(c *gin.Context) {
	var req ExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if len(req.Configs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "信号配置不能为空"})
		return
	}

	maxConcurrency := req.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	// 获取股票数据
	stocks, err := h.screenSvc.BuildAll(maxConcurrency, req.Date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取股票数据失败: " + err.Error()})
		return
	}

	if len(stocks) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": ExecuteResponse{
			Total:    0,
			Passed:   []indicator.EvaluatedStock{},
			Rejected: []indicator.EvaluatedStock{},
		}})
		return
	}

	// 执行选股
	results := h.registry.Engine().Execute(stocks, req.Configs, maxConcurrency)

	// 分组结果
	var passed, rejected []indicator.EvaluatedStock
	for _, r := range results {
		switch r.Result {
		case indicator.ResultPassed:
			passed = append(passed, *r)
		default:
			rejected = append(rejected, *r)
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": ExecuteResponse{
		Total:    len(stocks),
		Passed:   passed,
		Rejected: rejected,
	}})
}
