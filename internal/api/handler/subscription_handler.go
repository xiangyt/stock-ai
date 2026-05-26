package handler

import (
	"net/http"
	"strconv"
	"strings"

	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// ============================================================================
//  SubscriptionHandler 订阅 HTTP Handler
// ============================================================================

// SubscriptionHandler 订阅 HTTP Handler
type SubscriptionHandler struct {
	svc *service.SubscriptionService
}

// NewSubscriptionHandler 创建订阅 Handler
func NewSubscriptionHandler(svc *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc}
}

// Create 创建订阅
// POST /api/v1/subscriptions
func (h *SubscriptionHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	var req service.CreateSubscriptionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.Create(&req, userID)
	if err != nil {
		// 判断错误类型
		errMsg := err.Error()
		code := 500
		if containsAny(errMsg, "不存在", "无权") {
			code = 404
		} else if containsAny(errMsg, "上限", "格式无效", "必须为", "不能为空", "无效") {
			code = 400
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
}

// List 获取订阅列表
// GET /api/v1/subscriptions
func (h *SubscriptionHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// 可选 is_active 过滤
	var isActive *bool
	if activeStr := c.Query("is_active"); activeStr != "" {
		active := activeStr == "true" || activeStr == "1"
		isActive = &active
	}

	result, err := h.svc.List(userID, page, pageSize, isActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// GetByID 获取订阅详情
// GET /api/v1/subscriptions/:id
func (h *SubscriptionHandler) GetByID(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	detail, err := h.svc.GetByID(id, userID)
	if err != nil {
		errMsg := err.Error()
		code := 500
		if containsAny(errMsg, "不存在", "无权") {
			code = 404
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
}

// Update 更新订阅
// PUT /api/v1/subscriptions/:id
func (h *SubscriptionHandler) Update(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	var req service.UpdateSubscriptionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.Update(id, userID, &req)
	if err != nil {
		errMsg := err.Error()
		code := 500
		if containsAny(errMsg, "不存在", "无权") {
			code = 404
		} else if containsAny(errMsg, "无效", "格式") {
			code = 400
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
}

// Delete 删除订阅
// DELETE /api/v1/subscriptions/:id
func (h *SubscriptionHandler) Delete(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	err := h.svc.Delete(id, userID)
	if err != nil {
		errMsg := err.Error()
		code := 500
		if containsAny(errMsg, "不存在", "无权") {
			code = 404
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// SetActive 切换订阅启停状态
// PATCH /api/v1/subscriptions/:id/active
func (h *SubscriptionHandler) SetActive(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	err := h.svc.SetActive(id, userID, req.IsActive)
	if err != nil {
		errMsg := err.Error()
		code := 500
		if containsAny(errMsg, "不存在", "无权") {
			code = 404
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	action := "停用"
	if req.IsActive {
		action = "启用"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已" + action})
}

// TriggerRun 手动触发订阅执行
// POST /api/v1/subscriptions/:id/run
func (h *SubscriptionHandler) TriggerRun(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	result, err := h.svc.TriggerRun(id, userID, isAdmin(c))
	if err != nil {
		errMsg := err.Error()
		code := 500
		if containsAny(errMsg, "不存在", "无权") {
			code = 404
		} else if containsAny(errMsg, "已停用", "未初始化") {
			code = 400
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// UpdateBots 更新订阅关联的机器人
// PUT /api/v1/subscriptions/:id/bots
func (h *SubscriptionHandler) UpdateBots(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	var req struct {
		BotIDs []uint `json:"bot_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	err := h.svc.UpdateBots(id, userID, req.BotIDs)
	if err != nil {
		errMsg := err.Error()
		code := 500
		if containsAny(errMsg, "不存在", "无权") {
			code = 404
		} else if containsAny(errMsg, "不能超过", "已禁用") {
			code = 400
		}
		c.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// containsAny 检查字符串是否包含任一子串
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
