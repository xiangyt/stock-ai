package monitor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"stock-ai/internal/model"
	"stock-ai/internal/notifier"
)

// ============================================================================
//  PushBuilder — 消息构建器
// ============================================================================

// PushBuilder 构建告警推送消息
type PushBuilder struct{}

// NewPushBuilder 创建消息构建器
func NewPushBuilder() *PushBuilder {
	return &PushBuilder{}
}

// getDefaultTemplate 返回规则类型的默认推送文案
func getDefaultTemplate(ruleType string) string {
	switch ruleType {
	case string(model.RuleTypeRapidMove):
		return "[${alert_label}] ${name}(${code}) ${change_pct}% 报 ${price}"
	case string(model.RuleTypeVolumeRatio):
		return "[${alert_label}] ${name}(${code}) 量比 ${ratio} 成交 ${volume}"
	case string(model.RuleTypeSealBoard):
		return "[${alert_label}] ${name}(${code}) 封单不足 ${min_lots}手"
	default: // daily_change
		return "[${alert_label}] ${name}(${code}) 报 ${price} (${change_pct}%)"
	}
}

// Build 渲染推送消息
//
// 用户自定义模板支持变量:
//
//	${name}          — 股票名称
//	${code}          — 股票代码
//	${alert_type}    — 规则大类 "daily_change" / "short_term_volatility" / "seal_board"
//	${alert_subtype} — 告警子类型 "surge_big" / "振幅超标" / "封单不足" 等
//	${alert_label}   — 告警中文标签 "涨停" / "大跌" / "振幅超标" 等
//	${price}         — 当前价格（元）
//	${change_pct}    — 涨跌幅 %
//	${turnover}      — 换手率 %
//	${volume}        — 成交量
func (p *PushBuilder) Build(cfg *model.MonitorConfig, alert Alert, data *model.QuoteData) string {
	if data == nil {
		return ""
	}

	// 股票名称：优先缓存，否则用代码
	name := data.Name
	if name == "" {
		name = data.Code
	}

	// 模板：优先用户自定义，否则默认
	tmpl := cfg.Template
	if strings.TrimSpace(tmpl) == "" {
		tmpl = getDefaultTemplate(alert.RuleType)
	}

	msg := tmpl
	msg = strings.ReplaceAll(msg, "${name}", name)
	msg = strings.ReplaceAll(msg, "${code}", data.Code)
	msg = strings.ReplaceAll(msg, "${alert_type}", alert.RuleType)
	msg = strings.ReplaceAll(msg, "${alert_subtype}", alert.SubType)
	msg = strings.ReplaceAll(msg, "${alert_label}", alert.Label)
	msg = strings.ReplaceAll(msg, "${price}", fmt.Sprintf("%.2f", data.Price))
	msg = strings.ReplaceAll(msg, "${change_pct}", fmt.Sprintf("%+.2f", data.ChangePct))
	msg = strings.ReplaceAll(msg, "${turnover}", fmt.Sprintf("%.1f", data.Turnover))
	msg = strings.ReplaceAll(msg, "${volume}", fmt.Sprintf("%d", data.Volume))
	msg = strings.ReplaceAll(msg, "${minutes}", fmt.Sprintf("%d", alert.Minutes))
	msg = strings.ReplaceAll(msg, "${amplitude}", fmt.Sprintf("%.1f", alert.Amplitude))
	msg = strings.ReplaceAll(msg, "${ratio}", fmt.Sprintf("%.1f", alert.VolumeRatio))
	msg = strings.ReplaceAll(msg, "${min_lots}", fmt.Sprintf("%d", alert.MinLots))

	return msg
}

// ============================================================================
//  PushToBots — 向指定机器人列表推送消息
// ============================================================================

// PushToBots 向指定机器人列表推送消息（复用 notifier.Notifier）
func PushToBots(ntf notifier.Notifier, bots []model.PushBot, message string) {
	if message == "" || len(bots) == 0 {
		return
	}

	for _, bot := range bots {
		// 只推送启用的机器人
		if bot.Status != 1 {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := ntf.Send(ctx, &bot, message); err != nil {
			log.Printf("[Monitor] 推送失败 [%s bots%d]: %v", bot.Channel, bot.ID, err)
		}
		cancel()
	}
}
