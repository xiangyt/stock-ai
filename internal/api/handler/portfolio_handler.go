package handler

import (
	"net/http"
	"strconv"

	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// PortfolioHandler 持仓管理 HTTP Handler
type PortfolioHandler struct {
	svc *service.PortfolioService
}

// NewPortfolioHandler 创建持仓管理 Handler
func NewPortfolioHandler(svc *service.PortfolioService) *PortfolioHandler {
	return &PortfolioHandler{svc: svc}
}

// getPortfolioUID 从上下文获取用户 ID
func (h *PortfolioHandler) getPortfolioUID(c *gin.Context) (uint, bool) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return 0, false
	}
	return userID, true
}

// parsePositionID 从 URL 参数解析 ID
func (h *PortfolioHandler) parsePositionID(c *gin.Context, param string) (uint, bool) {
	idStr := c.Param(param)
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return 0, false
	}
	return uint(id), true
}

// ListPositions 获取持仓列表
// GET /api/v1/positions
func (h *PortfolioHandler) ListPositions(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := c.Query("status")

	resp, err := h.svc.ListPositions(uid, status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// GetPositionByID 获取持仓详情（含交易记录）
// GET /api/v1/positions/:id
func (h *PortfolioHandler) GetPositionByID(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}
	id, ok := h.parsePositionID(c, "id")
	if !ok {
		return
	}

	detail, err := h.svc.GetPositionByID(id, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// OpenPosition 建仓
// POST /api/v1/positions
func (h *PortfolioHandler) OpenPosition(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}

	var req service.OpenPositionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.OpenPosition(&req, uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": detail})
}

// BuyMore 加仓
// POST /api/v1/positions/:id/buy
func (h *PortfolioHandler) BuyMore(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}
	id, ok := h.parsePositionID(c, "id")
	if !ok {
		return
	}

	var req service.TradeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.BuyMore(id, &req, uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// SellPartial 减仓
// POST /api/v1/positions/:id/sell
func (h *PortfolioHandler) SellPartial(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}
	id, ok := h.parsePositionID(c, "id")
	if !ok {
		return
	}

	var req service.TradeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.SellPartial(id, &req, uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// ClosePosition 清仓
// POST /api/v1/positions/:id/close
func (h *PortfolioHandler) ClosePosition(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}
	id, ok := h.parsePositionID(c, "id")
	if !ok {
		return
	}

	var req struct {
		Price     float64 `json:"price" binding:"required,gt=0"`
		TradeDate string  `json:"trade_date" binding:"required"`
		Note      string  `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.ClosePosition(id, req.Price, req.TradeDate, req.Note, uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// UpdatePositionNote 更新持仓备注
// PUT /api/v1/positions/:id
func (h *PortfolioHandler) UpdatePositionNote(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}
	id, ok := h.parsePositionID(c, "id")
	if !ok {
		return
	}

	var req service.UpdatePositionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if err := h.svc.UpdatePositionNote(id, req.Note, uid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeletePosition 删除持仓
// DELETE /api/v1/positions/:id
func (h *PortfolioHandler) DeletePosition(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}
	id, ok := h.parsePositionID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.DeletePosition(id, uid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetTradeConfig 获取用户交易配置
// GET /api/v1/user/trade-config
func (h *PortfolioHandler) GetTradeConfig(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}

	config, err := h.svc.GetTradeConfig(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

// UpdateTradeConfig 更新用户交易配置
// PUT /api/v1/user/trade-config
func (h *PortfolioHandler) UpdateTradeConfig(c *gin.Context) {
	uid, ok := h.getPortfolioUID(c)
	if !ok {
		return
	}

	var req service.TradeConfigUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := h.svc.UpdateTradeConfig(uid, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}
