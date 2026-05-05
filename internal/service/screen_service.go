package service

import (
	"stock-ai/internal/db"
	"stock-ai/internal/indicator"
	stocksource "stock-ai/internal/indicator/stocksource"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ScreenService 负责为选股/回测引擎组装 indicator.StockSource 列表。
type ScreenService struct {
}

func NewScreenService() *ScreenService {
	return &ScreenService{}
}

// BuildAll 并发获取全量 A 股并组装 StockSource 列表。
// 每只股票只注入基本信息，各字段数据在指标访问时按需从 DB 加载（懒加载）。
func (s *ScreenService) BuildAll(maxConcurrency int, date string) ([]indicator.StockSource, error) {
	stocks := db.LoadAllStockCodes()
	if len(stocks) == 0 {
		return nil, nil
	}

	tradeDate, _ := parseDateToTradeDate(date)
	result := make([]indicator.StockSource, len(stocks))
	err := utils.ConcurrentExec(stocks, maxConcurrency, func(i int, stock model.Stock) error {
		result[i] = stocksource.NewDBStock(&stock, tradeDate)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
