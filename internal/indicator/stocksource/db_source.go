// Package stocksource 提供股票数据源的接口定义和实现。
//
// 设计思路:
//   - StockSource 是总接口，由 4 个分类子接口组合而成
//   - 各子接口对应一种指标分类所需的数据源:
//     TechnicalSource → 技术面指标 (K线序列)
//     MarketSource    → 行情面指标 (实时行情快照)
//     FundamentalSource → 基本面指标 (公司基本信息)
//     FinancialSource → 财务面指标 (财报/股本等)
//   - 实现类可自由组合子接口，灵活支持 DB/内存/网络等不同数据源
package stocksource

import (
	"stock-ai/internal/db"
	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
)

// ============================================================================
//  DBStock — StockSource 的 DB 实现（懒加载）
//
//  各字段在首次 Get 时检测缓存是否为空，为空则自动查 DB 加载。
//  不缓存 error，每次返回实时 DB 错误。
//  由 ScreenService 在 BuildAll 时构造，只需传入基本信息。
// ============================================================================

// --- 编译期接口约束 ---

var _ indicator.StockSource = (*DBStock)(nil)

type DBStock struct {
	code      string
	tradeDate int
	detail    *model.Stock

	// 缓存字段（首次 Get 时按需加载，只缓存值）
	dailyKlineVal   []*model.DailyKline
	weeklyKlineVal  []*model.WeeklyKline
	monthlyKlineVal []*model.MonthlyKline
	yearlyKlineVal  []*model.YearlyKline
	perfReportVal   []*model.PerformanceReport
	shareholderVal  *model.ShareholderCount
	snapshotVal     *model.StockDailySnapshot
}

// NewDBStock 构造一个 DB 懒加载 StockData。
// detail 提供 Code/Name/ListingBoard 等基本信息（立即可用，无需加载）。
func NewDBStock(detail *model.Stock, tradeDate int) *DBStock {
	return &DBStock{
		code:      detail.Code,
		detail:    detail,
		tradeDate: tradeDate,
	}
}

// NewDBStockByCode 构造一个 DB 懒加载 StockData。
func NewDBStockByCode(code string, tradeDate int) *DBStock {
	return &DBStock{
		code:      code,
		tradeDate: tradeDate,
	}
}

// --- 公共方法 ---

func (s *DBStock) GetCode() string { return s.code }
func (s *DBStock) GetName() string {
	detail, _ := s.GetDetail()
	if detail == nil {
		return ""
	}
	return detail.Name
}
func (s *DBStock) GetDetail() (*model.Stock, error) {
	detail, err := db.FindStockByCode(s.code)
	if err != nil {
		return nil, indicator.ErrDatabase
	}
	s.detail = &detail
	return s.detail, nil
}

// --- TechnicalSource ---

func (s *DBStock) GetDailyKline() ([]*model.DailyKline, error) {
	if s.dailyKlineVal == nil {
		var err error
		s.dailyKlineVal, err = db.FindDailyKlines(s.code, s.tradeDate, 250)
		if err != nil {
			return nil, indicator.ErrDatabase
		} else if len(s.dailyKlineVal) == 0 {
			return nil, indicator.ErrDataEmpty
		}
	}
	return s.dailyKlineVal, nil
}

func (s *DBStock) GetWeeklyKline() ([]*model.WeeklyKline, error) {
	if s.weeklyKlineVal == nil {
		var err error
		s.weeklyKlineVal, err = db.FindWeeklyKlines(s.code, s.tradeDate, 250)
		if err != nil {
			return nil, indicator.ErrDatabase
		} else if len(s.weeklyKlineVal) == 0 {
			return nil, indicator.ErrDataEmpty
		}
	}
	return s.weeklyKlineVal, nil
}

func (s *DBStock) GetMonthlyKline() ([]*model.MonthlyKline, error) {
	if s.monthlyKlineVal == nil {
		var err error
		s.monthlyKlineVal, err = db.FindMonthlyKlines(s.code, s.tradeDate, 250)
		if err != nil {
			return nil, indicator.ErrDatabase
		} else if len(s.monthlyKlineVal) == 0 {
			return nil, indicator.ErrDataEmpty
		}
	}
	return s.monthlyKlineVal, nil
}

func (s *DBStock) GetYearlyKline() ([]*model.YearlyKline, error) {
	if s.yearlyKlineVal == nil {
		var err error
		s.yearlyKlineVal, err = db.FindYearlyKlines(s.code, s.tradeDate, 250)
		if err != nil {
			return nil, indicator.ErrDatabase
		} else if len(s.yearlyKlineVal) == 0 {
			return nil, indicator.ErrDataEmpty
		}
	}
	return s.yearlyKlineVal, nil
}

// --- FinancialSource ---

func (s *DBStock) GetPerformanceReport() ([]*model.PerformanceReport, error) {
	if s.perfReportVal == nil {
		var err error
		s.perfReportVal, err = db.GetPerformanceReports(s.code, s.tradeDate, 8)
		if err != nil {
			return nil, indicator.ErrDatabase
		} else if len(s.perfReportVal) == 0 {
			return nil, indicator.ErrDataEmpty
		}
	}
	return s.perfReportVal, nil
}

func (s *DBStock) GetShareholderCount() (*model.ShareholderCount, error) {
	if s.shareholderVal == nil {
		var err error
		s.shareholderVal, err = db.FindLatestShareholderCount(s.code, s.tradeDate)
		if err != nil {
			return nil, indicator.ErrDatabase
		} else if s.shareholderVal == nil {
			return nil, indicator.ErrDataEmpty
		}
	}
	return s.shareholderVal, nil
}

func (s *DBStock) GetDailySnapshot() (*model.StockDailySnapshot, error) {
	if s.snapshotVal == nil {
		var err error
		// DailySnapshot 依赖最新交易日，先取 DailyKline 获取日期
		klines, err := s.GetDailyKline()
		if err != nil {
			return nil, err
		}
		if len(klines) > 0 {
			s.snapshotVal, err = db.FindSnapshotByStockAndDate(s.code, klines[0].TradeDate)
			if err != nil {
				return nil, indicator.ErrDatabase
			} else if s.snapshotVal == nil {
				return nil, indicator.ErrDataEmpty
			}

		}
	}
	return s.snapshotVal, nil
}
