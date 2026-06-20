package datacollect

import (
	"context"
	"fmt"
	"log"
	"strings"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// ============================================================================
//  基本信息采集结果（财报/股东户数/股本变动共用）
// ============================================================================

// CollectResult 批量采集结果汇总
type CollectResult struct {
	Total        int `json:"total"`          // 总股票数/总条数
	NewCount     int `json:"new_count"`      // 新增记录数
	UpdCount     int `json:"upd_count"`      // 更新记录数
	NameUpdCount int `json:"name_upd_count"` // UpdateStockName 成功数（仅名称变更采集使用）
	FailCount    int `json:"fail_count"`     // 失败股票数
}

// ============================================================================
//  单只股票采集入口（供 HTTP Handler 使用）
// ============================================================================

// RunPerformanceReports 采集单只股票的财报
func RunPerformanceReports(ctx context.Context, adp adapter.DataSource, code string) (*CollectResult, error) {
	reports, err := adp.GetPerformanceReports(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取财报失败 [%s]: %w", code, err)
	}
	result := upsertPerformanceReports(code, reports)
	log.Printf("[采集-财报] 完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// RunShareholderCounts 采集单只股票的股东户数
func RunShareholderCounts(ctx context.Context, adp adapter.DataSource, code string) (*CollectResult, error) {
	counts, err := adp.GetShareholderCounts(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取股东户数失败 [%s]: %w", code, err)
	}
	result := upsertShareholderCounts(code, counts)
	log.Printf("[采集-股东户数] 完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// RunShareChanges 采集单只股票的股本变动
func RunShareChanges(ctx context.Context, adp adapter.DataSource, code string) (*CollectResult, error) {
	changes, err := adp.GetShareChanges(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取股本变动失败 [%s]: %w", code, err)
	}
	result := upsertShareChanges(code, changes)
	log.Printf("[采集-股本变动] 完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// RunDividendHistory 采集单只股票的分红历史
func RunDividendHistory(ctx context.Context, adp adapter.DataSource, code string) (*CollectResult, error) {
	dividends, err := adp.GetDividendHistory(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取分红历史失败 [%s]: %w", code, err)
	}
	result := upsertDividendHistory(code, dividends)
	log.Printf("[采集-分红] 完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// ============================================================================
//  全量批量采集入口（供 Scheduler 和 HTTP Handler 共用）
// ============================================================================

// RunPerformanceReportsBatch 全量采集所有股票的财报
func RunPerformanceReportsBatch(ctx context.Context, adp adapter.DataSource) (*CollectResult, error) {
	stocks := db.LoadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
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
			log.Printf("[采集-财报] 全量进度: %d/%d (新增=%d, 更新=%d)", i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}
	log.Printf("[采集-财报] 全量完成: total=%d, new=%d, upd=%d, fail=%d", result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// RunShareholderCountsBatch 全量采集所有股票的股东户数
func RunShareholderCountsBatch(ctx context.Context, adp adapter.DataSource) (*CollectResult, error) {
	stocks := db.LoadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
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
			log.Printf("[采集-股东户数] 全量进度: %d/%d (新增=%d, 更新=%d)", i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}
	log.Printf("[采集-股东户数] 全量完成: total=%d, new=%d, upd=%d, fail=%d", result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// RunShareChangesBatch 全量采集所有股票的股本变动
func RunShareChangesBatch(ctx context.Context, adp adapter.DataSource) (*CollectResult, error) {
	stocks := db.LoadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
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
			log.Printf("[采集-股本变动] 全量进度: %d/%d (新增=%d, 更新=%d)", i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}
	log.Printf("[采集-股本变动] 全量完成: total=%d, new=%d, upd=%d, fail=%d", result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// RunDividendHistoryBatch 全量采集所有股票的分红历史
func RunDividendHistoryBatch(ctx context.Context, adp adapter.DataSource) (*CollectResult, error) {
	stocks := db.LoadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		dividends, fetchErr := adp.GetDividendHistory(ctx, stock.Code)
		if fetchErr != nil {
			log.Printf("[采集-分红] 获取失败 [%s]: %v", stock.Code, fetchErr)
			result.FailCount++
			continue
		}
		partial := upsertDividendHistory(stock.Code, dividends)
		result.NewCount += partial.NewCount
		result.UpdCount += partial.UpdCount
		if (i+1)%100 == 0 || i == len(stocks)-1 {
			log.Printf("[采集-分红] 全量进度: %d/%d (新增=%d, 更新=%d)", i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}
	log.Printf("[采集-分红] 全量完成: total=%d, new=%d, upd=%d, fail=%d", result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// RunNameChanges 采集单只股票的名称变更（增量：仅拉取 DB 最新记录之后的数据）
func RunNameChanges(ctx context.Context, adp adapter.DataSource, code string) (*CollectResult, error) {
	since := buildSince(code)
	changes, err := adp.GetNameChanges(ctx, code, since)
	if err != nil {
		return nil, fmt.Errorf("获取名称变更失败 [%s]: %w", code, err)
	}
	if len(changes) == 0 {
		return &CollectResult{Total: 0}, nil
	}
	result := upsertNameChanges(code, changes)
	// 同步最新名称到 stocks 表，同时更新退市日期
	// API 返回时间倒序（最新在前），取 changes[0]
	latest := changes[0]
	latestName := extractNewName(latest.SecurityName)
	delistDate := ""
	if strings.Contains(latestName, "退") && strings.Contains(latest.ChangeReason, "进入退市整理") {
		delistDate = latest.ChangeDate // 已是 YYYY-MM-DD 格式
	}
	_ = db.UpdateStockName(code, latestName, delistDate)
	log.Printf("[采集-名称变更] 完成 [%s]: total=%d, new=%d, upd=%d", code, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// RunNameChangesBatch 全量采集所有股票的名称变更（增量模式）
func RunNameChangesBatch(ctx context.Context, adp adapter.DataSource) (*CollectResult, error) {
	stocks := db.LoadAllStockCodes()
	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		since := buildSince(stock.Code)
		changes, fetchErr := adp.GetNameChanges(ctx, stock.Code, since)
		if fetchErr != nil {
			log.Printf("[采集-名称变更] 获取失败 [%s]: %v", stock.Code, fetchErr)
			result.FailCount++
			continue
		}
		if len(changes) == 0 {
			continue // 无新数据，跳过
		}
		partial := upsertNameChanges(stock.Code, changes)
		result.NewCount += partial.NewCount
		result.UpdCount += partial.UpdCount
		result.FailCount += partial.FailCount // 合并保存变更记录的失败数

		// 同步最新名称到 stocks 表，同时更新退市日期
		// API 返回时间倒序（最新在前），取 changes[0]
		latest := changes[0]
		latestName := extractNewName(latest.SecurityName)
		delistDate := ""
		if strings.Contains(latestName, "退") && strings.Contains(latest.ChangeReason, "进入退市整理") {
			delistDate = latest.ChangeDate // 已是 YYYY-MM-DD 格式
		}
		if err := db.UpdateStockName(stock.Code, latestName, delistDate); err != nil {
			log.Printf("[采集-名称变更] 更新股票名称失败 [%s]: %v", stock.Code, err)
			result.FailCount++
		} else {
			result.NameUpdCount++
		}

		if (i+1)%100 == 0 || i == len(stocks)-1 {
			log.Printf("[采集-名称变更] 全量进度: %d/%d (新增记录=%d, 名称更新=%d, 失败=%d)",
				i+1, len(stocks), result.NewCount, result.NameUpdCount, result.FailCount)
		}
	}
	log.Printf("[采集-名称变更] 全量完成: total=%d, 新增记录=%d, 名称更新=%d, 失败=%d",
		result.Total, result.NewCount, result.NameUpdCount, result.FailCount)
	return result, nil
}

// extractNewName 从名称变更字段中提取新名称
// 例: "*ST立方 → 立方退" → "立方退"
// 若无 "→" 分隔符，原样返回
func extractNewName(raw string) string {
	if idx := strings.Index(raw, "→"); idx >= 0 {
		return strings.TrimSpace(raw[idx+len("→"):])
	}
	return strings.TrimSpace(raw)
}

// buildSince 查询 DB 中该股票最近一次名称变更日期，返回 "YYYY-MM-DD" 格式；无记录时返回空串（全量拉取）
func buildSince(code string) string {
	nc, err := db.FindLatestNameChange(code)
	if err != nil || nc.ChangeDate == 0 {
		return ""
	}
	d := nc.ChangeDate // int YYYYMMDD
	return fmt.Sprintf("%d-%02d-%02d", d/10000, (d%10000)/100, d%100)
}

// ============================================================================
//  批量写入辅助函数
// ============================================================================

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
			NoticeDate:         parseTradeDate(r.NoticeDate),
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
			RevenueYoY:         clampDecimal(r.RevenueYoY, 10, 4),
			ParentNetProfitYoY: clampDecimal(r.ParentNetProfitYoY, 10, 4),
			DeductNetProfitYoY: clampDecimal(r.DeductNetProfitYoY, 10, 4),
			ROEW:               clampDecimal(r.ROEW, 10, 4),
			ROEDW:              clampDecimal(r.ROEDW, 10, 4),
			ROA:                clampDecimal(r.ROA, 10, 4),
			GrossMargin:        clampDecimal(r.GrossMargin, 10, 4),
			NetMargin:          clampDecimal(r.NetMargin, 10, 4),
			CurrentRatio:       clampDecimal(r.CurrentRatio, 10, 4),
			QuickRatio:         clampDecimal(r.QuickRatio, 10, 4),
			DebtRatio:          clampDecimal(r.DebtRatio, 10, 4),
			OCFToRevenue:       clampDecimal(r.OCFToRevenue, 10, 4),
		}
		rowsAffected := db.UpsertPerformanceReport(m)
		if rowsAffected == -1 {
			continue
		}
		if rowsAffected == 1 {
			result.NewCount++ // INSERT (新记录)
		} else {
			result.UpdCount++ // UPDATE (2=有变化, 0=无变化)
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
			EndDate:             parseTradeDate(c.EndDate),
			SecurityName:        c.SecurityName,
			HolderNum:           c.HolderNum,
			HolderNumChangePct:  clampDecimal(c.HolderNumChangePct, 10, 4),
			AvgFreeShares:       c.AvgFreeShares,
			AvgFreeSharesChgPct: clampDecimal(c.AvgFreeSharesChangePct, 10, 4),
			HoldFocus:           c.HoldFocus,
			Price:               clampDecimal(c.Price, 10, 4),
			AvgHoldAmount:       clampDecimal(c.AvgHoldAmount, 20, 4),
			HoldRatioTotal:      clampDecimal(c.HoldRatioTotal, 10, 4),
			FreeHoldRatioTotal:  clampDecimal(c.FreeHoldRatioTotal, 10, 4),
		}
		rowsAffected := db.UpsertShareholderCount(m)
		if rowsAffected == -1 {
			continue
		}
		if rowsAffected == 1 {
			result.NewCount++ // INSERT (新记录)
		} else {
			result.UpdCount++ // UPDATE (2=有变化, 0=无变化)
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
			ChangeDate:      parseTradeDate(c.Date),
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
		if rowsAffected == 1 {
			result.NewCount++ // INSERT (新记录)
		} else {
			result.UpdCount++ // UPDATE (2=有变化, 0=无变化)
		}
	}
	return result
}

// ============================================================================
//  数值工具函数
// ============================================================================

// clampDecimal 将浮点值钳制到 DECIMAL(p,s) 的合法范围
func clampDecimal(v float64, precision, scale int) float64 {
	maxVal := float64(intPow10(precision-scale)) - 1.0/float64(intPow10(scale))
	if v > maxVal {
		return maxVal
	}
	if v < -maxVal {
		return -maxVal
	}
	return v
}

// intPow10 计算 10^n
func intPow10(n int) int64 {
	r := int64(1)
	for i := 0; i < n; i++ {
		r *= 10
	}
	return r
}

// upsertDividendHistory 批量写入分红历史数据
func upsertDividendHistory(code string, dividends []adapter.DividendHistory) *CollectResult {
	result := &CollectResult{Total: len(dividends)}
	for _, d := range dividends {
		m := model.DividendHistory{
			StockCode:            code,
			NoticeDate:           parseTradeDate(d.NoticeDate),
			SecurityName:         d.SecurityName,
			PlanProfile:          d.PlanProfile,
			AssignProgress:       d.AssignProgress,
			EquityRecordDate:     parseTradeDate(d.EquityRecordDate),
			ExDividendDate:       parseTradeDate(d.ExDividendDate),
			PayCashDate:          parseTradeDate(d.PayCashDate),
			IsUnassign:           d.IsUnassign,
			ReportPeriod:         d.ReportDate,
			AssignObject:         d.AssignObject,
			NewProfile:           d.NewProfile,
			GmDecisionNoticeDate: parseTradeDate(d.GmDecisionNoticeDate),
			AnnualReportDate:     parseTradeDate(d.AnnualReportDate),
			TotalDividend:        clampDecimal(d.TotalDividend, 20, 2),
			TotalDividendA:       clampDecimal(d.TotalDividendA, 20, 2),
			ReportTime:           parseTradeDate(d.ReportTime),
		}
		rowsAffected := db.UpsertDividendHistory(m)
		if rowsAffected == -1 {
			continue
		}
		if rowsAffected == 1 {
			result.NewCount++ // INSERT (新记录)
		} else {
			result.UpdCount++ // UPDATE (2=有变化, 0=无变化)
		}
	}
	return result
}

// upsertNameChanges 批量写入名称变更数据
func upsertNameChanges(code string, changes []adapter.NameChange) *CollectResult {
	result := &CollectResult{Total: len(changes)}
	for _, c := range changes {
		m := model.NameChange{
			StockCode:    code,
			ChangeDate:   parseTradeDate(c.ChangeDate),
			SecurityName: c.SecurityName,
			ChangeReason: c.ChangeReason,
		}
		rowsAffected := db.UpsertNameChange(m)
		if rowsAffected == -1 {
			result.FailCount++ // 保存变更记录失败
			continue
		}
		if rowsAffected == 1 {
			result.NewCount++ // INSERT (新记录)
		} else {
			result.UpdCount++ // UPDATE (2=有变化, 0=无变化)
		}
	}
	return result
}
