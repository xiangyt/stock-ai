package handler

import (
	"net/http"

	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// ========== 指数日线 ==========

// IndexKLineHandler 指数日线查询
type IndexKLineHandler struct {
	svc *service.IndexKLineService
}

// NewIndexKLineHandler 创建指数日线处理器
func NewIndexKLineHandler() *IndexKLineHandler {
	return &IndexKLineHandler{svc: service.NewIndexKLineService()}
}

// Daily 指数日线
//
// GET /api/v1/index-kline?code=000001&start=2026-09-01&end=2026-10-31
//
// code 省略时默认上证指数(000001)；start/end 为 YYYY-MM-DD。
func (h *IndexKLineHandler) Daily(c *gin.Context) {
	code := c.DefaultQuery("code", "000001")
	startDate := c.Query("start")
	endDate := c.Query("end")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start 与 end 不能为空，格式 YYYY-MM-DD"})
		return
	}

	result, err := h.svc.Daily(c.Request.Context(), code, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
