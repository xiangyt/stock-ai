package service

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// ========== K线采集 ==========

// CollectKLine 采集单只股票的K线数据
func (s *DataCollectService) CollectKLine(sourceName, code, klineType, adjType string) (*CollectResult, error) {
	ctx := context.Background()

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	if adjType == "" {
		adjType = adapter.AdjQFQ
	}

	klines, err := s.fetchKLines(ctx, adp, code, klineType, adjType)
	if err != nil {
		return nil, err
	}

	result := upsertKLines(code, klineType, klines)
	log.Printf("[采集-K线] 完成 [%s/%s]: total=%d, new=%d, upd=%d", code, klineType, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// CollectKLineBatch 全量采集所有股票的K线数据
// 流程: 从数据库获取全量股票列表 → 顺序执行每只股票的K线采集
func (s *DataCollectService) CollectKLineBatch(sourceName, klineType, adjType string) (*CollectResult, error) {
	ctx := context.Background()

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	if adjType == "" {
		adjType = adapter.AdjQFQ
	}

	stocks := s.loadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	log.Printf("[采集-K线] 开始全量%sk线采集, 共 %d 只股票, 数据源=%s", klineType, len(stocks), sourceName)

	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		klines, fetchErr := s.fetchKLines(ctx, adp, stock.Code, klineType, adjType)
		if fetchErr != nil {
			log.Printf("[采集-K线] 获取K线失败 [%s]: %v", stock.Code, fetchErr)
			result.FailCount++
			continue
		}

		partial := upsertKLines(stock.Code, klineType, klines)
		result.NewCount += partial.NewCount
		result.UpdCount += partial.UpdCount

		if (i+1)%100 == 0 || i == len(stocks)-1 {
			log.Printf("[采集-K线] 全量%s进度: %d/%d (新增%d, 更新%d)", klineType, i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}

	log.Printf("[采集-K线] 全量%s完成: total=%d, new=%d, upd=%d, fail=%d", klineType, result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// ========== K线辅助函数 ==========

// fetchKLines 根据周期调用对应的 adapter 方法获取K线数据
func (s *DataCollectService) fetchKLines(ctx context.Context, adp adapter.DataSource, code, klineType, adjType string) ([]adapter.StockPriceDaily, error) {
	switch klineType {
	case KLineDaily:
		return adp.GetDailyKLine(ctx, code, adjType)
	case KLineWeekly:
		return adp.GetWeeklyKLine(ctx, code, adjType)
	case KLineMonthly:
		return adp.GetMonthlyKLine(ctx, code, adjType)
	case KLineYearly:
		return adp.GetYearlyKLine(ctx, code, adjType)
	default:
		return nil, fmt.Errorf("不支持的K线周期: %s (支持: daily/weekly/monthly/yearly)", klineType)
	}
}

// upsertKLines 将K线数据批量写入对应周期的表（包级函数，供 batch 调用）
func upsertKLines(code string, klineType string, klines []adapter.StockPriceDaily) *CollectResult {
	result := &CollectResult{Total: len(klines)}
	if len(klines) == 0 {
		return result
	}

	for _, k := range klines {
		tradeDate := parseTradeDate(k.Date)

		var rowsAffected int64
		switch klineType {
		case KLineDaily:
			m := model.DailyKline{
				StockCode: code, TradeDate: tradeDate,
				Open: int(k.Open), High: int(k.High), Low: int(k.Low), Close: int(k.Close),
				Volume: k.Volume, Amount: k.Amount, TurnoverRate: k.Turnover,
			}
			rowsAffected = db.UpsertDailyKline(m)
		case KLineWeekly:
			m := model.WeeklyKline{
				StockCode: code, TradeDate: tradeDate,
				Open: int(k.Open), High: int(k.High), Low: int(k.Low), Close: int(k.Close),
				Volume: k.Volume, Amount: k.Amount, TurnoverRate: k.Turnover,
			}
			rowsAffected = db.UpsertWeeklyKline(m)
		case KLineMonthly:
			m := model.MonthlyKline{
				StockCode: code, TradeDate: tradeDate,
				Open: int(k.Open), High: int(k.High), Low: int(k.Low), Close: int(k.Close),
				Volume: k.Volume, Amount: k.Amount, TurnoverRate: k.Turnover,
			}
			rowsAffected = db.UpsertMonthlyKline(m)
		case KLineYearly:
			m := model.YearlyKline{
				StockCode: code, TradeDate: tradeDate,
				Open: int(k.Open), High: int(k.High), Low: int(k.Low), Close: int(k.Close),
				Volume: k.Volume, Amount: k.Amount, TurnoverRate: k.Turnover,
			}
			rowsAffected = db.UpsertYearlyKline(m)
		default:
			continue
		}

		if rowsAffected == 0 {
			result.NewCount++
		} else if rowsAffected > 0 {
			result.UpdCount++
		}
	}

	return result
}

// parseTradeDate 将 YYYY-MM-DD 格式日期转为 YYYYMMDD 整数
func parseTradeDate(dateStr string) int {
	if len(dateStr) >= 10 {
		if v, err := strconv.Atoi(dateStr[:4] + dateStr[5:7] + dateStr[8:10]); err == nil {
			return v
		}
	}
	return 0
}
