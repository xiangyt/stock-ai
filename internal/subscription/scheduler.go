package subscription

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"stock-ai/internal/model"

	"github.com/robfig/cron/v3"
)

// ============================================================================
//  Scheduler 调度引擎
// ============================================================================

// Scheduler 调度引擎接口
type Scheduler interface {
	// Start 启动调度引擎（加载活跃订阅 → 注册 cron job → 开始调度）
	Start() error

	// Stop 停止调度引擎（等待当前 job 完成 → 清理 cron）
	Stop()

	// Trigger 手动触发一次订阅执行
	Trigger(subscriptionID uint) error
}

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

// SubscriptionLoader 订阅加载接口（由 main.go 注入，解耦 scheduler 对 db 的直接依赖）
type SubscriptionLoader interface {
	// LoadActive 加载所有活跃订阅
	LoadActive() ([]SubscriptionLoadResult, error)

	// LoadByID 根据 ID 加载订阅
	LoadByID(id uint) (*SubscriptionLoadResult, error)
}

// schedulerImpl 调度引擎实现
type schedulerImpl struct {
	cron     *cron.Cron
	runner   *SubscriptionRunner
	mu       sync.Mutex
	entryMap map[uint]cron.EntryID // 订阅ID → cron job ID
}

// NewScheduler 创建调度引擎
func NewScheduler(runner *SubscriptionRunner) Scheduler {
	return &schedulerImpl{
		runner:   runner,
		entryMap: make(map[uint]cron.EntryID),
	}
}

// Start 启动调度引擎
func (s *schedulerImpl) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建 cron 实例（支持秒级精度，备用）
	s.cron = cron.New(cron.WithSeconds())

	// 加载所有活跃订阅
	subs, err := globalLoader.LoadActive()
	if err != nil {
		return fmt.Errorf("加载活跃订阅失败: %w", err)
	}

	// 注册 cron job
	registered := 0
	for i := range subs {
		sub := &subs[i]
		cronExpr := resolveCronExpr(model.PresetType(sub.PresetType), sub.CronExpr)
		if cronExpr == "" {
			log.Printf("[Scheduler] 订阅 %d(%s) cron 表达式为空，跳过", sub.ID, sub.Name)
			continue
		}

		// 校验 cron 表达式
		if _, parseErr := cron.ParseStandard(cronExpr); parseErr != nil {
			log.Printf("[Scheduler] 订阅 %d(%s) cron 表达式无效: %s，跳过", sub.ID, sub.Name, cronExpr)
			continue
		}

		subID := sub.ID
		entryID, addErr := s.cron.AddFunc(cronExpr, func() {
			s.runSubscription(subID)
		})
		if addErr != nil {
			log.Printf("[Scheduler] 注册订阅 %d(%s) cron 失败: %v", sub.ID, sub.Name, addErr)
			continue
		}

		s.entryMap[sub.ID] = entryID
		registered++
	}

	// 启动 cron
	s.cron.Start()
	log.Printf("[Scheduler] 启动成功，已注册 %d 个订阅 job", registered)

	return nil
}

// Stop 停止调度引擎
func (s *schedulerImpl) Stop() {
	s.mu.Lock()
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
		log.Println("[Scheduler] 已停止，所有 job 执行完毕")
	}
	s.mu.Unlock()
}

// Trigger 手动触发一次订阅执行
func (s *schedulerImpl) Trigger(subscriptionID uint) error {
	go s.runSubscription(subscriptionID)
	return nil
}

// runSubscription 执行单次订阅（cron job 回调）
func (s *schedulerImpl) runSubscription(subscriptionID uint) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Scheduler] 订阅 %d 执行 panic 已恢复: %v", subscriptionID, r)
		}
	}()

	// 从 DB 重新加载订阅配置（热更新）
	sub, err := globalLoader.LoadByID(subscriptionID)
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
		if !IsTradingDay() {
			log.Printf("[Scheduler] 今日非交易日，跳过订阅 %d", subscriptionID)
			return
		}
		if !IsTradingHours() {
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

// ============================================================================
//  辅助函数
// ============================================================================

// resolveCronExpr 根据 presetType 和 cronExpr 解析 cron 表达式
func resolveCronExpr(presetType model.PresetType, cronExpr string) string {
	if presetType == model.PresetCustom && cronExpr != "" {
		return cronExpr
	}
	return presetTypeToCron(presetType)
}

// presetTypeToCron 预设频率映射为 cron 表达式
func presetTypeToCron(preset model.PresetType) string {
	switch preset {
	case model.PresetEvery15min:
		return "*/15 * * * *"
	case model.PresetEvery30min:
		return "*/30 * * * *"
	case model.PresetEveryHour:
		return "0 * * * *"
	case model.PresetDailyOpen:
		return "15 9 * * 1-5"
	case model.PresetDailyClose:
		return "0 15 * * 1-5"
	case model.PresetDailyTwice:
		return "15 9,0 15 * * 1-5"
	case model.PresetWeekly:
		return "0 9 * * 1"
	case model.PresetCustom:
		return ""
	default:
		return "*/30 * * * *"
	}
}

// IsTradingDay 判断今天是否交易日（排除周末 + 法定节假日）
func IsTradingDay() bool {
	now := time.Now()
	weekday := now.Weekday()

	// 排除周末
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	// 排除 2025 年法定节假日
	holidays := get2025Holidays()
	todayStr := now.Format("2006-01-02")
	for _, h := range holidays {
		if h == todayStr {
			return false
		}
	}

	return true
}

// IsTradingHours 判断当前时间是否在 9:15-15:00
func IsTradingHours() bool {
	now := time.Now()
	hour := now.Hour()
	minute := now.Minute()

	totalMinutes := hour*60 + minute
	return totalMinutes >= 9*60+15 && totalMinutes <= 15*60
}

// get2025Holidays 2025 年 A 股法定节假日列表
func get2025Holidays() []string {
	return []string{
		// 元旦
		"2025-01-01",
		// 春节
		"2025-01-28", "2025-01-29", "2025-01-30", "2025-01-31",
		"2025-02-03", "2025-02-04",
		// 清明节
		"2025-04-04",
		// 劳动节
		"2025-05-01", "2025-05-02", "2025-05-05",
		// 端午节
		"2025-05-31", "2025-06-02",
		// 中秋节 + 国庆节
		"2025-10-01", "2025-10-02", "2025-10-03", "2025-10-06",
		"2025-10-07", "2025-10-08",
	}
}

// ============================================================================
//  全局 SubscriptionLoader（由 main.go 设置）
// ============================================================================

var globalLoader SubscriptionLoader

// SetSubscriptionLoader 设置全局订阅加载器
func SetSubscriptionLoader(loader SubscriptionLoader) {
	globalLoader = loader
}
