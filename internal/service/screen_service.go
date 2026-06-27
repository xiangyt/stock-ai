package service

import (
	"fmt"

	"stock-ai/internal/db"
	"stock-ai/internal/backtest/indicator"
	stocksource "stock-ai/internal/backtest/indicator/stocksource"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ScreenService 负责为选股/回测引擎组装 indicator.StockSource 列表。
type ScreenService struct {
}

func NewScreenService() *ScreenService {
	return &ScreenService{}
}

// BuildAll 并发获取指定交易日的股票并组装 StockSource 列表。
// 每只股票只注入基本信息，各字段数据在指标访问时按需从 DB 加载（懒加载）。
// date 为空时自动从 daily_kline 取最近交易日；date 非空时会对齐到该日期之前最近的
// 实际交易日（处理前端传入周末/节假日的情况）。
func (s *ScreenService) BuildAll(maxConcurrency int, date string) ([]indicator.StockSource, error) {
	// 1. 确定交易日期
	var tradeDate int
	parsedDate, err := utils.ParseDateToTradeDate(date)
	if err != nil {
		// date 为空或格式错误，取全局最近交易日
		tradeDate, err = db.GetLatestDailyKlineDate()
		if err != nil {
			return nil, fmt.Errorf("获取最近交易日失败: %w", err)
		}
	} else {
		// date 有效，对齐到该日期之前最近的交易日（处理周末/节假日）
		tradeDate, err = db.FindNearestDailyKlineDate(parsedDate)
		if err != nil {
			return nil, fmt.Errorf("查找最近交易日失败: %w", err)
		}
	}
	if tradeDate == 0 {
		return nil, nil // daily_kline 表无数据
	}

	// 2. 只在指定交易日有日K数据的股票范围内选股
	stocks, err := db.LoadStockCodesByTradeDate(tradeDate)
	if err != nil {
		return nil, fmt.Errorf("加载股票列表失败: %w", err)
	}
	if len(stocks) == 0 {
		return nil, nil
	}

	// 3. 并发构建 StockSource
	result := make([]indicator.StockSource, len(stocks))
	err = utils.ConcurrentExec(stocks, maxConcurrency, func(i int, stock model.Stock) error {
		result[i] = stocksource.NewDBStock(&stock, tradeDate)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
