package scheduler

import (
	"testing"
	"time"

	"stock-ai/internal/model"

	"github.com/robfig/cron/v3"
)

// ============================================================================
//  resolveCronExpr / presetTypeToCrons 测试
// ============================================================================

func TestPresetTypeToCrons(t *testing.T) {
	tests := []struct {
		name     string
		preset   model.PresetType
		expected []string
	}{
		{"每15分钟", model.PresetEvery15min, []string{"10 */15 * * * *"}},
		{"每30分钟", model.PresetEvery30min, []string{"10 */30 * * * *"}},
		{"每小时", model.PresetEveryHour, []string{"10 0 * * * *"}},
		{"每日开盘", model.PresetDailyOpen, []string{"10 30 9 * * 1-5"}},
		{"每日收盘", model.PresetDailyClose, []string{"10 0 15 * * 1-5"}},
		{"每日两次", model.PresetDailyTwice, []string{"10 0 10,14 * * 1-5"}},
		{"中午12点", model.PresetNoon, []string{"10 0 12 * * 1-5"}},
		{"14:45收盘预警", model.PresetCloseAlert, []string{"10 45 14 * * 1-5"}},
		{"自定义返回nil", model.PresetCustom, nil},
		{"未知类型走默认", model.PresetType("unknown_type"), []string{"10 */30 * * * *"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := presetTypeToCrons(tt.preset)
			if len(got) != len(tt.expected) {
				t.Fatalf("长度不匹配: got %d, want %d (got=%v, want=%v)", len(got), len(tt.expected), got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("[%d] got %s, want %s", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestResolveCronExpr_CustomWithExpr(t *testing.T) {
	got := resolveCronExpr(model.PresetCustom, "*/5 * * * * *")
	if len(got) != 1 || got[0] != "*/5 * * * * *" {
		t.Errorf("自定义表达式未正确返回: got %v", got)
	}
}

func TestResolveCronExpr_CustomWithoutExpr(t *testing.T) {
	got := resolveCronExpr(model.PresetCustom, "")
	// 自定义但无表达式 → 走 presetTypeToCrons(custom) → 返回 nil
	if got != nil {
		t.Errorf("期望 nil, got %v", got)
	}
}

func TestResolveCronExpr_NonCustomIgnoresExpr(t *testing.T) {
	// 非自定义类型应忽略 cronExpr 参数，走预设映射
	got := resolveCronExpr(model.PresetEvery15min, "should_be_ignored")
	expected := []string{"10 */15 * * * *"}
	if len(got) != 1 || got[0] != expected[0] {
		t.Errorf("非自定义类型不应使用 cronExpr: got %v, want %v", got, expected)
	}
}

// ============================================================================
//  Scheduler 集成测试（真实 cron 实例，无 DB 依赖）
// ============================================================================

// 注意：runner.Run 签名是 func(ctx, *model.Subscription)，集成测试中我们直接构造
// schedulerImpl 来测试 add/remove，不依赖 runner 的具体行为。

func TestNewScheduler(t *testing.T) {
	// NewScheduler 不应返回 nil，且 changeCh 应该可用
	s := NewScheduler(nil)
	if s == nil {
		t.Fatal("NewScheduler 返回 nil")
	}

	// 检查接口满足性 — 应有 NotifyChange 方法且不 panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NotifyChange panic: %v", r)
		}
	}()
	s.NotifyChange(SubscriptionChange{Type: ChangeCreated, ID: 1})
}

func TestNotifyChange_NonBlocking(t *testing.T) {
	s := NewScheduler(nil).(*schedulerImpl)

	// 发送 1 个应该成功
	s.NotifyChange(SubscriptionChange{Type: ChangeCreated, ID: 1})
	if len(s.changeCh) != 1 {
		t.Errorf("channel 中应有 1 个事件, 实际 %d", len(s.changeCh))
	}

	// 填满 channel（cap=64），再发一个不应阻塞
	for i := 0; i < changeChannelSize+1; i++ {
		s.NotifyChange(SubscriptionChange{Type: ChangeCreated, ID: uint(i)})
	}
	// 如果到这里没卡住，说明非阻塞逻辑生效
}

func TestAddSubscription_InvalidCron(t *testing.T) {
	// 创建一个带真实 cron 的 schedulerImpl
	impl := &schedulerImpl{
		cron:     cron.New(cron.WithSeconds()),
		changeCh: make(chan SubscriptionChange, changeChannelSize),
		entryMap: make(map[uint][]cron.EntryID),
	}

	sub := &SubscriptionLoadResult{
		ID: 99, Name: "bad-cron",
		PresetType: string(model.PresetCustom),
		CronExpr:   "", // 自定义但无表达式 → resolveCronExpr 返回空
		IsActive:   true,
	}

	ok := impl.addSubscription(sub)
	if ok {
		t.Error("无效 cron 表达式不应注册成功")
	}

	if len(impl.entryMap) != 0 {
		t.Errorf("entryMap 应为空, 实际 %d 条", len(impl.entryMap))
	}
}

func TestAddAndRemoveSubscription(t *testing.T) {
	cr := cron.New(cron.WithSeconds())
	impl := &schedulerImpl{
		cron:     cr,
		changeCh: make(chan SubscriptionChange, changeChannelSize),
		entryMap: make(map[uint][]cron.EntryID),
	}

	sub := &SubscriptionLoadResult{
		ID: 100, Name: "test-add-remove",
		PresetType: string(model.PresetDailyOpen),
		IsActive:   true,
	}

	// 添加
	ok := impl.addSubscription(sub)
	if !ok {
		t.Fatal("添加订阅失败")
	}

	if _, exists := impl.entryMap[100]; !exists {
		t.Fatal("entryMap 中未找到订阅 100")
	}

	entries := impl.entryMap[100]
	if len(entries) != 1 {
		t.Errorf("daily_open 应产生 1 个 entry, 实际 %d", len(entries))
	}

	// 移除
	impl.removeSubscription(100)

	if _, exists := impl.entryMap[100]; exists {
		t.Error("移除后 entryMap 中仍存在订阅 100")
	}

	// 验证 cron 中的 entry 也被移除
	// cron.Remove 后通过 Entries() 检查
	for _, e := range cr.Entries() {
		for _, eid := range entries {
			if e.ID == eid {
				t.Error("cron 中仍存在已移除的 entry")
			}
		}
	}

	// 再次移除（幂等，不应 panic）
	impl.removeSubscription(100)
}

func TestAddSubscription_DailyTwice_MultipleEntries(t *testing.T) {
	cr := cron.New(cron.WithSeconds())
	impl := &schedulerImpl{
		cron:     cr,
		changeCh: make(chan SubscriptionChange, changeChannelSize),
		entryMap: make(map[uint][]cron.EntryID),
	}

	sub := &SubscriptionLoadResult{
		ID: 200, Name: "twice-daily",
		PresetType: string(model.PresetDailyTwice),
		IsActive:   true,
	}

	ok := impl.addSubscription(sub)
	if !ok {
		t.Fatal("添加 daily_twice 订阅失败")
	}

	entries, ok2 := impl.entryMap[200]
	if !ok2 || len(entries) != 1 {
		t.Errorf("daily_twice 应产生 1 个 entry（已合并为 10 0 10,14 * * 1-5）, 实际 %d, ok=%v", len(entries), ok2)
	}
}

func TestRemoveSubscription_NonExistent(t *testing.T) {
	cr := cron.New(cron.WithSeconds())
	impl := &schedulerImpl{
		cron:     cr,
		changeCh: make(chan SubscriptionChange, changeChannelSize),
		entryMap: make(map[uint][]cron.EntryID),
	}

	// 移除不存在的订阅 — 不应 panic
	impl.removeSubscription(999)
	if len(impl.entryMap) != 0 {
		t.Error("entryMap 应为空")
	}
}

// ============================================================================
//  SubscriptionChange 类型常量验证
// ============================================================================

func TestChangeTypeConstants(t *testing.T) {
	tests := []struct {
		ct   ChangeType
		want string
	}{
		{ChangeCreated, "created"},
		{ChangeUpdated, "updated"},
		{ChangeDeleted, "deleted"},
		{ChangeEnabled, "enabled"},
		{ChangeDisabled, "disabled"},
	}

	for _, tt := range tests {
		if string(tt.ct) != tt.want {
			t.Errorf("ChangeType 值错误: got %s, want %s", tt.ct, tt.want)
		}
	}
}

// ============================================================================
//  Cron 表达式可解析性验证（确保所有预设表达式能被 robfig/cron/v3 正确解析）
// ============================================================================

func TestAllPresetCronsAreValid(t *testing.T) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	allExprs := map[string][]string{
		"every_15min": {"10 */15 * * * *"},
		"every_30min": {"10 */30 * * * *"},
		"every_hour":  {"10 0 * * * *"},
		"daily_open":  {"10 30 9 * * 1-5"},
		"daily_close": {"10 0 15 * * 1-5"},
		"daily_twice": {"10 0 10,14 * * 1-5"},
		"noon":        {"10 0 12 * * 1-5"},
		"close_alert": {"10 45 14 * * 1-5"},
	}

	for name, exprs := range allExprs {
		for i, expr := range exprs {
			_, err := parser.Parse(expr)
			if err != nil {
				t.Errorf("%s[%d]: 表达式 %s 解析失败: %v", name, i, expr, err)
			}
		}
	}
}

// ============================================================================
//  Start/Stop 生命周期测试
// ============================================================================

func TestStartStop_Lifecycle(t *testing.T) {
	// 这个测试验证 Start/Stop 的基本生命周期
	// 由于 Start 依赖 DB（loadActive），我们无法在无 DB 环境下完整测试
	// 但可以验证 Stop 对空 scheduler 的安全性

	cr := cron.New(cron.WithSeconds())
	impl := &schedulerImpl{
		cron:     cr,
		runner:   nil, // 允许 nil 用于测试
		changeCh: make(chan SubscriptionChange, changeChannelSize),
		entryMap: make(map[uint][]cron.EntryID),
	}

	// Stop 未启动的 scheduler — 不应 panic
	done := make(chan bool)
	go func() {
		impl.Stop()
		done <- true
	}()

	select {
	case <-done:
		// 正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("Stop 超时，可能阻塞")
	}
}

// ============================================================================
//  并发安全测试
// ============================================================================

func TestConcurrentAddRemove(t *testing.T) {
	cr := cron.New(cron.WithSeconds())
	impl := &schedulerImpl{
		cron:     cr,
		changeCh: make(chan SubscriptionChange, changeChannelSize),
		entryMap: make(map[uint][]cron.EntryID),
	}

	// 并发添加不同的订阅
	done := make(chan bool)
	for i := 0; i < 20; i++ {
		go func(id uint) {
			sub := &SubscriptionLoadResult{
				ID: id, Name: "concurrent",
				PresetType: string(model.PresetEvery15min),
				IsActive:   true,
			}
			impl.addSubscription(sub)
			done <- true
		}(uint(i + 1))
	}

	// 等待所有添加完成
	for i := 0; i < 20; i++ {
		<-done
	}

	impl.mu.Lock()
	count := len(impl.entryMap)
	impl.mu.Unlock()

	if count == 0 {
		t.Error("并发添加后 entryMap 为空")
	}

	// 并发移除
	for i := 0; i < 20; i++ {
		go func(id uint) {
			impl.removeSubscription(id)
			done <- true
		}(uint(i + 1))
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	impl.mu.Lock()
	count = len(impl.entryMap)
	impl.mu.Unlock()

	if count != 0 {
		t.Errorf("并发移除后 entryMap 应为空, 实际 %d", count)
	}
}

// ============================================================================
//  边界情况
// ============================================================================

func TestResolveCronExpr_EmptyPresetType(t *testing.T) {
	// 空字符串 PresetType 应走 default 分支
	got := resolveCronExpr(model.PresetType(""), "")
	// 空字符串不是任何已知 preset，走 default → "10 */30 * * * *"
	if len(got) != 1 || got[0] != "10 */30 * * * *" {
		t.Errorf("空 PresetType 应返回默认值, got %v", got)
	}
}

func TestSubscriptionChange_Struct(t *testing.T) {
	sc := SubscriptionChange{Type: ChangeCreated, ID: 42}
	if sc.ID != 42 || sc.Type != ChangeCreated {
		t.Errorf("结构体字段错误: %+v", sc)
	}
}

func TestSubscriptionLoadResult_DefaultValues(t *testing.T) {
	r := SubscriptionLoadResult{}
	if r.IsActive {
		t.Error("默认 IsActive 应为 false")
	}
	if r.TradingHoursOnly {
		t.Error("默认 TradingHoursOnly 应为 false")
	}
	if r.ID != 0 {
		t.Error("默认 ID 应为 0")
	}
	if r.Name != "" {
		t.Error("默认 Name 应为空")
	}
}
