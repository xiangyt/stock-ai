package handler

import (
	"net/http"
	"strconv"

	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

type StockHandler struct {
	service *service.StockService
}

func NewStockHandler() *StockHandler {
	return &StockHandler{
		service: service.NewStockService(),
	}
}

// FilterStocks 条件选股
func (h *StockHandler) FilterStocks(c *gin.Context) {
	var req service.FilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.FilterStocks(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// AIQuery AI自然语言选股
func (h *StockHandler) AIQuery(c *gin.Context) {
	var req struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.AIQuery(req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetHotTopics 获取热门题材
func (h *StockHandler) GetHotTopics(c *gin.Context) {
	resp, err := h.service.GetHotTopics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetStockDetail 获取股票详情
func (h *StockHandler) GetStockDetail(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	resp, err := h.service.GetStockDetail(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetStockPrices 获取股票历史价格
func (h *StockHandler) GetStockPrices(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		days = 30
	}

	resp, err := h.service.GetStockPrices(code, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

