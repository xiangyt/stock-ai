package monitor

import (
	"testing"
	"time"
)

func TestCooldown(t *testing.T) {
	cc := NewCooldownController()

	// 第一次应该允许
	if !cc.ShouldAlert(1, "000001", "surge_big", 5, 3) {
		t.Error("第一次告警应该允许")
	}

	// 同一 config + code + subType 在冷却期内应拒绝
	if cc.ShouldAlert(1, "000001", "surge_big", 5, 3) {
		t.Error("冷却期内不应允许重复告警")
	}

	// 不同的 subType 应该独立计数
	if !cc.ShouldAlert(1, "000001", "surge_small", 5, 3) {
		t.Error("不同 subType 应该允许")
	}

	// 不同的 config 应该独立计数
	if !cc.ShouldAlert(2, "000001", "surge_big", 5, 3) {
		t.Error("不同 configID 应该允许")
	}

	// 每日上限：同 subType 多次触发
	cc.ResetDaily()
	for i := 0; i < 10; i++ {
		key := "1:000001:limit_test"
		cc.mu.Lock()
		cc.lastAlert[key] = time.Time{} // 零时绕过时间冷却
		cc.mu.Unlock()
		if i < 3 {
			if !cc.ShouldAlert(1, "000001", "limit_test", 5, 3) {
				t.Errorf("前3次应该允许，第%d次被拒绝", i+1)
			}
		} else {
			if cc.ShouldAlert(1, "000001", "limit_test", 5, 3) {
				t.Errorf("第%d次应超每日上限被拒绝", i+1)
			}
		}
	}

	// ResetDaily 后应重置
	cc.ResetDaily()
	cc.mu.Lock()
	cc.lastAlert["1:000001:limit_test"] = time.Time{}
	cc.mu.Unlock()
	if !cc.ShouldAlert(1, "000001", "limit_test", 5, 3) {
		t.Error("ResetDaily 后应该允许")
	}
}
