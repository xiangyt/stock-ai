package handler

import (
	"context"
	"log"
	"net/http"
	"strings"

	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// SnapshotHandler 每日估值快照处理器
type SnapshotHandler struct {
	service *service.SnapshotService
}

// NewSnapshotHandler 创建快照处理器
func NewSnapshotHandler() *SnapshotHandler {
	return &SnapshotHandler{
		service: service.NewSnapshotService(),
	}
}

// SnapshotRequest 快照计算请求
type SnapshotRequest struct {
	Code string `json:"code"` // 股票代码（空=所有股票）
}

// Calc 快照计算入口
// POST /api/v1/snapshot/calc
//
//	code 为空 → 全股票全日期
//	有 code  → 单股票全日期（从同花顺实时采集K线）
func (h *SnapshotHandler) Calc(c *gin.Context) {
	var req SnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		result := h.service.Calc(context.Background(), strings.TrimSpace(req.Code))
		log.Printf("[snapshot] 全部完成! 成功=%d 失败=%d 耗时=%.1fs",
			result.Success, result.Fail, result.CostSeconds)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "快照计算已启动",
	})
}
