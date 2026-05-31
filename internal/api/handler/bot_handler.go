package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/notifier"

	"github.com/gin-gonic/gin"
)

// validChannels 合法渠道列表
var validChannels = map[string]bool{
	"wecom": true, "dingtalk": true, "feishu": true,
}

// isValidChannel 检查渠道是否合法
func isValidChannel(ch string) bool {
	return validChannels[ch]
}

// BotHandler 机器人配置 HTTP Handler
type BotHandler struct{}

// NewBotHandler 创建推送 Handler
func NewBotHandler() *BotHandler {
	return &BotHandler{}
}

// ====== 权限辅助 ======

// checkBotOwner 验证当前用户是否有权操作指定机器人。
// 规则：
//   - 管理员：可操作所有管理员创建的机器人
//   - 普通用户：只能操作自己创建的机器人
//
// 返回 bot 对象（后续 DB 操作需要用原始 ownerID 做过滤）和是否允许。
func (h *BotHandler) checkBotOwner(c *gin.Context, botID uint) (*model.PushBot, bool) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return nil, false
	}

	bot, err := db.GetBotByID(botID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return nil, false
	}

	// 自己的机器人 → 直接放行
	if bot.UserID == userID {
		return bot, true
	}

	// 管理员可以操作其他管理员创建的机器人
	if isAdmin(c) {
		creator, err := db.GetUserByID(bot.UserID)
		if err == nil && creator.Role == "admin" {
			return bot, true
		}
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此配置"})
	return nil, false
}

// ====== 请求/响应类型 ======

type createBotReq struct {
	Name       string `json:"name" binding:"required"`
	Channel    string `json:"channel" binding:"required"`
	WebhookURL string `json:"webhook_url"`
	Token      string `json:"token"`
	Secret     string `json:"secret"`
}

type updateBotReq struct {
	Name       string `json:"name"`
	Channel    string `json:"channel"`
	WebhookURL string `json:"webhook_url"`
	Token      string `json:"token"`
	Secret     string `json:"secret"`
}

type toggleStatusReq struct {
	Status *int `json:"status"`
}

type botItem struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"user_id"`
	Name       string `json:"name"`
	Channel    string `json:"channel"`
	WebhookURL string `json:"webhook_url"`
	Token      string `json:"token"`
	Secret     string `json:"secret"`
	Status     int    `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// ====== CRUD ======

// List 获取推送配置列表
// GET /api/v1/bots
// 管理员：显示所有管理员创建的机器人
// 普通用户：仅显示自己的机器人
func (h *BotHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var (
		list []model.PushBot
		err  error
	)
	if isAdmin(c) {
		list, err = db.ListPushBotsForAdmin()
	} else {
		list, err = db.ListPushBots(userID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询机器人失败"})
		return
	}

	result := make([]botItem, 0, len(list))
	for _, cfg := range list {
		result = append(result, botItem{
			ID:         cfg.ID,
			UserID:     cfg.UserID,
			Name:       cfg.Name,
			Channel:    cfg.Channel,
			WebhookURL: cfg.WebhookURL,
			Token:      cfg.Token,
			Secret:     cfg.Secret,
			Status:     cfg.Status,
			CreatedAt:  cfg.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:  cfg.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Create 创建机器人配置
// POST /api/v1/bots
func (h *BotHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var req createBotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if !isValidChannel(req.Channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "渠道必须是 wecom / dingtalk / feishu 之一"})
		return
	}

	cfg := &model.PushBot{
		UserID:     userID,
		Name:       req.Name,
		Channel:    req.Channel,
		WebhookURL: req.WebhookURL,
		Token:      req.Token,
		Secret:     req.Secret,
		Status:     1,
	}

	if err := db.CreatePushBot(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"id": cfg.ID,
	}, "message": "创建成功"})
}

// Update 更新机器人配置
// PUT /api/v1/bots/:id
func (h *BotHandler) Update(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}

	bot, ok := h.checkBotOwner(c, id)
	if !ok {
		return
	}

	var req updateBotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Channel != "" && !isValidChannel(req.Channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "渠道必须是 wecom / dingtalk / feishu 之一"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Channel != "" {
		updates["channel"] = req.Channel
	}
	if req.WebhookURL != "" {
		updates["webhook_url"] = req.WebhookURL
	}
	if req.Token != "" || c.GetHeader("X-Clear-Token") == "1" {
		updates["token"] = req.Token
	}
	if req.Secret != "" || c.GetHeader("X-Clear-Secret") == "1" {
		updates["secret"] = req.Secret
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有需要更新的字段"})
		return
	}

	// 使用 bot 的原始 owner ID（管理员操作他人机器人时 userID 不等于 owner）
	if err := db.UpdatePushBot(id, bot.UserID, updates); err != nil {
		if err == db.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在或无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// Delete 删除机器人配置
// DELETE /api/v1/bots/:id
func (h *BotHandler) Delete(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}

	bot, ok := h.checkBotOwner(c, id)
	if !ok {
		return
	}

	if err := db.DeletePushBot(id, bot.UserID); err != nil {
		if err == db.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在或无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ToggleStatus 切换启用/禁用状态
// PUT /api/v1/bots/:id/status
func (h *BotHandler) ToggleStatus(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}

	bot, ok := h.checkBotOwner(c, id)
	if !ok {
		return
	}

	var req toggleStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}
	if req.Status == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 status 参数"})
		return
	}
	s := *req.Status
	if s != 0 && s != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status 只能是 0 或 1"})
		return
	}

	if err := db.UpdatePushStatus(id, bot.UserID, s); err != nil {
		if err == db.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在或无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新状态失败"})
		return
	}

	action := "禁用"
	if s == 1 {
		action = "启用"
	}
	c.JSON(http.StatusOK, gin.H{"message": "已" + action})
}

// Test 测试机器人
// POST /api/v1/bots/:id/test
func (h *BotHandler) Test(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}

	bot, ok := h.checkBotOwner(c, id)
	if !ok {
		return
	}
	cfg := bot // bot 已经是完整对象

	msg := fmt.Sprintf("【AI选股】测试推送\n机器人: %s\n渠道: %s\n时间: %s",
		cfg.Name, cfg.Channel, time.Now().Format("2006-01-02 15:04:05"))

	var httpErr error

	switch cfg.Channel {
	case "dingtalk":
		payload := notifier.DingTalkPayload(msg)
		jsonBytes, _ := json.Marshal(payload)
		httpErr = notifier.SendDingTalk(cfg.WebhookURL, cfg.Secret, jsonBytes)
	case "feishu":
		payload := notifier.FeishuPayload(msg)
		jsonBytes, _ := json.Marshal(payload)
		httpErr = notifier.PostJSON(cfg.WebhookURL, jsonBytes)
	case "wecom":
		payload := notifier.WecomPayload(msg)
		jsonBytes, _ := json.Marshal(payload)
		httpErr = notifier.PostJSON(cfg.WebhookURL, jsonBytes)
	default:
		httpErr = fmt.Errorf("不支持的渠道: %s", cfg.Channel)
	}

	if httpErr != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"success": false,
			"message": "推送失败: " + httpErr.Error(),
		}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"success": true,
		"message": "测试消息发送成功",
	}})
}

// parseID 从 URL 参数解析 ID，出错时直接写响应并返回 0
func parseID(c *gin.Context) uint {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return 0
	}
	return uint(id)
}
