package handler

import (
	"net/http"
	"strconv"
	"strings"

	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// ============================================================================
//  MonitorConfigHandler 监控配置 HTTP Handler
// ============================================================================

// MonitorConfigHandler 监控配置 HTTP Handler
type MonitorConfigHandler struct {
	svc *service.MonitorConfigService
}

// NewMonitorConfigHandler 创建监控配置 Handler
func NewMonitorConfigHandler(svc *service.MonitorConfigService) *MonitorConfigHandler {
	return &MonitorConfigHandler{svc: svc}
}

// ============================================================================
//  错误判断辅助函数（复用 subscription_handler 的 containsAny）
// ============================================================================

func errCode(errMsg string) int {
	if containsAny(errMsg, "不存在", "无权") {
		return 404
	}
	if containsAny(errMsg, "上限", "格式无效", "必须为", "不能为空", "无效", "只能", "至少") {
		return 400
	}
	return 500
}

// ============================================================================
//  CRUD 接口
// ============================================================================

// List 获取监控配置列表
// GET /api/v1/monitor-configs
func (h *MonitorConfigHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	details, total, err := h.svc.List(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"data":      details,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Create 创建监控配置
// POST /api/v1/monitor-configs
func (h *MonitorConfigHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	var req service.CreateMonitorConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.Create(&req, userID)
	if err != nil {
		c.JSON(errCode(err.Error()), gin.H{"code": errCode(err.Error()), "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
}

// GetByID 获取监控配置详情
// GET /api/v1/monitor-configs/:id
func (h *MonitorConfigHandler) GetByID(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的ID"})
		return
	}

	detail, err := h.svc.GetByID(uint(id), userID)
	if err != nil {
		c.JSON(errCode(err.Error()), gin.H{"code": errCode(err.Error()), "message": "配置不存在或无权访问"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
}

// Update 更新监控配置
// PUT /api/v1/monitor-configs/:id
func (h *MonitorConfigHandler) Update(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的ID"})
		return
	}

	var req service.UpdateMonitorConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.Update(uint(id), userID, &req)
	if err != nil {
		c.JSON(errCode(err.Error()), gin.H{"code": errCode(err.Error()), "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
}

// Delete 删除监控配置
// DELETE /api/v1/monitor-configs/:id
func (h *MonitorConfigHandler) Delete(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的ID"})
		return
	}

	if err := h.svc.Delete(uint(id), userID); err != nil {
		c.JSON(errCode(err.Error()), gin.H{"code": errCode(err.Error()), "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已删除"})
}

// SetActive 切换启用/停用
// PATCH /api/v1/monitor-configs/:id/active
func (h *MonitorConfigHandler) SetActive(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的ID"})
		return
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if err := h.svc.SetActive(uint(id), userID, req.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	status := "已启用"
	if !req.IsActive {
		status = "已停用"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": status})
}

// UpdateBots 更新配置关联的机器人
// PUT /api/v1/monitor-configs/:id/bots
func (h *MonitorConfigHandler) UpdateBots(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的ID"})
		return
	}

	var req struct {
		BotIDs []uint `json:"bot_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if err := h.svc.UpdateBots(uint(id), userID, req.BotIDs); err != nil {
		code := 500
		if containsAny(err.Error(), "上限", "不存在") {
			code = 400
		}
		c.JSON(code, gin.H{"code": code, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已更新机器人关联"})
}

// strings 包引用确认
var _ = strings.TrimSpace
