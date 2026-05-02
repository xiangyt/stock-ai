package handler

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"stock-ai/internal/db"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证 HTTP Handler
type AuthHandler struct {
	svc *service.AuthService
}

// NewAuthHandler 创建认证 Handler
func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login 登录
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}

	resp, err := h.svc.Login(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// GetMe 获取当前用户信息
// GET /api/v1/auth/me
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	info, err := h.svc.GetCurrentUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": info})
}

// UpdateAccount 更新账号信息（昵称/头像/密码）
// PUT /api/v1/auth/account
func (h *AuthHandler) UpdateAccount(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var req service.UpdateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	info, err := h.svc.UpdateAccount(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": info})
}

// Register 注册
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空，密码至少6位"})
		return
	}

	info, err := h.svc.Register(req.Username, req.Password, req.Nickname)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": info})
}

// ======== 管理员接口（仅 role=admin 可调用） ========

// adminRequired 校验当前用户是否为管理员
func (h *AuthHandler) adminRequired(c *gin.Context) (uint, bool) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return 0, false
	}
	user, err := db.GetUserByID(userID)
	if err != nil || user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return 0, false
	}
	return userID, true
}

// ListUsers 获取所有用户列表
// GET /api/v1/auth/admin/users
func (h *AuthHandler) ListUsers(c *gin.Context) {
	if _, ok := h.adminRequired(c); !ok {
		return
	}

	users, err := db.ListAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}

	// 返回给前端的信息（不含密码）
	type userItem struct {
		ID        uint   `json:"id"`
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		Avatar    string `json:"avatar"`
		Role      string `json:"role"`
		Status    int    `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	list := make([]userItem, 0, len(users))
	for _, u := range users {
		list = append(list, userItem{
			ID:        u.ID,
			Username:  u.Username,
			Nickname:  u.Nickname,
			Avatar:    u.Avatar,
			Role:      u.Role,
			Status:    u.Status,
			CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": list})
}

// ToggleUserStatus 启用/禁用用户
// PUT /api/v1/auth/admin/users/:id/status
func (h *AuthHandler) ToggleUserStatus(c *gin.Context) {
	adminID, ok := h.adminRequired(c)
	if !ok {
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID"})
		return
	}

	// 不允许操作自己
	if uint(id) == adminID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能修改自己的状态"})
		return
	}

	var req struct {
		Status int `json:"status"` // 1=启用 0=禁用
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if req.Status != 0 && req.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status 只能是 0 或 1"})
		return
	}

	if err := db.UpdateUserStatus(uint(id), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	action := "禁用"
	if req.Status == 1 {
		action = "启用"
	}
	c.JSON(http.StatusOK, gin.H{"message": "已" + action + "该用户"})
}

// ResetUserPassword 重置用户密码
// PUT /api/v1/auth/admin/users/:id/password
func (h *AuthHandler) ResetUserPassword(c *gin.Context) {
	if _, ok := h.adminRequired(c); !ok {
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID"})
		return
	}

	var req struct {
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少6位"})
		return
	}

	// MD5 哈希
	hash := md5.Sum([]byte(req.Password))
	passwordHex := hex.EncodeToString(hash[:])

	if err := db.UpdatePassword(uint(id), passwordHex); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码已重置"})
}

// ======== 工具函数 ========

// ExtractToken 从请求头提取 Bearer token
func ExtractToken(c *gin.Context) string {
	bearer := c.GetHeader("Authorization")
	if strings.HasPrefix(bearer, "Bearer ") {
		return strings.TrimPrefix(bearer, "Bearer ")
	}
	return ""
}
