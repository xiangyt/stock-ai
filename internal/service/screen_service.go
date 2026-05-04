package service

import (
	"stock-ai/internal/db"
	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ScreenService 负责为选股/回测引擎组装 indicator.StockData 列表。
type ScreenService struct {
}

func NewScreenService() *ScreenService {
	return &ScreenService{}
}

// BuildAll 并发获取全量 A 股并组装 StockData 列表。
func (s *ScreenService) BuildAll(maxConcurrency int) ([]*indicator.StockData, error) {
	stocks := db.LoadAllStockCodes()
	if len(stocks) == 0 {
		return nil, nil
	}

	result := make([]*indicator.StockData, len(stocks))
	err := utils.ConcurrentExec(stocks, maxConcurrency, func(i int, stock model.Stock) error {
		sd, err := s.buildOne(&stock)
		if err != nil {
			return err
		}
		result[i] = sd
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// buildOne 为单只股票调用 db 包组装 StockData（串行，由 BuildAll 并发调用）。
func (s *ScreenService) buildOne(stock *model.Stock) (*indicator.StockData, error) {
	code := stock.Code
	sd := &indicator.StockData{Detail: stock}
	var err error
	// ---------- DailyKline ----------
	sd.DailyKline, err = db.FindDailyKlines(code, 250)
	if err != nil {
		return nil, err
	}

	// // ---------- WeeklyKline ----------
	// sd.WeeklyKline, err = db.FindWeeklyKlines(code, 250)
	// if err != nil {
	// 	return nil, err
	// }

	// // ---------- MonthlyKline ----------
	// sd.MonthlyKline, err = db.FindMonthlyKlines(code, 250)
	// if err != nil {
	// 	return nil, err
	// }

	// // ---------- YearlyKline ----------
	// sd.YearlyKline, err = db.FindYearlyKlines(code, 250)
	// if err != nil {
	// 	return nil, err
	// }

	// ---------- PerformanceReport ----------
	sd.PerformanceReport, err = db.GetPerformanceReports(code, 8)
	if err != nil {
		return nil, err
	}

	// ---------- ShareholderCount ----------
	sc, err := db.FindLatestShareholderCount(code)
	if err == nil {
		sd.ShareholderCount = sc
	} else {
		return nil, err
	}

	// ---------- DailySnapshot（依赖 DailyKline[0].TradeDate） ----------
	if len(sd.DailyKline) > 0 {
		snap, err := db.FindSnapshotByStockAndDate(code, sd.DailyKline[0].TradeDate)
		if err == nil {
			sd.DailySnapshot = snap
		} else {
			return nil, err
		}
	}

	return sd, nil
}

// toPtrSlice 将值切片转换为指针切片（供 StockData 的 []*model.Xxx 字段使用）。
func toPtrSlice[T any](vs []T) []*T {
	ptrs := make([]*T, len(vs))
	for i := range vs {
		ptrs[i] = &vs[i]
	}
	return ptrs
}
