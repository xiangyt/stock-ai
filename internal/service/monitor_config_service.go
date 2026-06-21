package service

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/subscription/monitor"
	"stock-ai/internal/subscription/watchlist"
)

// ============================================================================
//  MonitorConfigService 监控配置业务逻辑层
//
//  参照 SubscriptionService 模式：Service → notifyChange → Monitor
// ============================================================================

const maxMonitorConfigsPerUser = 10
const maxBotsPerMonitorConfig = 5

// MonitorConfigService 监控配置业务逻辑层
type MonitorConfigService struct {
	notifyChangeFn func(monitor.ChangeType, uint)
	watchlistMgr   *watchlist.Manager // 可选：为 nil 时不同步关注列表
}

// NewMonitorConfigService 创建监控配置服务
func NewMonitorConfigService() *MonitorConfigService {
	return &MonitorConfigService{}
}

// SetNotifyChange 设置变更通知回调（由 main.go 注入）
func (s *MonitorConfigService) SetNotifyChange(fn func(monitor.ChangeType, uint)) {
	s.notifyChangeFn = fn
}

// SetWatchlistManager 设置关注列表管理器（由 main.go 注入）
func (s *MonitorConfigService) SetWatchlistManager(mgr *watchlist.Manager) {
	s.watchlistMgr = mgr
}

// notifyChange 安全调用回调
func (s *MonitorConfigService) notifyChange(ct monitor.ChangeType, id uint) {
	if s.notifyChangeFn != nil {
		s.notifyChangeFn(ct, id)
	}
}

// notifyWatchlist 通知关注列表重新计算优先级
func (s *MonitorConfigService) notifyWatchlist(uid uint) {
	if s.watchlistMgr != nil {
		s.watchlistMgr.OnMonitorChanged(uid)
	}
}

// ============================================================================
//  请求/响应结构体
// ============================================================================

// CreateMonitorConfigReq 创建监控配置请求
type CreateMonitorConfigReq struct {
	Name   string                `json:"name" binding:"required"`
	Scope  string                `json:"scope"`
	Stocks []string              `json:"stocks"`
	Rule    model.MonitorRule   `json:"rule" binding:"required"`
	Cooldown    model.MonitorCooldown `json:"cooldown" binding:"required"`
	Template    string                `json:"template"`
	BotIDs      []uint                `json:"bot_ids"`
}

// UpdateMonitorConfigReq 更新监控配置请求
type UpdateMonitorConfigReq struct {
	Name     *string               `json:"name"`
	Scope    *string               `json:"scope"`
	Stocks   []string              `json:"stocks"`
	Rule     *model.MonitorRule   `json:"rule"`
	Cooldown *model.MonitorCooldown `json:"cooldown"`
	Template *string               `json:"template"`
	IsActive *bool                 `json:"is_active"`
}

// MonitorConfigDetail 监控配置详情响应
type MonitorConfigDetail struct {
	ID        uint                   `json:"id"`
	UID       uint                   `json:"uid"`
	Name      string                 `json:"name"`
	Scope     string                 `json:"scope"`
	Stocks    []string               `json:"stocks"`
	Rule      model.MonitorRule    `json:"rule"`
	Cooldown  model.MonitorCooldown  `json:"cooldown"`
	Template  string                 `json:"template"`
	IsActive  bool                   `json:"is_active"`
	Bots      []BotInfo              `json:"bots"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// ============================================================================
//  CRUD
// ============================================================================

// Create 创建监控配置
func (s *MonitorConfigService) Create(req *CreateMonitorConfigReq, uid uint) (*MonitorConfigDetail, error) {
	// 1. 校验数量上限
	count, err := db.CountUserMonitorConfigs(uid)
	if err != nil {
		return nil, fmt.Errorf("查询配置数量失败: %w", err)
	}
	if count >= maxMonitorConfigsPerUser {
		return nil, fmt.Errorf("监控配置数量已达上限(%d个)", maxMonitorConfigsPerUser)
	}

	// 2. 校验 scope
	scope := model.MonitorScope(req.Scope)
	if scope == "" {
		scope = model.ScopeHeld
	}
	if scope != model.ScopeHeld && scope != model.ScopeCustom {
		return nil, fmt.Errorf("无效的监控范围: %s (可选: held, custom)", scope)
	}

	// 3. scope=custom 时 Stocks 非空 + 6位数字校验
	var stocksJSON string
	if scope == model.ScopeCustom {
		if len(req.Stocks) == 0 {
			return nil, fmt.Errorf("自选模式下必须指定股票代码")
		}
		if err := validateStockCodes(req.Stocks); err != nil {
			return nil, err
		}
		b, _ := json.Marshal(req.Stocks)
		stocksJSON = string(b)
	}

	// 4. 校验 rule
	validTypes := map[model.RuleType]bool{
		model.RuleTypeDailyChange: true,
		model.RuleTypeRapidMove:   true,
		model.RuleTypeVolumeRatio: true,
		model.RuleTypeSealBoard:   true,
	}
	if !validTypes[req.Rule.Type] {
		return nil, fmt.Errorf("无效的规则类型: %s", req.Rule.Type)
	}
	ruleJSON, err := json.Marshal(req.Rule)
	if err != nil {
		return nil, fmt.Errorf("规则配置格式无效: %w", err)
	}

	// 5. 校验 cooldown
	if req.Cooldown.IntervalMinutes <= 0 {
		req.Cooldown.IntervalMinutes = 5
	}
	if req.Cooldown.DailyMax <= 0 {
		req.Cooldown.DailyMax = 3
	}
	cdJSON, err := json.Marshal(req.Cooldown)
	if err != nil {
		return nil, fmt.Errorf("冷却配置格式无效: %w", err)
	}

	// 6. 校验 bot_ids 数量
	if len(req.BotIDs) > maxBotsPerMonitorConfig {
		return nil, fmt.Errorf("每个监控配置最多关联%d个机器人", maxBotsPerMonitorConfig)
	}

	// 7. 创建配置
	cfg := &model.MonitorConfig{
		UID:      uid,
		Name:     strings.TrimSpace(req.Name),
		Scope:    scope,
		Stocks:   stocksJSON,
		Rules:    string(ruleJSON),
		Cooldown: string(cdJSON),
		Template: strings.TrimSpace(req.Template),
		IsActive: true,
	}
	if err := db.CreateMonitorConfig(cfg); err != nil {
		return nil, fmt.Errorf("创建监控配置失败: %w", err)
	}

	// 8. 设置机器人关联
	if len(req.BotIDs) > 0 {
		if err := db.SetMonitorConfigBots(cfg.ID, req.BotIDs); err != nil {
			log.Printf("[MonitorConfigService] 设置机器人关联失败 cfg=%d: %v", cfg.ID, err)
		}
	}

	// 9. 通知 Monitor
	s.notifyChange(monitor.ChangeCreated, cfg.ID)
	// 10. 同步到关注列表
	s.notifyWatchlist(uid)

	return s.buildDetail(cfg), nil
}

// Update 更新监控配置
func (s *MonitorConfigService) Update(id, uid uint, req *UpdateMonitorConfigReq) (*MonitorConfigDetail, error) {
	// 1. 查询已有配置
	cfg, err := db.GetMonitorConfigByID(id, uid)
	if err != nil {
		return nil, err
	}

	// 2. 按字段更新
	if req.Name != nil {
		cfg.Name = strings.TrimSpace(*req.Name)
	}
	if req.Scope != nil {
		scope := model.MonitorScope(*req.Scope)
		if scope != model.ScopeHeld && scope != model.ScopeCustom {
			return nil, fmt.Errorf("无效的监控范围: %s", scope)
		}
		cfg.Scope = scope
	}
	if req.Stocks != nil {
		if cfg.Scope == model.ScopeCustom {
			if err := validateStockCodes(req.Stocks); err != nil {
				return nil, err
			}
			b, _ := json.Marshal(req.Stocks)
			cfg.Stocks = string(b)
		}
	}
	if req.Rule != nil {
		b, _ := json.Marshal(*req.Rule)
		cfg.Rules = string(b)
	}
	if req.Cooldown != nil {
		if req.Cooldown.IntervalMinutes <= 0 {
			req.Cooldown.IntervalMinutes = 5
		}
		if req.Cooldown.DailyMax <= 0 {
			req.Cooldown.DailyMax = 3
		}
		b, _ := json.Marshal(req.Cooldown)
		cfg.Cooldown = string(b)
	}
	if req.Template != nil {
		cfg.Template = strings.TrimSpace(*req.Template)
	}
	if req.IsActive != nil {
		cfg.IsActive = *req.IsActive
	}

	// 3. 保存
	if err := db.UpdateMonitorConfig(cfg); err != nil {
		return nil, fmt.Errorf("更新监控配置失败: %w", err)
	}

	// 4. 通知 Monitor
	s.notifyChange(monitor.ChangeUpdated, cfg.ID)
	// 5. 同步到关注列表
	s.notifyWatchlist(uid)

	return s.buildDetail(cfg), nil
}

// Delete 软删除监控配置
func (s *MonitorConfigService) Delete(id, uid uint) error {
	if err := db.DeleteMonitorConfig(id, uid); err != nil {
		return err
	}
	s.notifyChange(monitor.ChangeDeleted, id)
	s.notifyWatchlist(uid)
	return nil
}

// SetActive 切换启用/停用
func (s *MonitorConfigService) SetActive(id, uid uint, active bool) error {
	if err := db.SetMonitorConfigActive(id, uid, active); err != nil {
		return err
	}
	if active {
		s.notifyChange(monitor.ChangeEnabled, id)
	} else {
		s.notifyChange(monitor.ChangeDisabled, id)
	}
	s.notifyWatchlist(uid)
	return nil
}

// ============================================================================
//  查询
// ============================================================================

// GetByID 获取配置详情
func (s *MonitorConfigService) GetByID(id, uid uint) (*MonitorConfigDetail, error) {
	cfg, err := db.GetMonitorConfigByID(id, uid)
	if err != nil {
		return nil, err
	}
	return s.buildDetail(cfg), nil
}

// List 分页查询配置列表
func (s *MonitorConfigService) List(uid uint, page, pageSize int) ([]MonitorConfigDetail, int64, error) {
	cfgs, total, err := db.ListMonitorConfigs(uid, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	details := make([]MonitorConfigDetail, len(cfgs))
	for i, cfg := range cfgs {
		details[i] = *s.buildDetail(&cfg)
	}
	return details, total, nil
}

// UpdateBots 更新配置关联的机器人
func (s *MonitorConfigService) UpdateBots(id, uid uint, botIDs []uint) error {
	// 校验归属
	_, err := db.GetMonitorConfigByID(id, uid)
	if err != nil {
		return err
	}

	if len(botIDs) > maxBotsPerMonitorConfig {
		return fmt.Errorf("每个监控配置最多关联%d个机器人", maxBotsPerMonitorConfig)
	}

	return db.SetMonitorConfigBots(id, botIDs)
}

// ============================================================================
//  辅助方法
// ============================================================================

var stockCodeRe = regexp.MustCompile(`^\d{6}$`)

// validateStockCodes 校验股票代码格式（6位数字）
func validateStockCodes(codes []string) error {
	for _, code := range codes {
		if !stockCodeRe.MatchString(strings.TrimSpace(code)) {
			return fmt.Errorf("无效的股票代码: %s (必须为6位纯数字)", code)
		}
	}
	return nil
}

// buildDetail 构建响应详情
func (s *MonitorConfigService) buildDetail(cfg *model.MonitorConfig) *MonitorConfigDetail {
	detail := &MonitorConfigDetail{
		ID:        cfg.ID,
		UID:       cfg.UID,
		Name:      cfg.Name,
		Scope:     string(cfg.Scope),
		Template:  cfg.Template,
		IsActive:  cfg.IsActive,
		CreatedAt: cfg.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: cfg.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	// 解析 Stocks
	if cfg.Stocks != "" {
		var stocks []string
		if json.Unmarshal([]byte(cfg.Stocks), &stocks) == nil {
			detail.Stocks = stocks
		}
	}

	// 解析 Rule
	rule, _ := cfg.ParseRule()
	if rule != nil {
		detail.Rule = *rule
	}

	// 解析 Cooldown
	cd, _ := cfg.ParseCooldown()
	if cd != nil {
		detail.Cooldown = *cd
	}

	// 加载关联机器人
	bots, err := db.GetMonitorConfigBots(cfg.ID)
	if err == nil {
		for _, b := range bots {
			detail.Bots = append(detail.Bots, BotInfo{
				ID:      b.ID,
				Name:    b.Name,
				Channel: b.Channel,
			})
		}
	}

	return detail
}

// BotInfo 机器人简要信息（与 subscription service 共用结构）
// 已在 subscription_service.go 中定义，此处复用
