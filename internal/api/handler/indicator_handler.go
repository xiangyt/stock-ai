package handler

import (
	"net/http"

	"stock-ai/internal/screener/indicator"
	"stock-ai/internal/model"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// IndicatorHandler 指标 HTTP Handler
type IndicatorHandler struct {
	registry  *indicator.Registry
	stockSvc *service.StockService // 用于获取股票数据
}

// NewIndicatorHandler 创建指标 Handler（初始化全部内置指标）
func NewIndicatorHandler(stockSvc *service.StockService) *IndicatorHandler {
	reg := indicator.NewRegistry(allBuiltins())
	return &IndicatorHandler{registry: reg, stockSvc: stockSvc}
}

// ListIndicators 获取全部指标元数据
// GET /api/v1/indicators
func (h *IndicatorHandler) ListIndicators(c *gin.Context) {
	meta := h.registry.ToAPIMeta()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"categories":  meta.Categories,
			"indicators":  meta.Indicators,
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
	Configs []indicator.SignalConfig `json:"configs"`       // 信号配置列表
	MaxConcurrency int               `json:"max_concurrency"` // 最大并发数 (默认10)
}

// ExecuteResponse 选股执行响应
type ExecuteResponse struct {
	Total   int                      `json:"total"`            // 输入股票总数
	Passed  []indicator.EvaluatedStock `json:"passed"`         // 通过列表
	Rejected []indicator.EvaluatedStock `json:"rejected"`      // 未通过列表
}

// Execute 执行选股筛选
// POST /api/v1/indicators/execute
//
// 处理流程:
//  1. 从请求体解析 SignalConfig 列表
//  2. 调用 StockService 获取全量股票数据，构建 StockData 列表
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
	stocks, err := h.buildStockData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取股票数据失败: " + err.Error()})
		return
	}

	if len(stocks) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": ExecuteResponse{
			Total:    0,
			Passed:  []indicator.EvaluatedStock{},
			Rejected: []indicator.EvaluatedStock{},
		}})
		return
	}

	// 执行选股
	// 将 []SignalConfig 转为 []*SignalConfig (Engine 接口要求指针切片)
		configPtrs := make([]*indicator.SignalConfig, len(req.Configs))
	for i := range req.Configs {
		configPtrs[i] = &req.Configs[i]
	}
	results := h.registry.Engine().Execute(stocks, configPtrs, maxConcurrency)

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

// buildStockData 从 StockService 获取全量 A 股数据并构建 StockData 列表
func (h *IndicatorHandler) buildStockData() ([]*indicator.StockData, error) {
	// 获取全量股票基本信息
	stockList, err := h.stockSvc.GetAllStocks()
	if err != nil {
		return nil, err
	}

	// TODO: 后续可优化为按需加载 K线/财务数据
	// 当前先构建基础 StockData（仅含 Detail）
	result := make([]*indicator.StockData, 0, len(stockList))
	for _, s := range stockList {
		result = append(result, &indicator.StockData{
			Detail: s.(*model.Stock),
		})
	}
	return result, nil
}
