package handler

import (
	"net/http"

	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// SoftwareConfigHandler 软件配置 Handler
type SoftwareConfigHandler struct {
	svc *service.SoftwareConfigService
}

// NewSoftwareConfigHandler 创建软件配置 Handler
func NewSoftwareConfigHandler(svc *service.SoftwareConfigService) *SoftwareConfigHandler {
	return &SoftwareConfigHandler{svc: svc}
}

// ListSupported 获取支持的软件列表
// GET /api/v1/auth/software-configs/supported
func (h *SoftwareConfigHandler) ListSupported(c *gin.Context) {
	list := h.svc.ListSupportedSoftware()
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// ListConfigs 获取当前用户的软件配置列表
// GET /api/v1/auth/software-configs
func (h *SoftwareConfigHandler) ListConfigs(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	items, err := h.svc.ListUserConfigs(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// UpdateConfig 更新当前用户的某个软件配置
// PUT /api/v1/auth/software-configs/:name
func (h *SoftwareConfigHandler) UpdateConfig(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少软件名称"})
		return
	}

	var req service.UpdateSoftwareConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	item, err := h.svc.UpdateUserConfig(userID, name, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}
