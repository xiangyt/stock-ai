package service

import (
	"encoding/json"
	"fmt"
	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// SoftwareConfigService 用户软件配置服务
type SoftwareConfigService struct {
	registry *adapter.Registry
}

// NewSoftwareConfigService 创建软件配置服务
func NewSoftwareConfigService() *SoftwareConfigService {
	return &SoftwareConfigService{
		registry: adapter.GetRegistry(),
	}
}

// ListUserConfigs 列出当前用户的所有软件配置
func (s *SoftwareConfigService) ListUserConfigs(userID uint) ([]model.SoftwareConfigItem, error) {
	configs, err := db.GetSoftwareConfigsByUser(userID)
	if err != nil {
		return nil, err
	}

	items := make([]model.SoftwareConfigItem, 0, len(configs))
	for _, c := range configs {
		items = append(items, toItem(c))
	}
	return items, nil
}

// UpdateUserConfig 更新当前用户的某个软件配置
func (s *SoftwareConfigService) UpdateUserConfig(userID uint, name string, req *UpdateSoftwareConfigReq) (*model.SoftwareConfigItem, error) {
	meta, ok := SoftwareMetaMap[name]
	if !ok {
		return nil, fmt.Errorf("不支持的软件: %s", name)
	}

	cfg := &model.UserSoftwareConfig{
		UserID:       userID,
		SoftwareName: name,
		DisplayName:  meta.DisplayName,
		Cookie:       req.Cookie,
		Enabled:      req.Enabled,
	}
	extra, err := normalizeExtra(req.Extra)
	if err != nil {
		return nil, err
	}
	cfg.Extra = extra

	if err := db.UpsertSoftwareConfig(cfg); err != nil {
		return nil, err
	}

	// 应用配置到已注册的数据源（仅启用时）
	if cfg.Enabled {
		_ = s.applyToAdapter(name, cfg)
	}

	// 重新读取以获得最新时间戳
	latest, err := db.GetSoftwareConfig(userID, name)
	if err != nil {
		// 回退到内存对象
		latest = cfg
	}
	item := toItem(*latest)
	return &item, nil
}

// normalizeExtra 校验并规范化扩展配置
// 前端传空（nil 或空 map）时返回 "{}"；否则序列化为 JSON 并校验格式
func normalizeExtra(extra map[string]string) (string, error) {
	if len(extra) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(extra)
	if err != nil {
		return "", fmt.Errorf("扩展配置格式错误: %w", err)
	}
	// 防御性校验：确保是合法 JSON 对象
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", fmt.Errorf("扩展配置不是有效 JSON: %w", err)
	}
	return string(b), nil
}

// applyToAdapter 将用户配置应用到已注册的数据源
func (s *SoftwareConfigService) applyToAdapter(name string, cfg *model.UserSoftwareConfig) error {
	ds, ok := s.registry.Get(name)
	if !ok {
		return fmt.Errorf("数据源未注册: %s", name)
	}

	initConfig := map[string]interface{}{
		"cookie": cfg.Cookie,
	}
	if cfg.Extra != "" {
		var extra map[string]string
		if err := json.Unmarshal([]byte(cfg.Extra), &extra); err == nil {
			for k, v := range extra {
				initConfig[k] = v
			}
		}
	}
	return ds.Init(initConfig)
}

// UpdateSoftwareConfigReq 更新软件配置请求
type UpdateSoftwareConfigReq struct {
	Cookie  string            `json:"cookie"`
	Extra   map[string]string `json:"extra,omitempty"`
	Enabled bool              `json:"enabled"`
}

// SoftwareMeta 软件元信息
type SoftwareMeta struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// SoftwareMetaMap 支持的软件清单
var SoftwareMetaMap = map[string]SoftwareMeta{
	"eastmoney":    {Name: "eastmoney", DisplayName: "东方财富", Description: "Cookie 用于 K线/财务数据补全"},
	"ths":          {Name: "ths", DisplayName: "同花顺", Description: "Cookie 用于 K线全量采集"},
	"ths2":         {Name: "ths2", DisplayName: "同花顺 v2", Description: "Cookie 用于 i问财/token 接口"},
	"tencentstock": {Name: "tencentstock", DisplayName: "腾讯证券", Description: "无需 Cookie，可保留备用"},
}

// ListSupportedSoftware 返回支持的软件列表
func (s *SoftwareConfigService) ListSupportedSoftware() []SoftwareMeta {
	list := make([]SoftwareMeta, 0, len(SoftwareMetaMap))
	for _, m := range SoftwareMetaMap {
		list = append(list, m)
	}
	return list
}

func toItem(c model.UserSoftwareConfig) model.SoftwareConfigItem {
	return model.SoftwareConfigItem{
		SoftwareName: c.SoftwareName,
		DisplayName:  c.DisplayName,
		Cookie:       c.Cookie,
		Extra:        c.Extra,
		Enabled:      c.Enabled,
		UpdatedAt:    c.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// LoadAndApplyAdminConfigs 启动时从数据库加载 admin 用户配置并应用到已注册数据源
func (s *SoftwareConfigService) LoadAndApplyAdminConfigs() error {
	const adminUserID = 1
	configs, err := db.GetSoftwareConfigsByUser(adminUserID)
	if err != nil {
		return err
	}
	for _, cfg := range configs {
		if cfg.Enabled {
			_ = s.applyToAdapter(cfg.SoftwareName, &cfg)
		}
	}
	return nil
}
