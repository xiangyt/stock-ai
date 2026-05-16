package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/model"

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

// PushHandler 推送配置 HTTP Handler
type PushHandler struct{}

// NewPushHandler 创建推送 Handler
func NewPushHandler() *PushHandler {
	return &PushHandler{}
}

// ====== 请求/响应类型 ======

type createPushReq struct {
	Name       string `json:"name" binding:"required"`
	Channel    string `json:"channel" binding:"required"`
	WebhookURL string `json:"webhook_url"`
	Token      string `json:"token"`
	Secret     string `json:"secret"`
}

type updatePushReq struct {
	Name       string `json:"name"`
	Channel    string `json:"channel"`
	WebhookURL string `json:"webhook_url"`
	Token      string `json:"token"`
	Secret     string `json:"secret"`
}

type toggleStatusReq struct {
	Status int `json:"status" binding:"required"`
}

type pushBotItem struct {
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

// List 获取当前用户的推送配置列表
// GET /api/v1/push-configs
func (h *PushHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	list, err := db.ListPushConfigs(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询推送配置失败"})
		return
	}

	result := make([]pushBotItem, 0, len(list))
	for _, cfg := range list {
		result = append(result, pushBotItem{
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

// Create 创建推送配置
// POST /api/v1/push-configs
func (h *PushHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var req createPushReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if !isValidChannel(req.Channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "渠道必须是 wecom / dingtalk / feishu 之一"})
		return
	}

	cfg := &model.PushConfig{
		UserID:     userID,
		Name:       req.Name,
		Channel:    req.Channel,
		WebhookURL: req.WebhookURL,
		Token:      req.Token,
		Secret:     req.Secret,
		Status:     1,
	}

	if err := db.CreatePushConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"id": cfg.ID,
	}, "message": "创建成功"})
}

// Update 更新推送配置
// PUT /api/v1/push-configs/:id
func (h *PushHandler) Update(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	var req updatePushReq
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

	if err := db.UpdatePushConfig(id, userID, updates); err != nil {
		if err == db.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在或无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// Delete 删除推送配置
// DELETE /api/v1/push-configs/:id
func (h *PushHandler) Delete(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	if err := db.DeletePushConfig(id, userID); err != nil {
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
// PUT /api/v1/push-configs/:id/status
func (h *PushHandler) ToggleStatus(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	var req toggleStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status 只能是 0 或 1"})
		return
	}
	if req.Status != 0 && req.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status 只能是 0 或 1"})
		return
	}

	if err := db.UpdatePushStatus(id, userID, req.Status); err != nil {
		if err == db.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在或无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新状态失败"})
		return
	}

	action := "禁用"
	if req.Status == 1 {
		action = "启用"
	}
	c.JSON(http.StatusOK, gin.H{"message": "已" + action})
}

// Test 测试推送
// POST /api/v1/push-configs/:id/test
func (h *PushHandler) Test(c *gin.Context) {
	userID := c.GetUint("user_id")
	id := parseID(c)
	if id == 0 {
		return
	}

	cfg, err := db.GetPushConfigByID(id, userID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在或无权操作"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询配置失败"})
		return
	}

	msg := fmt.Sprintf("【AI选股】测试推送\n机器人: %s\n渠道: %s\n时间: %s",
		cfg.Name, cfg.Channel, time.Now().Format("2006-01-02 15:04:05"))

	var httpErr error

	switch cfg.Channel {
	case "dingtalk":
		payload := dingTalkPayload(msg)
		jsonBytes, _ := json.Marshal(payload)
		httpErr = sendDingTalk(cfg.WebhookURL, cfg.Secret, jsonBytes)
	case "feishu":
		payload := feishuPayload(msg)
		jsonBytes, _ := json.Marshal(payload)
		httpErr = postJSON(cfg.WebhookURL, jsonBytes)
	case "wecom":
		payload := wecomPayload(msg)
		jsonBytes, _ := json.Marshal(payload)
		httpErr = postJSON(cfg.WebhookURL, jsonBytes)
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

// ============================================================
//  钉钉 Webhook（带加签）
// ============================================================

// dingTalkPayload 构建钉钉消息体
func dingTalkPayload(content string) interface{} {
	return map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}
}

// sendDingTalk 钉钉推送（带加签）
func sendDingTalk(webhook, secret string, body []byte) error {
	url := webhook
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sign := dingTalkSign(timestamp, secret)
		url = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhook, timestamp, sign)
	}
	return postJSON(url, body)
}

// dingTalkSign 计算钉钉加签
func dingTalkSign(timestamp, secret string) string {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}

// ============================================================
//  飞书 Webhook
// ============================================================

// feishuPayload 构建飞书消息体
func feishuPayload(content string) interface{} {
	return map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	}
}

// ============================================================
//  企业微信 Webhook
// ============================================================

// wecomPayload 构建企微消息体
func wecomPayload(content string) interface{} {
	return map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": content,
		},
	}
}

// ============================================================
//  通用工具
// ============================================================

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

// postJSON 通用的 JSON POST 请求
func postJSON(url string, body []byte) error {
	resp, err := http.Post(url, "application/json; charset=utf-8", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
