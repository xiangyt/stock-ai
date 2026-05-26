package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"stock-ai/internal/service"
	"github.com/gin-gonic/gin"
)

// StrategyHandler 策略 HTTP Handler
type StrategyHandler struct {
	svc *service.StrategyService
}

// NewStrategyHandler 创建策略 Handler 实例
func NewStrategyHandler() *StrategyHandler {
	return &StrategyHandler{
		svc: service.NewStrategyService(),
	}
}

// getUID 从 JWT 中间件注入的 context 中提取当前用户 ID
func getUID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	return userID.(uint)
}

// isAdmin 判断当前用户是否为管理员
func isAdmin(c *gin.Context) bool {
	role, exists := c.Get("user_role")
	if !exists {
		return false
	}
	return role.(string) == "admin"
}

// checkOwner 校验策略是否属于当前用户（管理员可操作任意策略）
func (h *StrategyHandler) checkOwner(c *gin.Context, id uint) error {
	if isAdmin(c) {
		return nil
	}
	detail, err := h.svc.GetByID(id)
	if err != nil {
		return fmt.Errorf("策略不存在")
	}
	if detail.UID != getUID(c) {
		return fmt.Errorf("无权操作此策略")
	}
	return nil
}

// Create 创建策略
// POST /api/v1/strategies
func (h *StrategyHandler) Create(c *gin.Context) {
	var req service.CreateStrategyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.Create(&req, getUID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// List 获取策略列表（仅当前用户的策略）
// GET /api/v1/strategies?keyword=xxx&page=1&size=20
func (h *StrategyHandler) List(c *gin.Context) {
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	resp, err := h.svc.List(getUID(c), isAdmin(c), keyword, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// GetByID 获取策略详情
// GET /api/v1/strategies/:id
func (h *StrategyHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	detail, err := h.svc.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
		return
	}
	if detail.UID != getUID(c) && !isAdmin(c) && !detail.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问此策略"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// Update 更新策略
// PUT /api/v1/strategies/:id
func (h *StrategyHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.checkOwner(c, uint(id)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req service.CreateStrategyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	detail, err := h.svc.Update(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// Rename 重命名策略
// PUT /api/v1/strategies/:id/rename
func (h *StrategyHandler) Rename(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.checkOwner(c, uint(id)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req service.RenameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := h.svc.Rename(uint(id), req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重命名失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "重命名成功"})
}

// Delete 删除单个策略
// DELETE /api/v1/strategies/:id
func (h *StrategyHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.checkOwner(c, uint(id)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.Delete(uint(id), getUID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// BatchDelete 批量删除策略
// DELETE /api/v1/strategies/batch?ids=1,2,3
func (h *StrategyHandler) BatchDelete(c *gin.Context) {
	var req service.BatchDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := h.svc.BatchDelete(req.IDs, getUID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "批量删除成功"})
}

// SetPublic 切换策略公开/私有状态
// PUT /api/v1/strategies/:id/public
func (h *StrategyHandler) SetPublic(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.checkOwner(c, uint(id)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		IsPublic bool `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := h.svc.SetPublic(uint(id), req.IsPublic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	msg := "已设为私有"
	if req.IsPublic {
		msg = "已设为公开"
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

// Copy 复制策略（仅公开策略或自己的策略可复制）
// POST /api/v1/strategies/:id/copy
func (h *StrategyHandler) Copy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	detail, err := h.svc.Copy(uint(id), getUID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "复制失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail, "message": "复制成功"})
}
