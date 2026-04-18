package service

import (
	"fmt"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"gorm.io/gorm"
)

// ========== 数据采集服务 ==========

// DataCollectService 数据采集服务
type DataCollectService struct {
	db       *gorm.DB
	registry *adapter.Registry // 数据源注册中心
}

func NewDataCollectService() *DataCollectService {
	return &DataCollectService{
		db:       db.GetDB(),
		registry: adapter.GetRegistry(),
	}
}

// ========== 采集结果 ==========

// CollectResult 采集结果
type CollectResult struct {
	Total     int `json:"total"`      // 总条数
	NewCount  int `json:"new_count"`  // 新增数量
	UpdCount  int `json:"upd_count"`  // 更新数量
	FailCount int `json:"fail_count"` // 失败数量
}

// KLineCollectRequest K线采集请求参数
type KLineCollectRequest struct {
	Source    string `json:"source"`     // 数据源名称: eastmoney / ths
	KLineType string `json:"kline_type"` // 周期: daily / weekly / monthly / yearly
	AdjType   string `json:"adj_type"`   // 复权类型: qfq(前复权)/不复权/bqq(后复权), 默认 qfq
}

// KLinePeriod K线周期常量
const (
	KLineDaily   = "daily"
	KLineWeekly  = "weekly"
	KLineMonthly = "monthly"
	KLineYearly  = "yearly"
)

// ========== 内部辅助函数 ==========

// getAdapter 从注册中心获取指定名称的适配器
func (s *DataCollectService) getAdapter(preferredName string) (adapter.DataSource, error) {
	if preferredName != "" {
		adp, ok := s.registry.Get(preferredName)
		if ok {
			return adp, nil
		}
		return nil, fmt.Errorf("数据源未注册或不可用: %s", preferredName)
	}

	// 没指定则按优先级从数据库取可用源
	var configs []model.DataSourceConfig
	s.db.Where("status = ?", "active").Order("priority ASC").Find(&configs)

	for _, cfg := range configs {
		adp, ok := s.registry.Get(cfg.Name)
		if ok && cfg.IsActive() {
			return adp, nil
		}
	}

	// fallback: 注册中心的第一个可用源
	for _, name := range s.registry.Names() {
		adp, ok := s.registry.Get(name)
		if ok {
			return adp, nil
		}
	}

	return nil, fmt.Errorf("无可用数据源，请先在 config.yaml 中启用并注册数据源")
}

// loadAllStockCodes 从数据库加载全量股票代码列表
func (s *DataCollectService) loadAllStockCodes() []model.Stock {
	return db.LoadAllStockCodes()
}
