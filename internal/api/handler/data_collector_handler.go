package handler

import (
	"fmt"
	"log"
	"net/http"
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

// StockListCollectRequest 股票列表采集请求
type StockListCollectRequest struct {
	Source string `json:"source" binding:"required"` // 数据源名称: eastmoney / ths
}

// RunStockList 运行股票列表采集
// 流程: 指定数据源 → 获取全量股票列表 → 遍历获取详情 → 数据库不存在则新增
func (h *DataCollectorHandler) RunStockList(c *gin.Context) {
	var req StockListCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		result, err := h.service.CollectStockList(req.Source)
		if err != nil {
			log.Printf("[collector] 采集失败: %v", err)
			return
		}
		log.Printf("[collector] 采集完成: total=%d, new=%d, upd=%d", result.Total, result.NewCount, result.UpdCount)
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
func (h *DataCollectorHandler) RunPriceData(c *gin.Context) {
	code := c.Param("code")

	var req StockDetailCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		_, err := h.service.CollectStockDetail(req.Source, code)
		if err != nil {
			log.Printf("[collector] 详情采集失败 [%s]: %v", code, err)
			return
		}
		log.Printf("[collector] 详情采集成功 [%s]", code)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("股票详情采集已启动: %s (源=%s)", code, req.Source),
	})
}

// ========== K线采集接口 ==========

// KLineCollectRequest K线采集请求
type KLineCollectRequest struct {
	Source    string `json:"source"`             // 数据源名称(可选, 默认 eastmoney)
	KLineType string `json:"kline_type"`         // 周期: daily / weekly / monthly / yearly
	AdjType   string `json:"adj_type,omitempty"` // 复权类型(可选, 默认前复权 qfq)
}

// RunKLineData 运行单只股票K线采集
// POST /api/v1/collector/kline/:code
func (h *DataCollectorHandler) RunKLineData(c *gin.Context) {
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

	go func() {
		result, err := h.service.CollectKLine(req.Source, code, req.KLineType, req.AdjType)
		if err != nil {
			log.Printf("[collector] K线采集失败 [%s/%s]: %v", code, req.KLineType, err)
			return
		}
		log.Printf("[collector] K线采集完成 [%s/%s]: total=%d, new=%d, upd=%d", code, req.KLineType, result.Total, result.NewCount, result.UpdCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("K线采集已启动: %s, 周期=%s, 源=%s", code, req.KLineType, req.Source),
	})
}

// RunKLineBatch 运行全量股票K线采集
// POST /api/v1/collector/kline-batch
func (h *DataCollectorHandler) RunKLineBatch(c *gin.Context) {
	var req KLineCollectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.KLineType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kline_type 必填 (daily/weekly/monthly/yearly)"})
		return
	}

	go func() {
		result, err := h.service.CollectKLineBatch(req.Source, req.KLineType, req.AdjType)
		if err != nil {
			log.Printf("[collector] 全量K线采集失败 [%s]: %v", req.KLineType, err)
			return
		}
		log.Printf("[collector] 全量K线采集完成 [%s]: total=%d, new=%d, upd=%d, fail=%d",
			req.KLineType, result.Total, result.NewCount, result.UpdCount, result.FailCount)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("全量%sK线采集已启动, 数据源=%s", req.KLineType, req.Source),
	})
}

// HealthCheck 健康检查
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"version":   "1.0.0",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
