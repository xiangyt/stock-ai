package service

import (
	"context"
	"fmt"
	"log"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// ========== 基本面/财务面采集 ==========

// CollectPerformanceReports 采集单只股票的财报数据
func (s *DataCollectService) CollectPerformanceReports(sourceName, code string) (*CollectResult, error) {
	ctx := context.Background()
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	reports, err := adp.GetPerformanceReports(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取财报失败 [%s]: %w", code, err)
	}

	result := upsertPerformanceReports(code, reports)
	log.Printf("[采集-财报] 完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// CollectPerformanceReportsBatch 全量采集所有股票的财报
func (s *DataCollectService) CollectPerformanceReportsBatch(sourceName string) (*CollectResult, error) {
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	stocks := s.loadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	ctx := context.Background()
	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		reports, fetchErr := adp.GetPerformanceReports(ctx, stock.Code)
		if fetchErr != nil {
			log.Printf("[采集-财报] 获取失败 [%s]: %v", stock.Code, fetchErr)
			result.FailCount++
			continue
		}
		partial := upsertPerformanceReports(stock.Code, reports)
		result.NewCount += partial.NewCount
		result.UpdCount += partial.UpdCount

		if (i+1)%100 == 0 || i == len(stocks)-1 {
			log.Printf("[采集-财报] 全量进度: %d/%d (新增%d, 更新%d)", i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}
	log.Printf("[采集-财报] 全量完成: total=%d, new=%d, upd=%d, fail=%d", result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// CollectShareholderCounts 采集单只股票的股东户数
func (s *DataCollectService) CollectShareholderCounts(sourceName, code string) (*CollectResult, error) {
	ctx := context.Background()
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	counts, err := adp.GetShareholderCounts(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取股东户数失败 [%s]: %w", code, err)
	}

	result := upsertShareholderCounts(code, counts)
	log.Printf("[采集-股东户数] 完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// CollectShareholderCountsBatch 全量采集所有股票的股东户数
func (s *DataCollectService) CollectShareholderCountsBatch(sourceName string) (*CollectResult, error) {
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	stocks := s.loadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	ctx := context.Background()
	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		counts, fetchErr := adp.GetShareholderCounts(ctx, stock.Code)
		if fetchErr != nil {
			log.Printf("[采集-股东户数] 获取失败 [%s]: %v", stock.Code, fetchErr)
			result.FailCount++
			continue
		}
		partial := upsertShareholderCounts(stock.Code, counts)
		result.NewCount += partial.NewCount
		result.UpdCount += partial.UpdCount

		if (i+1)%100 == 0 || i == len(stocks)-1 {
			log.Printf("[采集-股东户数] 全量进度: %d/%d (新增%d, 更新%d)", i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}
	log.Printf("[采集-股东户数] 全量完成: total=%d, new=%d, upd=%d, fail=%d", result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// CollectShareChanges 采集单只股票的股本变动
func (s *DataCollectService) CollectShareChanges(sourceName, code string) (*CollectResult, error) {
	ctx := context.Background()
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	changes, err := adp.GetShareChanges(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取股本变动失败 [%s]: %w", code, err)
	}

	result := upsertShareChanges(code, changes)
	log.Printf("[采集-股本变动] 完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// CollectShareChangesBatch 全量采集所有股票的股本变动
func (s *DataCollectService) CollectShareChangesBatch(sourceName string) (*CollectResult, error) {
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	stocks := s.loadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	ctx := context.Background()
	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		changes, fetchErr := adp.GetShareChanges(ctx, stock.Code)
		if fetchErr != nil {
			log.Printf("[采集-股本变动] 获取失败 [%s]: %v", stock.Code, fetchErr)
			result.FailCount++
			continue
		}
		partial := upsertShareChanges(stock.Code, changes)
		result.NewCount += partial.NewCount
		result.UpdCount += partial.UpdCount

		if (i+1)%100 == 0 || i == len(stocks)-1 {
			log.Printf("[采集-股本变动] 全量进度: %d/%d (新增%d, 更新%d)", i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}
	log.Printf("[采集-股本变动] 全量完成: total=%d, new=%d, upd=%d, fail=%d", result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// ========== 基本面批量写入辅助函数（包级函数）==========

// upsertPerformanceReports 批量写入财报数据
func upsertPerformanceReports(code string, reports []adapter.PerformanceReport) *CollectResult {
	result := &CollectResult{Total: len(reports)}

	for _, r := range reports {
		m := model.PerformanceReport{
			StockCode:          code,
			ReportDate:         parseTradeDate(r.ReportDate),
			ReportType:         r.ReportType,
			ReportName:         r.ReportDateName,
			Currency:           r.Currency,
			BasicEPS:           r.BasicEPS,
			DeductedEPS:        r.DeductedEPS,
			DilutedEPS:         r.DilutedEPS,
			BVPS:               r.BVPS,
			EquityReserve:      r.EquityReservePerShare,
			UndistProfit:       r.UndistributedProfitPS,
			OCFPS:              r.OCFPS,
			TotalRevenue:       r.TotalRevenue,
			GrossProfit:        r.GrossProfit,
			ParentNetProfit:    r.ParentNetProfit,
			DeductNetProfit:    r.DeductNetProfit,
			RevenueYoY:         r.RevenueYoY,
			ParentNetProfitYoY: r.ParentNetProfitYoY,
			DeductNetProfitYoY: r.DeductNetProfitYoY,
			ROEW:               r.ROEW,
			ROEDW:              r.ROEDW,
			ROA:                r.ROA,
			GrossMargin:        r.GrossMargin,
			NetMargin:          r.NetMargin,
			CurrentRatio:       r.CurrentRatio,
			QuickRatio:         r.QuickRatio,
			DebtRatio:          r.DebtRatio,
			OCFToRevenue:       r.OCFToRevenue,
		}
		rowsAffected := db.UpsertPerformanceReport(m)
		if rowsAffected == -1 {
			continue
		}
		if rowsAffected == 0 {
			result.NewCount++
		} else {
			result.UpdCount++
		}
	}
	return result
}

// upsertShareholderCounts 批量写入股东户数数据
func upsertShareholderCounts(code string, counts []adapter.ShareholderCount) *CollectResult {
	result := &CollectResult{Total: len(counts)}

	for _, c := range counts {
		m := model.ShareholderCount{
			StockCode:           code,
			EndDate:             c.EndDate,
			SecurityName:        c.SecurityName,
			HolderNum:           c.HolderNum,
			HolderNumChangePct:  c.HolderNumChangePct,
			AvgFreeShares:       c.AvgFreeShares,
			AvgFreeSharesChgPct: c.AvgFreeSharesChangePct,
			HoldFocus:           c.HoldFocus,
			Price:               c.Price,
			AvgHoldAmount:       c.AvgHoldAmount,
			HoldRatioTotal:      c.HoldRatioTotal,
			FreeHoldRatioTotal:  c.FreeHoldRatioTotal,
		}
		rowsAffected := db.UpsertShareholderCount(m)
		if rowsAffected == -1 {
			continue
		}
		if rowsAffected == 0 {
			result.NewCount++
		} else {
			result.UpdCount++
		}
	}
	return result
}

// upsertShareChanges 批量写入股本变动数据
func upsertShareChanges(code string, changes []adapter.ShareChange) *CollectResult {
	result := &CollectResult{Total: len(changes)}

	for _, c := range changes {
		m := model.ShareChange{
			StockCode:       code,
			Date:            c.Date,
			ChangeReason:    c.ChangeReason,
			TotalShares:     c.TotalShares,
			LimitedShares:   c.LimitedShares,
			UnlimitedShares: c.UnlimitedShares,
			FloatAShares:    c.FloatAShares,
		}
		rowsAffected := db.UpsertShareChange(m)
		if rowsAffected == -1 {
			continue
		}
		if rowsAffected == 0 {
			result.NewCount++
		} else {
			result.UpdCount++
		}
	}
	return result
}
