package tencentstock

import (
	"context"
	"testing"
)

// newTestAdapter 创建用于集成测试的适配器实例
func newTestAdapter(t *testing.T) *Adapter {
	t.Helper()
	a := New()
	if err := a.Init(nil); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	return a
}

func TestConnection(t *testing.T) {
	a := newTestAdapter(t)
	if err := a.TestConnection(context.Background()); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
}

func TestNameAndDisplayName(t *testing.T) {
	a := newTestAdapter(t)
	if a.Name() != AdapterName {
		t.Fatalf("Name 不匹配: want=%s got=%s", AdapterName, a.Name())
	}
	if a.DisplayName() == "" {
		t.Fatal("DisplayName 为空")
	}
	t.Logf("适配器: %s (%s)", a.Name(), a.DisplayName())
}
