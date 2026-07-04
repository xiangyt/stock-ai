package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/subscription/runner"
	"stock-ai/internal/subscription/scheduler"
	"stock-ai/internal/subscription/watchlist"
)

// ============================================================================
//  SubscriptionService 订阅业务逻辑层
// ============================================================================

// maxUserSubscriptions 用户订阅上限
const maxUserSubscriptions = 20

// maxBotsPerSubscription 单订阅关联机器人上限
const maxBotsPerSubscription = 5

// SubscriptionService 订阅业务逻辑层
type SubscriptionService struct {
	notifyChangeFn func(scheduler.ChangeType, uint) // 通知 Scheduler 变更的回调
	runner         *runner.SubscriptionRunner
	watchlistMgr   *watchlist.Manager // 可选：为 nil 时不同步关注列表
}

// NewSubscriptionService 创建订阅服务
func NewSubscriptionService() *SubscriptionService {
	return &SubscriptionService{}
}

// SetNotifyChange 设置变更通知回调（由 main.go 注入）
func (s *SubscriptionService) SetNotifyChange(fn func(scheduler.ChangeType, uint)) {
	s.notifyChangeFn = fn
}

// SetRunner 设置执行器引用（由 main.go 注入）
func (s *SubscriptionService) SetRunner(r *runner.SubscriptionRunner) {
	s.runner = r
}

// SetWatchlistManager 设置关注列表管理器（由 main.go 注入）
func (s *SubscriptionService) SetWatchlistManager(mgr *watchlist.Manager) {
	s.watchlistMgr = mgr
}

// notifyWatchlist 通知关注列表重新计算优先级
func (s *SubscriptionService) notifyWatchlist(uid uint) {
	if s.watchlistMgr != nil {
		s.watchlistMgr.OnSubscriptionChanged(uid)
	}
}

// ============================================================================
//  请求/响应结构体
// ============================================================================

// CreateSubscriptionReq 创建订阅请求
type CreateSubscriptionReq struct {
	Name             string   `json:"name" binding:"required"`
	StrategyID       uint     `json:"strategy_id" binding:"required"`
	Scope            string   `json:"scope"`
	CustomStocks     []string `json:"custom_stocks"`
	PresetType       string   `json:"preset_type"`
	CronExpr         string   `json:"cron_expr"`
	TradingHoursOnly bool     `json:"trading_hours_only"`
	Template         string   `json:"template"`
	BotIDs           []uint   `json:"bot_ids"`
}

// UpdateSubscriptionReq 更新订阅请求
type UpdateSubscriptionReq struct {
	Name             string   `json:"name"`
	StrategyID       uint     `json:"strategy_id"`
	Scope            string   `json:"scope"`
	CustomStocks     []string `json:"custom_stocks"`
	PresetType       string   `json:"preset_type"`
	CronExpr         string   `json:"cron_expr"`
	TradingHoursOnly bool     `json:"trading_hours_only"`
	Template         string   `json:"template"`
}

// SubscriptionDetail 订阅详情响应
type SubscriptionDetail struct {
	ID               uint      `json:"id"`
	UID              uint      `json:"uid"`
	Name             string    `json:"name"`
	StrategyID       uint      `json:"strategy_id"`
	StrategyName     string    `json:"strategy_name"`
	Scope            string    `json:"scope"`
	CustomStocks     string    `json:"custom_stocks"`
	PresetType       string    `json:"preset_type"`
	CronExpr         string    `json:"cron_expr"`
	TradingHoursOnly bool      `json:"trading_hours_only"`
	IsActive         bool      `json:"is_active"`
	Template         string    `json:"template"`
	LastRunAt        string    `json:"last_run_at"`
	Bots             []BotInfo `json:"bots"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
}

// SubscriptionListItem 订阅列表项
type SubscriptionListItem struct {
	ID               uint      `json:"id"`
	Name             string    `json:"name"`
	StrategyID       uint      `json:"strategy_id"`
	StrategyName     string    `json:"strategy_name"`
	Scope            string    `json:"scope"`
	PresetType       string    `json:"preset_type"`
	CronExpr         string    `json:"cron_expr"`
	TradingHoursOnly bool      `json:"trading_hours_only"`
	IsActive         bool      `json:"is_active"`
	LastRunAt        string    `json:"last_run_at"`
	Bots             []BotInfo `json:"bots"`
	BotCount         int       `json:"bot_count"`
	CreatedAt        string    `json:"created_at"`
}

// SubscriptionListResp 订阅列表响应
type SubscriptionListResp struct {
	List  []SubscriptionListItem `json:"list"`
	Total int64                  `json:"total"`
}

// BotInfo 机器人信息
type BotInfo struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Channel string `json:"channel"`
}

// LogListResp 执行日志列表响应
type LogListResp struct {
	List  []SubscriptionLogItem `json:"list"`
	Total int64                 `json:"total"`
}

// SubscriptionLogItem 执行日志列表项
type SubscriptionLogItem struct {
	ID           uint   `json:"id"`
	RunTime      string `json:"run_time"`
	FinishedAt   string `json:"finished_at"`
	Scope        string `json:"scope"`
	TotalScanned int    `json:"total_scanned"`
	MatchCount   int    `json:"match_count"`
	MatchStocks  string `json:"match_stocks"`
	DurationMs   int    `json:"duration_ms"`
	Status       string `json:"status"`
	ErrorMsg     string `json:"error_msg"`
}

// TriggerResult 手动触发结果
type TriggerResult struct {
	Message string `json:"message"`
}

// ============================================================================
//  CRUD 方法
// ============================================================================

// Create 创建订阅
func (s *SubscriptionService) Create(req *CreateSubscriptionReq, uid uint) (*SubscriptionDetail, error) {
	// 校验 strategy 归属
	strategy, err := db.GetStrategyByID(req.StrategyID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, fmt.Errorf("策略不存在")
		}
		return nil, fmt.Errorf("查询策略失败: %w", err)
	}
	if strategy.UID != uid {
		return nil, fmt.Errorf("无权使用该策略")
	}

	// 校验订阅上限
	count, err := db.CountUserSubscriptions(uid)
	if err != nil {
		return nil, fmt.Errorf("查询订阅数量失败: %w", err)
	}
	if count >= maxUserSubscriptions {
		return nil, fmt.Errorf("订阅数量已达上限 %d 个", maxUserSubscriptions)
	}

	// 校验 scope
	scope := model.MonitorScope(req.Scope)
	if scope == "" {
		scope = model.ScopeAll
	}
	if scope != model.ScopeAll && scope != model.ScopeHeld && scope != model.ScopeCustom {
		return nil, fmt.Errorf("scope 必须是 all / held / custom 之一")
	}

	// 校验 custom_stocks
	var customStocksJSON string
	if scope == model.ScopeCustom {
		if len(req.CustomStocks) == 0 {
			return nil, fmt.Errorf("scope=custom 时 custom_stocks 不能为空")
		}
		for _, code := range req.CustomStocks {
			if !isValidStockCode(code) {
				return nil, fmt.Errorf("股票代码 %s 格式无效，必须为 6 位纯数字", code)
			}
		}
		bytes, _ := json.Marshal(req.CustomStocks)
		customStocksJSON = string(bytes)
	}

	// 校验预设频率
	presetType := model.PresetType(req.PresetType)
	if presetType == "" {
		presetType = model.PresetEvery30min
	}
	if !isValidPresetType(presetType) {
		return nil, fmt.Errorf("preset_type 无效")
	}

	// 校验 cron 表达式（preset=custom 时必须提供）
	if presetType == model.PresetCustom {
		if req.CronExpr == "" {
			return nil, fmt.Errorf("preset_type=custom 时 cron_expr 不能为空")
		}
		if !isValidCronExpr(req.CronExpr) {
			return nil, fmt.Errorf("cron 表达式无效: %s", req.CronExpr)
		}
	}

	// 构建订阅
	sub := &model.Subscription{
		UID:              uid,
		Name:             req.Name,
		StrategyID:       req.StrategyID,
		Scope:            scope,
		CustomStocks:     customStocksJSON,
		PresetType:       presetType,
		CronExpr:         req.CronExpr,
		TradingHoursOnly: req.TradingHoursOnly,
		IsActive:         true,
		Template:         req.Template,
	}

	if err := db.CreateSubscription(sub); err != nil {
		return nil, fmt.Errorf("创建订阅失败: %w", err)
	}

	// 设置关联机器人
	if len(req.BotIDs) > 0 {
		if err := db.SetSubscriptionBots(sub.ID, req.BotIDs); err != nil {
			return nil, fmt.Errorf("设置关联机器人失败: %w", err)
		}
	}

	// 通知 Scheduler 注册新订阅
	s.notifyChange(scheduler.ChangeCreated, sub.ID)
	s.notifyWatchlist(uid)

	// 返回详情
	return s.GetByID(sub.ID, uid)
}

// GetByID 获取订阅详情
func (s *SubscriptionService) GetByID(id, uid uint) (*SubscriptionDetail, error) {
	sub, err := db.GetSubscriptionByID(id, uid)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, fmt.Errorf("订阅不存在或无权操作")
		}
		return nil, fmt.Errorf("查询订阅失败: %w", err)
	}

	// 加载策略名称
	strategy, _ := db.GetStrategyByID(sub.StrategyID)
	strategyName := ""
	if strategy != nil {
		strategyName = strategy.Name
	}

	// 加载关联机器人
	bots, _ := db.GetSubscriptionBots(sub.ID)
	botInfos := make([]BotInfo, 0, len(bots))
	for _, bot := range bots {
		botInfos = append(botInfos, BotInfo{
			ID:      bot.ID,
			Name:    bot.Name,
			Channel: bot.Channel,
		})
	}

	detail := &SubscriptionDetail{
		ID:               sub.ID,
		UID:              sub.UID,
		Name:             sub.Name,
		StrategyID:       sub.StrategyID,
		StrategyName:     strategyName,
		Scope:            string(sub.Scope),
		CustomStocks:     sub.CustomStocks,
		PresetType:       string(sub.PresetType),
		CronExpr:         sub.CronExpr,
		TradingHoursOnly: sub.TradingHoursOnly,
		IsActive:         sub.IsActive,
		Template:         sub.Template,
		Bots:             botInfos,
		CreatedAt:        sub.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        sub.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	if sub.LastRunAt != nil {
		detail.LastRunAt = sub.LastRunAt.Format("2006-01-02 15:04:05")
	}

	return detail, nil
}

// List 查询订阅列表
func (s *SubscriptionService) List(uid uint, page, pageSize int, isActive *bool) (*SubscriptionListResp, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	subs, total, err := db.ListSubscriptions(uid, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("查询订阅列表失败: %w", err)
	}

	// 构建 strategy 名称缓存
	strategyNames := make(map[uint]string)
	for _, sub := range subs {
		if _, ok := strategyNames[sub.StrategyID]; !ok {
			strategy, _ := db.GetStrategyByID(sub.StrategyID)
			if strategy != nil {
				strategyNames[sub.StrategyID] = strategy.Name
			}
		}
	}

	items := make([]SubscriptionListItem, 0, len(subs))
	for _, sub := range subs {
		// 过滤 is_active
		if isActive != nil && sub.IsActive != *isActive {
			continue
		}

		bots, _ := db.GetSubscriptionBots(sub.ID)
		botInfos := make([]BotInfo, 0, len(bots))
		for _, b := range bots {
			botInfos = append(botInfos, BotInfo{
				ID:      b.ID,
				Name:    b.Name,
				Channel: string(b.Channel),
			})
		}
		item := SubscriptionListItem{
			ID:               sub.ID,
			Name:             sub.Name,
			StrategyID:       sub.StrategyID,
			StrategyName:     strategyNames[sub.StrategyID],
			Scope:            string(sub.Scope),
			PresetType:       string(sub.PresetType),
			CronExpr:         sub.CronExpr,
			TradingHoursOnly: sub.TradingHoursOnly,
			IsActive:         sub.IsActive,
			Bots:             botInfos,
			BotCount:         len(bots),
			CreatedAt:        sub.CreatedAt.Format("2006-01-02 15:04:05"),
		}

		if sub.LastRunAt != nil {
			item.LastRunAt = sub.LastRunAt.Format("2006-01-02 15:04:05")
		}

		items = append(items, item)
	}

	return &SubscriptionListResp{
		List:  items,
		Total: total,
	}, nil
}

// Update 更新订阅
func (s *SubscriptionService) Update(id, uid uint, req *UpdateSubscriptionReq) (*SubscriptionDetail, error) {
	sub, err := db.GetSubscriptionByID(id, uid)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, fmt.Errorf("订阅不存在或无权操作")
		}
		return nil, fmt.Errorf("查询订阅失败: %w", err)
	}

	// 更新字段
	if req.Name != "" {
		sub.Name = req.Name
	}
	if req.StrategyID != 0 {
		strategy, err := db.GetStrategyByID(req.StrategyID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return nil, fmt.Errorf("策略不存在")
			}
			return nil, fmt.Errorf("查询策略失败: %w", err)
		}
		if strategy.UID != uid {
			return nil, fmt.Errorf("无权使用该策略")
		}
		sub.StrategyID = req.StrategyID
	}
	if req.Scope != "" {
		scope := model.MonitorScope(req.Scope)
		if scope != model.ScopeAll && scope != model.ScopeHeld && scope != model.ScopeCustom {
			return nil, fmt.Errorf("scope 必须是 all / held / custom 之一")
		}
		sub.Scope = scope
	}
	if req.CustomStocks != nil {
		for _, code := range req.CustomStocks {
			if !isValidStockCode(code) {
				return nil, fmt.Errorf("股票代码 %s 格式无效", code)
			}
		}
		bytes, _ := json.Marshal(req.CustomStocks)
		sub.CustomStocks = string(bytes)
	}
	if req.PresetType != "" {
		presetType := model.PresetType(req.PresetType)
		if !isValidPresetType(presetType) {
			return nil, fmt.Errorf("preset_type 无效")
		}
		sub.PresetType = presetType
	}
	if req.CronExpr != "" {
		if sub.PresetType == model.PresetCustom {
			if !isValidCronExpr(req.CronExpr) {
				return nil, fmt.Errorf("cron 表达式无效")
			}
		}
		sub.CronExpr = req.CronExpr
	}
	sub.TradingHoursOnly = req.TradingHoursOnly
	if req.Template != "" {
		sub.Template = req.Template
	}

	if err := db.UpdateSubscription(sub); err != nil {
		return nil, fmt.Errorf("更新订阅失败: %w", err)
	}

	// 通知 Scheduler 重新加载
	s.notifyChange(scheduler.ChangeUpdated, id)
	s.notifyWatchlist(uid)

	return s.GetByID(id, uid)
}

// Delete 删除订阅
func (s *SubscriptionService) Delete(id, uid uint) error {
	_, err := db.GetSubscriptionByID(id, uid)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("订阅不存在或无权操作")
		}
		return fmt.Errorf("查询订阅失败: %w", err)
	}

	if err := db.DeleteSubscription(id, uid); err != nil {
		return fmt.Errorf("删除订阅失败: %w", err)
	}

	// 通知 Scheduler 移除 cron job
	s.notifyChange(scheduler.ChangeDeleted, id)
	s.notifyWatchlist(uid)

	return nil
}

// SetActive 切换订阅启停状态
func (s *SubscriptionService) SetActive(id, uid uint, active bool) error {
	_, err := db.GetSubscriptionByID(id, uid)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("订阅不存在或无权操作")
		}
		return fmt.Errorf("查询订阅失败: %w", err)
	}

	if err := db.SetActive(id, uid, active); err != nil {
		return fmt.Errorf("切换状态失败: %w", err)
	}

	// 通知 Scheduler
	if active {
		s.notifyChange(scheduler.ChangeEnabled, id)
	} else {
		s.notifyChange(scheduler.ChangeDisabled, id)
	}
	s.notifyWatchlist(uid)

	return nil
}

// UpdateBots 更新订阅关联的机器人
func (s *SubscriptionService) UpdateBots(id, uid uint, botIDs []uint) error {
	sub, err := db.GetSubscriptionByID(id, uid)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("订阅不存在或无权操作")
		}
		return fmt.Errorf("查询订阅失败: %w", err)
	}

	_ = sub // 确认归属

	// 校验上限
	if len(botIDs) > maxBotsPerSubscription {
		return fmt.Errorf("关联机器人数量不能超过 %d 个", maxBotsPerSubscription)
	}

	// 校验每个 bot 归属当前用户
	for _, botID := range botIDs {
		bot, err := db.GetPushBotByID(botID, uid)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return fmt.Errorf("机器人 %d 不存在或无权操作", botID)
			}
			return fmt.Errorf("查询机器人 %d 失败: %w", botID, err)
		}
		if bot.Status != 1 {
			return fmt.Errorf("机器人 %d(%s) 已禁用", botID, bot.Name)
		}
	}

	return db.SetSubscriptionBots(id, botIDs)
}

// GetLogs 获取订阅执行日志
func (s *SubscriptionService) GetLogs(id, uid uint, page, pageSize int, status string) (*LogListResp, error) {
	// 校验订阅归属
	_, err := db.GetSubscriptionByID(id, uid)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, fmt.Errorf("订阅不存在或无权操作")
		}
		return nil, fmt.Errorf("查询订阅失败: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	logs, total, err := db.ListSubscriptionLogs(id, page, pageSize, status)
	if err != nil {
		return nil, fmt.Errorf("查询执行日志失败: %w", err)
	}

	items := make([]SubscriptionLogItem, 0, len(logs))
	for _, l := range logs {
		item := SubscriptionLogItem{
			ID:           l.ID,
			RunTime:      l.RunTime.Format("2006-01-02 15:04:05"),
			Scope:        string(l.Scope),
			TotalScanned: l.TotalScanned,
			MatchCount:   l.MatchCount,
			MatchStocks:  l.MatchStocks,
			DurationMs:   l.DurationMs,
			Status:       string(l.Status),
			ErrorMsg:     l.ErrorMsg,
		}
		if l.FinishedAt != nil {
			item.FinishedAt = l.FinishedAt.Format("2006-01-02 15:04:05")
		}
		items = append(items, item)
	}

	return &LogListResp{
		List:  items,
		Total: total,
	}, nil
}

// TriggerRun 手动触发订阅执行（直接调用 Runner，不经过 Scheduler）
func (s *SubscriptionService) TriggerRun(id, uid uint, isAdmin bool) (*TriggerResult, error) {
	var sub *model.Subscription
	var err error
	if isAdmin {
		sub, err = db.GetSubscriptionByIDForScheduler(id)
	} else {
		sub, err = db.GetSubscriptionByID(id, uid)
	}
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, fmt.Errorf("订阅不存在或无权操作")
		}
		return nil, fmt.Errorf("查询订阅失败: %w", err)
	}

	if !sub.IsActive {
		return nil, fmt.Errorf("订阅已停用，请先启用")
	}

	if s.runner == nil {
		return nil, fmt.Errorf("执行器未初始化")
	}

	// 异步执行，立即返回
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		result, err := s.runner.Run(ctx, sub)
		if err != nil {
			log.Printf("[TriggerRun] 订阅 %d 执行失败: %v", id, err)
			return
		}
		log.Printf("[TriggerRun] 订阅 %d 执行完成: 扫描 %d, 匹配 %d, 耗时 %dms, 状态 %s",
			id, result.TotalScanned, result.MatchCount, result.DurationMs, result.Status)
	}()

	return &TriggerResult{
		Message: "已触发执行",
	}, nil
}

// ============================================================================
//  辅助方法
// ============================================================================

// notifyChange 安全地通知 Scheduler（回调为 nil 时不报错）
func (s *SubscriptionService) notifyChange(changeType scheduler.ChangeType, id uint) {
	if s.notifyChangeFn != nil {
		s.notifyChangeFn(changeType, id)
	}
}

// isValidStockCode 校验股票代码（6 位纯数字）
var stockCodeRegex = regexp.MustCompile(`^\d{6}$`)

func isValidStockCode(code string) bool {
	return stockCodeRegex.MatchString(code)
}

// isValidPresetType 校验预设频率类型
func isValidPresetType(preset model.PresetType) bool {
	switch preset {
	case model.PresetEvery15min, model.PresetEvery30min, model.PresetEveryHour,
		model.PresetDailyOpen, model.PresetDailyClose, model.PresetDailyTwice,
		model.PresetNoon, model.PresetCloseAlert, model.PresetCustom:
		return true
	default:
		return false
	}
}

// isValidCronExpr 校验 cron 表达式
func isValidCronExpr(expr string) bool {
	fields := strings.Fields(expr)
	// 标准 cron 5 段 或 6 段（含秒）
	if len(fields) != 5 && len(fields) != 6 {
		return false
	}
	// 简单校验：至少包含数字和合法字符
	for _, field := range fields {
		if field == "" {
			return false
		}
		if !isValidCronField(field) {
			return false
		}
	}
	return true
}

// isValidCronField 校验单个 cron 字段
func isValidCronField(field string) bool {
	validChars := "0123456789*/,-"
	for _, ch := range field {
		if !strings.ContainsRune(validChars, ch) {
			return false
		}
	}
	return true
}

// parsePageAndSize 解析分页参数
func parsePageAndSize(pageStr, sizeStr string) (int, int) {
	page, _ := strconv.Atoi(pageStr)
	size, _ := strconv.Atoi(sizeStr)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}
