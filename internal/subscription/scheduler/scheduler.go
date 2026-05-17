package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"stock-ai/internal/db"
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
	s.mu.Lock()
	defer s.mu.Unlock()

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
	log.Printf("[Scheduler] 启动成功，已注册 %d 个订阅 job", registered)

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
		log.Println("[Scheduler] 已停止，所有 job 执行完毕")
	}
	s.mu.Unlock()
}

// NotifyChange 接收订阅变更通知（非阻塞）
func (s *schedulerImpl) NotifyChange(change SubscriptionChange) {
	select {
	case s.changeCh <- change:
	default:
		log.Printf("[Scheduler] 变更通道已满，丢弃事件: %s (id=%d)", change.Type, change.ID)
	}
}

// changeLoop 消费变更事件，动态更新 cron 注册
func (s *schedulerImpl) changeLoop() {
	for change := range s.changeCh {
		log.Printf("[Scheduler] 收到变更事件: %s (id=%d)", change.Type, change.ID)

		switch change.Type {
		case ChangeCreated, ChangeEnabled:
			// 从 DB 加载并注册
			sub, err := loadByID(change.ID)
			if err != nil {
				log.Printf("[Scheduler] 加载订阅 %d 失败: %v", change.ID, err)
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
				log.Printf("[Scheduler] 加载订阅 %d 失败: %v", change.ID, err)
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
		log.Printf("[Scheduler] 订阅 %d(%s) cron 表达式为空，跳过", sub.ID, sub.Name)
		return false
	}

	subID := sub.ID
	var entries []cron.EntryID

	for _, cronExpr := range cronExprs {
		entryID, addErr := s.cron.AddFunc(cronExpr, func() {
			s.runSubscription(subID)
		})
		if addErr != nil {
			log.Printf("[Scheduler] 注册订阅 %d(%s) cron 失败: %v", sub.ID, sub.Name, addErr)
			continue
		}

		entries = append(entries, entryID)
		log.Printf("[Scheduler] 已注册订阅 %d(%s), cron=%s", sub.ID, sub.Name, cronExpr)
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
	log.Printf("[Scheduler] 已移除订阅 %d（共 %d 个 job）", subscriptionID, len(entries))
}

// runSubscription 执行单次订阅（cron job 回调）
func (s *schedulerImpl) runSubscription(subscriptionID uint) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Scheduler] 订阅 %d 执行 panic 已恢复: %v", subscriptionID, r)
		}
	}()

	// 从 DB 重新加载订阅配置（热更新）
	sub, err := loadByID(subscriptionID)
	if err != nil {
		log.Printf("[Scheduler] 加载订阅 %d 失败: %v", subscriptionID, err)
		return
	}

	// 检查 is_active
	if !sub.IsActive {
		log.Printf("[Scheduler] 订阅 %d 已停用，跳过", subscriptionID)
		return
	}

	// 检查 trading_hours_only
	if sub.TradingHoursOnly {
		if !utils.IsTradingDay() {
			log.Printf("[Scheduler] 今日非交易日，跳过订阅 %d", subscriptionID)
			return
		}
		if !utils.IsTradingHours() {
			log.Printf("[Scheduler] 当前非交易时段，跳过订阅 %d", subscriptionID)
			return
		}
	}

	// 转换为 model.Subscription 供 runner 使用
	modelSub := toModelSubscription(sub)

	// 单次扫描总体超时 10 分钟
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	log.Printf("[Scheduler] 开始执行订阅 %d(%s)", subscriptionID, sub.Name)
	result, runErr := s.runner.Run(ctx, modelSub)
	if runErr != nil {
		log.Printf("[Scheduler] 订阅 %d 执行失败: %v", subscriptionID, runErr)
		return
	}
	log.Printf("[Scheduler] 订阅 %d 执行完成: 扫描 %d 只, 匹配 %d 只, 耗时 %dms, 状态 %s",
		subscriptionID, result.TotalScanned, result.MatchCount, result.DurationMs, result.Status)
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
		return []string{"0 */15 * * * *"}
	case model.PresetEvery30min:
		return []string{"0 */30 * * * *"}
	case model.PresetEveryHour:
		return []string{"0 0 * * * *"}
	case model.PresetDailyOpen:
		return []string{"0 15 9 * * 1-5"}
	case model.PresetDailyClose:
		return []string{"0 0 15 * * 1-5"}
	case model.PresetDailyTwice:
		return []string{"0 15 9 * * 1-5", "0 0 15 * * 1-5"}
	case model.PresetWeekly:
		return []string{"0 0 9 * * 1"}
	case model.PresetCustom:
		return nil
	default:
		return []string{"0 */30 * * * *"}
	}
}
