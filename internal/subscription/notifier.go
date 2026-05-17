package subscription

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"stock-ai/internal/model"
)

// ============================================================================
//  Notifier 通知推送接口 + 实现
// ============================================================================

// Notifier 通知推送接口
type Notifier interface {
	// Send 通过指定机器人发送消息，失败时自动重试 1 次
	Send(ctx context.Context, bot *model.PushBot, message string) error

	// Render 渲染通知消息（模板 + 变量替换）
	Render(template string, vars map[string]string) string
}

// notifierImpl Notifier 接口实现
type notifierImpl struct{}

// NewNotifier 创建 Notifier 实例
func NewNotifier() Notifier {
	return &notifierImpl{}
}

// Send 通过指定机器人发送消息，失败时自动重试 1 次
func (n *notifierImpl) Send(ctx context.Context, bot *model.PushBot, message string) error {
	err := n.sendOnce(bot, message)
	if err != nil {
		// 自动重试 1 次
		time.Sleep(500 * time.Millisecond)
		err = n.sendOnce(bot, message)
	}
	return err
}

// sendOnce 单次发送
func (n *notifierImpl) sendOnce(bot *model.PushBot, message string) error {
	switch bot.Channel {
	case "dingtalk":
		payload := DingTalkPayload(message)
		jsonBytes, _ := json.Marshal(payload)
		return SendDingTalk(bot.WebhookURL, bot.Secret, jsonBytes)
	case "feishu":
		payload := FeishuPayload(message)
		jsonBytes, _ := json.Marshal(payload)
		return PostJSON(bot.WebhookURL, jsonBytes)
	case "wecom":
		payload := WecomPayload(message)
		jsonBytes, _ := json.Marshal(payload)
		return PostJSON(bot.WebhookURL, jsonBytes)
	default:
		return fmt.Errorf("不支持的渠道: %s", bot.Channel)
	}
}

// Render 渲染通知消息（模板 + 变量替换）
func (n *notifierImpl) Render(template string, vars map[string]string) string {
	return RenderTemplate(template, vars)
}

// ============================================================================
//  导出发送函数（从 bot_handler.go 抽离，供 handler 包和其他模块调用）
// ============================================================================

// DingTalkPayload 构建钉钉消息体
func DingTalkPayload(content string) interface{} {
	return map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}
}

// SendDingTalk 钉钉推送（带加签）
func SendDingTalk(webhook, secret string, body []byte) error {
	url := webhook
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sign := dingTalkSign(timestamp, secret)
		url = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhook, timestamp, sign)
	}
	return PostJSON(url, body)
}

// dingTalkSign 计算钉钉加签（包内可见）
func dingTalkSign(timestamp, secret string) string {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}

// FeishuPayload 构建飞书消息体
func FeishuPayload(content string) interface{} {
	return map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	}
}

// WecomPayload 构建企微消息体
func WecomPayload(content string) interface{} {
	return map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": content,
		},
	}
}

// PostJSON 通用的 JSON POST 请求
func PostJSON(url string, body []byte) error {
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
