package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"stock-ai/internal/db"
	applog "stock-ai/internal/log"
	"stock-ai/internal/model"
	"stock-ai/internal/subscription/runner"
	"stock-ai/utils"

	"github.com/robfig/cron/v3"
)

// ============================================================================
//  订阅变更事件
// ============================================================================

// ChangeType 订阅变更事件类型
type ChangeType string

const (
	ChangeCreated  ChangeType = "created"
	ChangeUpdated  ChangeType = "updated"
	ChangeDeleted  ChangeType = "deleted"
	ChangeEnabled  ChangeType = "enabled"
	ChangeDisabled ChangeType = "disabled"
)

// SubscriptionChange 订阅变更事件
type SubscriptionChange struct {
	Type ChangeType // created/updated/deleted/enabled/disabled
	ID   uint       // 订阅 ID
}

// ============================================================================
//  Scheduler 调度引擎接口
// ============================================================================

// Scheduler 调度引擎接口
type Scheduler interface {
	// Start 启动调度引擎（加载活跃订阅 → 注册 cron job → 开始调度 → 启动变更监听）
	Start() error

	// Stop 停止调度引擎（关闭变更通道 → 等待当前 job 完成 → 清理 cron）
	Stop()

	// NotifyChange 接收订阅变更通知（由 Service 层调用）
	NotifyChange(change SubscriptionChange)
}

// ============================================================================
//  SubscriptionLoader 订阅加载接口
// ============================================================================

// ============================================================================
//  订阅加载（内置 DB 依赖）
// ============================================================================

// SubscriptionLoadResult 加载的订阅数据（用于调度引擎，与 model.Subscription 对齐）
type SubscriptionLoadResult struct {
	ID               uint
	UID              uint
	Name             string
	StrategyID       uint
	Scope            string
	CustomStocks     string
	PresetType       string
	CronExpr         string
	TradingHoursOnly bool
	IsActive         bool
	Template         string
}

// loadActive 加载所有活跃订阅
func loadActive() ([]SubscriptionLoadResult, error) {
	subs, err := db.GetActiveSubscriptions()
	if err != nil {
		return nil, err
	}
	return toLoadResults(subs), nil
}

// loadByID 根据 ID 加载订阅
func loadByID(id uint) (*SubscriptionLoadResult, error) {
	sub, err := db.GetSubscriptionByIDForScheduler(id)
	if err != nil {
		return nil, err
	}
	r := toLoadResults([]model.Subscription{*sub})
	return &r[0], nil
}

// toLoadResults 将 model.Subscription 批量转换为 SubscriptionLoadResult
func toLoadResults(subs []model.Subscription) []SubscriptionLoadResult {
	results := make([]SubscriptionLoadResult, len(subs))
	for i, sub := range subs {
		results[i] = SubscriptionLoadResult{
			ID:               sub.ID,
			UID:              sub.UID,
			Name:             sub.Name,
			StrategyID:       sub.StrategyID,
			Scope:            string(sub.Scope),
			CustomStocks:     sub.CustomStocks,
			PresetType:       string(sub.PresetType),
			CronExpr:         sub.CronExpr,
			TradingHoursOnly: sub.TradingHoursOnly,
			IsActive:         sub.IsActive,
			Template:         sub.Template,
		}
	}
	return results
}

// ============================================================================
//  Scheduler 实现
// ============================================================================

const changeChannelSize = 64

// schedulerImpl 调度引擎实现
type schedulerImpl struct {
	cron     *cron.Cron
	runner   *runner.SubscriptionRunner // 持有 Runner 单例引用
	changeCh chan SubscriptionChange    // 变更通知 channel
	mu       sync.Mutex
	entryMap map[uint][]cron.EntryID // 订阅ID → cron job IDs（一个订阅可能对应多个 entry）
}

// NewScheduler 创建调度引擎
// r: Runner 单例引用
func NewScheduler(r *runner.SubscriptionRunner) Scheduler {
	return &schedulerImpl{
		runner:   r,
		changeCh: make(chan SubscriptionChange, changeChannelSize),
		entryMap: make(map[uint][]cron.EntryID),
	}
}

// Start 启动调度引擎
func (s *schedulerImpl) Start() error {
	// 创建 cron 实例（秒级精度，6 段格式）
	s.cron = cron.New(cron.WithSeconds())

	// 加载所有活跃订阅
	subs, err := loadActive()
	if err != nil {
		return fmt.Errorf("加载活跃订阅失败: %w", err)
	}

	// 注册 cron job
	registered := 0
	for i := range subs {
		sub := &subs[i]
		if s.addSubscription(sub) {
			registered++
		}
	}

	// 启动 cron
	s.cron.Start()
	slog.Info("Scheduler 启动成功", "registered", registered)

	// 启动变更监听 goroutine
	go s.changeLoop()

	return nil
}

// Stop 停止调度引擎
func (s *schedulerImpl) Stop() {
	// 关闭变更通道（changeLoop 会退出）
	close(s.changeCh)

	s.mu.Lock()
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
		slog.Info("Scheduler 已停止，所有 job 执行完毕")
	}
	s.mu.Unlock()
}

// NotifyChange 接收订阅变更通知（非阻塞）
func (s *schedulerImpl) NotifyChange(change SubscriptionChange) {
	select {
	case s.changeCh <- change:
	default:
		slog.Warn("变更通道已满，丢弃事件", "type", change.Type, "id", change.ID)
	}
}

// changeLoop 消费变更事件，动态更新 cron 注册
func (s *schedulerImpl) changeLoop() {
	for change := range s.changeCh {
		slog.Info("收到变更事件", "type", change.Type, "id", change.ID)

		switch change.Type {
		case ChangeCreated, ChangeEnabled:
			// 从 DB 加载并注册
			sub, err := loadByID(change.ID)
			if err != nil {
				slog.Error("加载订阅失败", "id", change.ID, "error", err)
				continue
			}
			if sub.IsActive {
				s.addSubscription(sub)
			}

		case ChangeUpdated:
			// 先移除旧的，再根据当前状态决定是否重新注册
			s.removeSubscription(change.ID)

			sub, err := loadByID(change.ID)
			if err != nil {
				slog.Error("加载订阅失败", "id", change.ID, "error", err)
				continue
			}
			if sub.IsActive {
				s.addSubscription(sub)
			}

		case ChangeDeleted, ChangeDisabled:
			s.removeSubscription(change.ID)
		}
	}
}

// addSubscription 注册单个订阅到 cron
// 返回是否至少注册成功一个 cron job
func (s *schedulerImpl) addSubscription(sub *SubscriptionLoadResult) bool {
	cronExprs := resolveCronExpr(model.PresetType(sub.PresetType), sub.CronExpr)
	if len(cronExprs) == 0 {
		slog.Warn("订阅 cron 表达式为空，跳过", "sub_id", sub.ID, "name", sub.Name)
		return false
	}

	subID := sub.ID
	var entries []cron.EntryID

	for _, cronExpr := range cronExprs {
		entryID, addErr := s.cron.AddFunc(cronExpr, func() {
			s.runSubscription(subID)
		})
		if addErr != nil {
			slog.Error("注册订阅 cron 失败", "sub_id", sub.ID, "name", sub.Name, "error", addErr)
			continue
		}

		entries = append(entries, entryID)
		slog.Info("已注册订阅", "sub_id", sub.ID, "name", sub.Name, "cron", cronExpr)
	}

	if len(entries) == 0 {
		return false
	}

	s.mu.Lock()
	s.entryMap[sub.ID] = entries
	s.mu.Unlock()
	return true
}

// removeSubscription 从 cron 移除单个订阅的所有 job
func (s *schedulerImpl) removeSubscription(subscriptionID uint) {
	s.mu.Lock()
	entries, ok := s.entryMap[subscriptionID]
	delete(s.entryMap, subscriptionID)
	s.mu.Unlock()

	if !ok {
		return
	}

	for _, entryID := range entries {
		s.cron.Remove(entryID)
	}
	slog.Info("已移除订阅", "sub_id", subscriptionID, "jobs", len(entries))
}

// runSubscription 执行单次订阅（cron job 回调）
func (s *schedulerImpl) runSubscription(subscriptionID uint) {
	traceID := applog.NewTraceID()
	ctx, cancel := context.WithTimeout(applog.WithTraceID(context.Background(), traceID), 10*time.Minute)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("订阅执行 panic 已恢复", "trace_id", traceID, "sub_id", subscriptionID, "panic", r)
		}
	}()

	// 从 DB 重新加载订阅配置（热更新）
	sub, err := loadByID(subscriptionID)
	if err != nil {
		slog.Error("加载订阅失败", "trace_id", traceID, "sub_id", subscriptionID, "error", err)
		return
	}

	// 检查 is_active
	if !sub.IsActive {
		slog.Info("订阅已停用，跳过", "trace_id", traceID, "sub_id", subscriptionID)
		return
	}

	// 检查 trading_hours_only
	if sub.TradingHoursOnly {
		if !utils.IsTradingDay() {
			slog.Info("今日非交易日，跳过订阅", "trace_id", traceID, "sub_id", subscriptionID)
			return
		}
		if !utils.IsTradingHours() {
			slog.Info("当前非交易时段，跳过订阅", "trace_id", traceID, "sub_id", subscriptionID)
			return
		}
	}

	// 转换为 model.Subscription 供 runner 使用
	modelSub := toModelSubscription(sub)

	slog.Info("开始执行订阅", "trace_id", traceID, "sub_id", subscriptionID, "name", sub.Name)
	result, runErr := s.runner.Run(ctx, modelSub)
	if runErr != nil {
		slog.Error("订阅执行失败", "trace_id", traceID, "sub_id", subscriptionID, "name", sub.Name, "error", runErr)
		return
	}
	slog.Info("订阅执行完成", "trace_id", traceID, "sub_id", subscriptionID, "name", sub.Name,
		"scanned", result.TotalScanned, "matched", result.MatchCount, "elapsed_ms", result.DurationMs, "status", result.Status)
}

// ============================================================================
//  辅助函数
// ============================================================================

// toModelSubscription 将 SubscriptionLoadResult 转换为 model.Subscription
func toModelSubscription(r *SubscriptionLoadResult) *model.Subscription {
	return &model.Subscription{
		ID:               r.ID,
		UID:              r.UID,
		Name:             r.Name,
		StrategyID:       r.StrategyID,
		Scope:            model.MonitorScope(r.Scope),
		CustomStocks:     r.CustomStocks,
		PresetType:       model.PresetType(r.PresetType),
		CronExpr:         r.CronExpr,
		TradingHoursOnly: r.TradingHoursOnly,
		IsActive:         r.IsActive,
		Template:         r.Template,
	}
}

// resolveCronExpr 根据 presetType 和 cronExpr 解析 cron 表达式（返回多个）
func resolveCronExpr(presetType model.PresetType, cronExpr string) []string {
	if presetType == model.PresetCustom && cronExpr != "" {
		return []string{cronExpr}
	}
	return presetTypeToCrons(presetType)
}

// presetTypeToCrons 预设频率映射为 cron 表达式列表
func presetTypeToCrons(preset model.PresetType) []string {
	switch preset {
	case model.PresetEvery15min:
		return []string{"10 */15 * * * *"}
	case model.PresetEvery30min:
		return []string{"10 */30 * * * *"}
	case model.PresetEveryHour:
		return []string{"10 0 * * * *"}
	case model.PresetDailyOpen:
		return []string{"10 30 9 * * 1-5"}
	case model.PresetDailyClose:
		return []string{"10 0 15 * * 1-5"}
	case model.PresetDailyTwice:
		return []string{"10 0 10,14 * * 1-5"}
	case model.PresetNoon:
		return []string{"10 0 12 * * 1-5"}
	case model.PresetCloseAlert:
		return []string{"10 45 14 * * 1-5"}
	case model.PresetCustom:
		return nil
	default:
		return []string{"10 */30 * * * *"}
	}
}
