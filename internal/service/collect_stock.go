package service

import (
	"context"
	"fmt"
	"log"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// ========== 股票采集 ==========

// CollectStockList 执行股票列表采集（外部调用入口）
// 流程: 指定数据源 → 获取全量股票列表 → 遍历获取详情 → upsert入库
func (s *DataCollectService) CollectStockList(sourceName string) (*CollectResult, error) {
	ctx := context.Background()

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	log.Printf("[采集] 开始股票列表采集, source=%s", sourceName)

	allStocks, err := adp.GetStockList(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取股票列表失败: %w", err)
	}

	log.Printf("[采集] 股票列表获取完成, 共 %d 只", len(allStocks))

	result := &CollectResult{Total: len(allStocks)}
	for i, stock := range allStocks {
		code := stock.Code

		detail, detailErr := adp.GetStockDetail(ctx, code)
		if detailErr != nil {
			log.Printf("[采集] 获取详情失败 [%s]: %v", code, detailErr)
			continue
		}

		newStock := toModelStock(code, detail)
		rowsAffected := db.UpsertStock(newStock)

		if rowsAffected == 0 {
			result.NewCount++
		} else {
			result.UpdCount++
		}

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

	stock := toModelStock(code, detail)
	db.UpsertStock(stock)

	log.Printf("[采集] 详情采集完成 [%s]: %s", code, stock.Name)
	return &stock, nil
}

// ========== 股票辅助函数 ==========

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
