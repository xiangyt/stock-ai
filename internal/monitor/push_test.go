package monitor

import (
	"testing"

	"stock-ai/internal/model"
)

func TestPushBuild(t *testing.T) {
	pb := NewPushBuilder()

	cfg := &model.MonitorConfig{
		ID:       1,
		Name:     "测试监控",
		Template: "",
	}
	alert := Alert{
		RuleType:    string(model.RuleTypeDailyChange),
		SubType:     "surge_big",
		Label:       "大涨",
		ChangePct:   7.5,
		Minutes:     5,
		Amplitude:   3.0,
		VolumeRatio: 4.2,
		MinLots:     500,
	}
	data := makeQuoteData("000001", 7.5, 20000000)

	// 空模板 → 使用默认模板
	msg := pb.Build(cfg, alert, data)
	t.Logf("默认模板渲染: %s", msg)

	// 自定义模板
	cfg.Template = "[自定义] ${name}(${code}) 涨幅 ${change_pct}% 价格 ${price}"
	msg2 := pb.Build(cfg, alert, data)
	t.Logf("自定义模板渲染: %s", msg2)

	// nil data 不 panic
	msg3 := pb.Build(cfg, alert, nil)
	if msg3 != "" {
		t.Error("nil data 应返回空字符串")
	}
}
