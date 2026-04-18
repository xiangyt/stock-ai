package service

import (
	"context"
	"fmt"
	"log"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	Total    int `json:"total"`     // 总条数
	NewCount int `json:"new_count"` // 新增数量
	UpdCount int `json:"upd_count"` // 更新数量
}

// ========== 采集执行 ==========

// CollectStockList 执行股票列表采集（外部调用入口）
// 流程: 指定数据源 → 获取全量股票列表 → 遍历获取详情 → upsert入库
func (s *DataCollectService) CollectStockList(sourceName string) (*CollectResult, error) {
	ctx := context.Background()

	// 1. 获取适配器
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	log.Printf("[采集] 开始股票列表采集, source=%s", sourceName)

	// 2. 获取全量股票列表
	allStocks, err := adp.GetStockList(ctx, func(current, total int, msg string) {
		log.Printf("[采集] 列表进度: %d/%d - %s", current, total, msg)
	})
	if err != nil {
		return nil, fmt.Errorf("获取股票列表失败: %w", err)
	}

	log.Printf("[采集] 股票列表获取完成, 共 %d 只", len(allStocks))

	// 3. 遍历每只股票获取详情并upsert入库
	result := &CollectResult{Total: len(allStocks)}
	for i, stock := range allStocks {
		code := stock.Code

		detail, detailErr := adp.GetStockDetail(ctx, code)
		if detailErr != nil {
			log.Printf("[采集] 获取详情失败 [%s]: %v", code, detailErr)
			continue
		}

		newStock := toModelStock(code, detail)
		rowsAffected := s.upsertStock(newStock)

		if rowsAffected == 0 {
			result.NewCount++
		} else {
			result.UpdCount++
		}

		// 每100只打印一次进度
		if (i+1)%100 == 0 || i == len(allStocks)-1 {
			log.Printf("[采集] 详情进度: %d/%d (新增%d, 更新%d)", i+1, len(allStocks), result.NewCount, result.UpdCount)
		}
	}

	log.Printf("[采集] 完成: total=%d, new=%d, upd=%d", result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// CollectStockDetail 采集单只股票详情（外部调用入口）
func (s *DataCollectService) CollectStockDetail(sourceName, code string) (*model.Stock, error) {
	ctx := context.Background()

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	detail, err := adp.GetStockDetail(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取股票详情失败: %w", err)
	}

	// upsert (原子操作，无并发问题)
	stock := toModelStock(code, detail)
	s.upsertStock(stock)

	log.Printf("[采集] 详情采集完成 [%s]: %s", code, stock.Name)
	return &stock, nil
}

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

// upsertStock 原子 upsert 操作 (INSERT ON DUPLICATE KEY UPDATE)
// 返回受影响的行数: 0=新增(INSERT), >0=更新(UPDATE)
func (s *DataCollectService) upsertStock(stock model.Stock) int64 {
	result := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "full_name", "english_name",
			"exchange", "exchange_name",
			"listing_board", "board_name",
			"list_date", "delist_date",
			"issue_price", "issue_pe", "issue_shares",
			"industry", "industry_code", "sector",
			"updated",
		}),
	}).Create(&stock)
	if result.Error != nil {
		log.Printf("[采集] upsert失败 [%s]: %v", stock.Code, result.Error)
		return -1
	}
	return result.RowsAffected
}

// toModelStock 将适配器的 StockBasic 转换为 GORM 模型 model.Stock
func toModelStock(code string, detail *adapter.StockBasic) model.Stock {
	if detail == nil {
		return model.Stock{Code: code}
	}
	return model.Stock{
		Code:         code,
		Name:         detail.Name,
		FullName:     detail.FullName,
		EnglishName:  detail.FullNameEn,
		Exchange:     detail.Exchange,
		ExchangeName: getExchangeName(detail.Exchange),
		ListingBoard: detail.ListingBoard,
		BoardName:    getBoardName(detail.ListingBoard),
		ListDate:     detail.ListDate,
		IssuePrice:   detail.IssuePrice,
		IssuePE:      detail.IssuePE,
		IssueShares:  detail.TotalIssueNum,
		Industry:     detail.Industry,
		Sector:       detail.Sector,
	}
}

// getExchangeName 获取交易所中文名
func getExchangeName(exchange string) string {
	switch exchange {
	case "SSE":
		return "上海证券交易所"
	case "SZSE":
		return "深圳证券交易所"
	case "BSE":
		return "北京证券交易所"
	default:
		return exchange
	}
}

// getBoardName 获取板块中文名
func getBoardName(board string) string {
	switch board {
	case "main":
		return "主板"
	case "chinext":
		return "创业板"
	case "star":
		return "科创板"
	case "bse":
		return "北交所"
	default:
		return board
	}
}
