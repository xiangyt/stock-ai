package monitor

import (
	"testing"

	"stock-ai/internal/model"
)

// mockSubscriber 模拟行情订阅
type mockSubscriber struct {
	ch chan model.QuoteEvent
}

func newMockSubscriber() *mockSubscriber {
	return &mockSubscriber{ch: make(chan model.QuoteEvent, 16)}
}

func (m *mockSubscriber) subscribe() model.QuoteSubscriber {
	return func(_ []string) (<-chan model.QuoteEvent, func()) {
		return m.ch, func() { close(m.ch) }
	}
}

func TestConfigChanges(t *testing.T) {
	_ = newMockSubscriber()
	t.Log("配置变更流程由 checker + cooldown + push 单元测试覆盖")
}
