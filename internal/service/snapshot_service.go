package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// ========== 快照计算模式 ==========

// SnapshotMode 快照计算模式
type SnapshotMode string

const (
	SnapshotSingleStockAllDates SnapshotMode = "single_stock_all_dates" // 1只股票 x 所有日期
	SnapshotAllStocksAllDates   SnapshotMode = "all_stocks_all_dates"   // 所有股票 x 所有日期
)

// ========== 结果结构体 ==========

// SnapshotResult 单次快照计算结果
type SnapshotResult struct {
	Code        string       `json:"code"`
	TradeDate   int          `json:"trade_date"`
	Mode        SnapshotMode `json:"mode"`
	UpsertCount int          `json:"upsert_count"`
	Success     int          `json:"success"` // 1=成功, 0=失败
	CostSeconds float64      `json:"cost_seconds"`
	Error       error        `json:"error,omitempty"`
}

// SnapshotBatchResult 批量计算汇总
type SnapshotBatchResult struct {
	Mode        SnapshotMode     `json:"mode"`
	Total       int              `json:"total"`
	Success     int              `json:"success"`
	Fail        int              `json:"fail"`
	CostSeconds float64          `json:"cost_seconds"`
	Details     []SnapshotResult `json:"details,omitempty"`
}

// ========== 服务入口 ==========

// SnapshotService 每日估值快照服务
type SnapshotService struct {
	registry *adapter.Registry // 数据源注册中心
}

// NewSnapshotService 创建快照服务
func NewSnapshotService() *SnapshotService {
	return &SnapshotService{
		registry: adapter.GetRegistry(),
	}
}

// ========== 统一入口 ==========

// Calc 统一快照计算入口，根据 code 和 tradeDate 自动分发
//   - code != "" → 单股票全日期（走 THS 实时采集 + 批量路径）
//   - code == "" → 全股票全日期
func (s *SnapshotService) Calc(ctx context.Context, code string) []SnapshotBatchResult {
	switch {
	case code != "":
		result := s.calcSingleStockAllDates(ctx, code)
		return []SnapshotBatchResult{result}
	default:
		return s.calcAllStocksAllDates(ctx)
	}
}

// ========== 内部实现（两种模式） ==========

// calcSingleStockAllDates 计算一只股票所有日期的快照（THS 实时采集 + 双指针批量优化）
//
// 算法流程：
//  1. 通过同花顺接口获取全部不复权日K线数据（保证实时性）
//  2. 从数据库预加载股本变化数据，按变动日期升序排序
//  3. 从数据库预加载财报数据，按报告期升序排序
//  4. 双指针遍历：
//     - 股本指针：推进到 <= 当日K线的最新股本变动记录
//     - 财报指针：推进到 <= 当日K线的最新财报记录
//  5. 用当日收盘价和当日股本计算总市值和流通市值
//  6. 根据当日之前的最新一期财报确定 PE(TTM/静态/动态)、PB、PS
//  7. 批量 upsert 写入数据库
func (s *SnapshotService) calcSingleStockAllDates(ctx context.Context, code string) SnapshotBatchResult {
	start := time.Now()
	result := SnapshotBatchResult{
		Mode: SnapshotSingleStockAllDates,
	}

	// ---- Step 1: 从同花顺获取全部不复权日K线（实时数据）----
	thsAdapter, ok := s.registry.Get(ths.AdapterName)
	if !ok {
		log.Printf("[snapshot] %s THS数据源未注册，无法执行批量路径", code)
		result.Fail = 1
		result.CostSeconds = time.Since(start).Seconds()
		return result
	}
	ths, ok := thsAdapter.(*ths.Adapter)
	if !ok {
		log.Printf("[snapshot] %s THS数据源类型错误", code)
		result.Fail = 1
		result.CostSeconds = time.Since(start).Seconds()
		return result
	}

	klineRecords, err := s.fetchKlinesFromTHS(ctx, code, ths)
	if err != nil {
		log.Printf("[snapshot] %s 从同花顺获取K线失败: %v", code, err)
		result.Fail = 1
		result.CostSeconds = time.Since(start).Seconds()
		return result
	}
	if len(klineRecords) == 0 {
		log.Printf("[snapshot] %s 无K线数据，跳过", code)
		result.CostSeconds = time.Since(start).Seconds()
		return result
	}
	log.Printf("[snapshot] %s 从同花顺获取 %d 条不复权日K", code, len(klineRecords))

	// ---- Step 2: 从DB预加载股本变动数据（按 change_date 升序）----
	shares, _ := s.loadAllShareChanges(code)

	// ---- Step 3: 从DB预加载财报数据（按 notice_date 公告日期升序）----
	reports, _ := s.loadAllReports(code)

	// ---- Step 3.5: 预处理财报：分为季度组 + 年报组 ----
	groups := preprocessReports(reports)

	// ---- Step 4: 双指针遍历 K线，逐日计算快照 ----
	snapshots := make([]model.StockDailySnapshot, 0, len(klineRecords))
	shareIdx := 0 // 股本指针（财报已预分组，无需指针）

	for _, kr := range klineRecords {
		select {
		case <-ctx.Done():
			log.Printf("[snapshot] %s 计算被取消", code)
			goto done
		default:
		}

		tradeDate := kr.TradeDate

		// 4a. 推进股本指针：找到 <= trade_date 的最新一条
		for shareIdx+1 < len(shares) && shares[shareIdx+1].ChangeDate <= tradeDate {
			shareIdx++
		}

		// 4b. 取当前有效股本（财报由 buildSnapshotFromTHSKline 内部从 groups 查找）
		var currentShare *model.ShareChange
		if shareIdx < len(shares) && shares[shareIdx].ChangeDate <= tradeDate {
			currentShare = &shares[shareIdx]
		}

		// 4c. 构建当日快照（内部自动从 groups 查找当日有效财报）
		snap := s.buildSnapshotFromTHSKline(code, tradeDate, kr.Close, currentShare, &groups)
		snapshots = append(snapshots, snap)
	}

done:
	// ---- Step 5: 批量写入数据库 ----
	successCount := 0
	if len(snapshots) > 0 {
		totalRows, batchErr := db.BatchUpsertSnapshots(snapshots)
		if batchErr != nil {
			log.Printf("[snapshot] %s 批量写入失败: %v", code, batchErr)
			for _, snap := range snapshots {
				ok := db.UpsertSnapshot(snap)
				detail := SnapshotResult{
					Code: code, TradeDate: snap.TradeDate, Mode: SnapshotSingleStockAllDates,
				}
				if ok {
					successCount++
					detail.UpsertCount = 1
				} else {
					detail.Error = batchErr
				}
				result.Details = append(result.Details, detail)
			}
		} else {
			successCount = int(totalRows)
			if len(snapshots) <= 50 {
				for _, snap := range snapshots {
					result.Details = append(result.Details, SnapshotResult{
						Code: code, TradeDate: snap.TradeDate, Mode: SnapshotSingleStockAllDates,
						UpsertCount: 1,
					})
				}
			}
		}
	}

	result.Total = len(klineRecords)
	result.Success = successCount
	result.Fail = result.Total - successCount
	result.CostSeconds = time.Since(start).Seconds()

	if result.Total > 0 {
		log.Printf("[snapshot] 单股票全日期完成 [%s]: K线=%d 快照=%d 成功=%d 失败=%d 耗时=%.1fs",
			code, result.Total, len(snapshots), result.Success, result.Fail, result.CostSeconds)
	}
	return result
}

// calcAllStocksAllDates 计算所有股票所有日期的快照
func (s *SnapshotService) calcAllStocksAllDates(ctx context.Context) []SnapshotBatchResult {
	stocks := db.LoadAllStockCodes()
	var results []SnapshotBatchResult

	for _, stock := range stocks {
		select {
		case <-ctx.Done():
			log.Printf("[snapshot] 全股票全日期计算被取消")
			return results
		default:
		}

		br := s.calcSingleStockAllDates(ctx, stock.Code)
		results = append(results, br)
	}

	totalS, totalF := 0, 0
	for _, r := range results {
		totalS += r.Success
		totalF += r.Fail
	}
	log.Println("==============================")
	log.Printf("全量快照计算完成! 成功=%d 失败=%d", totalS, totalF)
	log.Println("==============================")

	return results
}

// ================================================================
//  核心计算：从 THS K线 数据构建快照（批量路径）
// ================================================================

// buildSnapshotFromTHSKline 从同花顺日K线 + 股本 + 预处理财报组 构建快照（批量路径）
//
// share:  当日有效股本（可为 nil）
// groups: 预处理后的财报分组（季度组+年报组），内部自行查找当日有效财报
func (s *SnapshotService) buildSnapshotFromTHSKline(code string, tradeDate int, closePriceCents int64,
	share *model.ShareChange, groups *reportGroups) model.StockDailySnapshot {

	snap := model.StockDailySnapshot{
		StockCode: code,
		TradeDate: tradeDate,
	}

	closePriceYuan := float64(closePriceCents) / 100.0 // 分 → 元

	// --- 市值计算：收盘价 × 股本 ---
	var totalShares, floatShares int64
	if share != nil {
		totalShares = share.TotalShares
		floatShares = share.FloatAShares
	}
	snap.TotalShares = totalShares
	snap.FloatShares = floatShares

	if totalShares > 0 {
		snap.TotalMarketCap = float64(totalShares) * closePriceYuan
	}
	if floatShares > 0 {
		snap.CirculateMarketCap = float64(floatShares) * closePriceYuan
	}

	// --- 估值指标：从预处理分组中查找当日有效财报 ---
	report := groups.findLatestEffectiveReport(tradeDate)
	if report != nil && snap.TotalMarketCap > 0 {
		marketCap := snap.TotalMarketCap

		// 每股净资产 & 市净率
		snap.BVPS = report.BVPS
		if report.BVPS > 0 {
			snap.PB = closePriceYuan / report.BVPS
		}

		// 盈利能力指标：直接取财报原始值
		snap.ROE = report.ROEW           // 净资产收益率-加权(%)
		snap.ROA = report.ROA            // 总资产收益率(%)
		snap.GrossMargin = report.GrossMargin // 销售毛利率(%)
		snap.NetMargin = report.NetMargin     // 销售净利率(%)

		// 每股指标
		snap.BasicEPS = report.BasicEPS       // 基本每股收益(元)

		// 财报当期数据：直接取最新一期财报值
		snap.ParentNetProfit = report.ParentNetProfit // 归母净利润
		snap.DeductNetProfit = report.DeductNetProfit // 扣非净利润
		snap.TotalRevenue = report.TotalRevenue        // 营业总收入

		// 偿债能力指标
		snap.DebtRatio = report.DebtRatio              // 资产负债率(%)

		// 市盈率(动态): 总市值 / (最近一期季报归母净利润 × 年化系数)
		//   一季报 → ×4, 中报 → ×2, 三季报 → ÷3×4, 年报 → 跳过
		snap.PEDynamic = s.calcDynamicPE(marketCap, report)

		// 市盈率(静态): 总市值 / 最近一期年报归母净利润（从年报组查找）
		snap.PEStatic = s.calcStaticPE(marketCap, groups)

		// 市盈率(TTM): 总市值 / 最近四个单季度归母净利润之和（差值法）
		snap.PETTM = s.calcTTMPE(marketCap, groups)

		// 市销率(TTM): 总市值 / 最近4个季度营业总收入之和（差值法推导）
		qProfits := s.calcQuarterlyProfits(groups)
		if len(qProfits) > 0 {
			startIdx := len(qProfits) - 4
			if startIdx < 0 {
				startIdx = 0
			}
			var ttmRevenue float64
			for i := startIdx; i < len(qProfits); i++ {
				ttmRevenue += qProfits[i].Revenue
			}
			if ttmRevenue > 0 {
				snap.PSTTM = marketCap / ttmRevenue
			}
		}
	}

	return snap
}

// ================================================================
//  PE 计算子函数
// ================================================================

// calcDynamicPE 计算市盈率(动态)
//
// 规则：根据最近一期财报的类型，对归母净利润进行年化，再除总市值。
//   - 一季报: 归母净利润 × 4
//   - 中报:   归母净利润 × 2
//   - 三季报: 归母净利润 ÷ 3 × 4
//   - 年报:   跳过不计算（年报与一季报通常同日公告，用一季报数据）
//
// 允许净利润为负 → PE 为负值（亏损公司）。
// 返回 0 表示无法计算（年报类型或年化利润恰好为 0）。
func (s *SnapshotService) calcDynamicPE(marketCap float64, r *model.PerformanceReport) float64 {
	var annualizedProfit float64
	switch r.ReportType {
	case "一季报":
		annualizedProfit = r.ParentNetProfit * 4
	case "中报":
		annualizedProfit = r.ParentNetProfit * 2
	case "三季报":
		annualizedProfit = r.ParentNetProfit / 3.0 * 4
	case "年报":
		return 0 // 跳过：年报和一季报公告日期通常相同，应使用一季报数据做动态预估
	default:
		annualizedProfit = r.ParentNetProfit * 4 // 未识别的 ReportType，默认按一季报处理
	}

	if annualizedProfit == 0 {
		return 0 // 净利润恰好为0时无法定义PE
	}
	return marketCap / annualizedProfit
}

// calcStaticPE 计算市盈率(静态)
//
// 规则：从年报组(按 notice_date 降序)取最近一期年报，
//
//	用其归母净利润作为"全年净利润"，总市值 ÷ 该值。
//	如果没有可用的年报（如新股上市不足一年），返回 0。
func (s *SnapshotService) calcStaticPE(marketCap float64, groups *reportGroups) float64 {
	// 年报组已按 notice_date 升序，从末尾往前找
	for i := len(groups.Annual) - 1; i >= 0; i-- {
		if groups.Annual[i].ParentNetProfit == 0 {
			return 0
		}
		return marketCap / groups.Annual[i].ParentNetProfit // 允许负值 → 负PE
	}
	return 0
}

// calcTTMPE 计算市盈率(TTM) —— 基于单季度利润差值法
//
// 规则：通过财报数据的差值推导各单季度归母净利润：
//
//	Q1 = 一季报.ParentNetProfit
//	Q2 = 中报.ParentNetProfit − 一季报.ParentNetProfit
//	Q3 = 三季报.ParentNetProfit − 中报.ParentNetProfit
//	Q4 = 年报.ParentNetProfit − 三季报.ParentNetProfit
//
// 取最近4个可用单季度利润求和 → 总市值 ÷ TTM利润之和。
func (s *SnapshotService) calcTTMPE(marketCap float64, groups *reportGroups) float64 {
	qProfits := s.calcQuarterlyProfits(groups)
	if len(qProfits) == 0 {
		return 0
	}

	// 取最近4个单季度（列表已按年份+季度升序）
	startIdx := len(qProfits) - 4
	if startIdx < 0 {
		startIdx = 0
	}

	var ttmSum float64
	for i := startIdx; i < len(qProfits); i++ {
		ttmSum += qProfits[i].SingleProfit
	}

	if ttmSum == 0 {
		return 0 // 除零保护，允许负值但不允许恰好为0
	}
	return marketCap / ttmSum // 允许负值 → 负PE（亏损公司）
}

// ================================================================
//  同花顺数据获取（批量路径专用）
// ================================================================

// thsKlineRecord 同花顺K线记录内部表示
// 用于双指针算法遍历，只保留必要字段
type thsKlineRecord struct {
	Code      string // 股票代码
	TradeDate int    // 交易日期 YYYYMMDD
	Close     int64  // 收盘价（分）
}

// fetchKlinesFromTHS 通过同花顺接口获取指定股票的全部不复权日K线数据
// 返回按交易日期升序排列的 K 线记录
func (s *SnapshotService) fetchKlinesFromTHS(ctx context.Context, code string, ths *ths.Adapter) ([]thsKlineRecord, error) {
	raw, err := ths.GetDailyKLine(ctx, code, adapter.AdjNone)
	if err != nil {
		return nil, fmt.Errorf("同花顺获取日K失败 [%s]: %w", code, err)
	}

	if len(raw) == 0 {
		return nil, nil
	}

	records := make([]thsKlineRecord, 0, len(raw))
	for _, r := range raw {
		td, err := parseDateToTradeDate(r.Date)
		if err != nil {
			// 跳过无法解析日期的记录
			log.Printf("[snapshot] 跳过无效日期 %s [%s]", r.Date, code)
			continue
		}
		records = append(records, thsKlineRecord{
			Code:      r.Code,
			TradeDate: td,
			Close:     r.Close,
		})
	}

	today, err := ths.GetTodayData(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("同花顺获取日K失败 [%s]: %w", code, err)
	}
	td, err := parseDateToTradeDate(today.Date)
	if err != nil {
		// 跳过无法解析日期的记录
		log.Printf("[snapshot] 跳过无效日期 %s [%s]", today.Date, code)
		return records, nil
	}
	if records[len(records)-1].TradeDate < td {
		records = append(records, thsKlineRecord{
			Code:      today.Code,
			TradeDate: td,
			Close:     today.Close,
		})
	} else {
		records[len(records)-1].Close = today.Close
	}

	return records, nil
}

// parseDateToTradeDate 将 "2026-04-12" 格式转为 20260412 整数
func parseDateToTradeDate(dateStr string) (int, error) {
	// 支持 "2026-04-12" 和 "20260412" 两种格式
	clean := strings.ReplaceAll(dateStr, "-", "")
	if len(clean) != 8 {
		return 0, fmt.Errorf("日期格式错误: %s", dateStr)
	}
	return strconv.Atoi(clean)
}

// ================================================================
//  DB 预加载方法（批量路径专用）
// ================================================================

// loadAllShareChanges 加载指定股票的全部股本变动数据（按 change_date 升序）
func (s *SnapshotService) loadAllShareChanges(code string) ([]model.ShareChange, error) {
	var shares []model.ShareChange
	err := db.GetDB().
		Where("stock_code = ?", code).
		Order("change_date ASC").
		Find(&shares).Error
	return shares, err
}

// loadAllReports 加载指定股票的全部财报数据（按 notice_date 公告日期升序）
func (s *SnapshotService) loadAllReports(code string) ([]model.PerformanceReport, error) {
	var reports []model.PerformanceReport
	err := db.GetDB().
		Where("stock_code = ?", code).
		Order("notice_date ASC").
		Find(&reports).Error
	return reports, err
}

// ================================================================
//  财报预处理：分组 + 单季度利润推导
// ================================================================

// reportGroups 预处理后的财报分组
//
// 将原始财报列表拆分为两组，用于后续 PE 精确计算：
//   - Quarterly: 一季报 / 中报 / 三季报（按 notice_date 升序）
//   - Annual:    年报（按 notice_date 升序）
//   - 其他类型（如盈报、快报等）舍弃
type reportGroups struct {
	Quarterly []model.PerformanceReport // 季度财报
	Annual    []model.PerformanceReport // 年报
}

// preprocessReports 预处理财报：按类型分为季度组和年报组
func preprocessReports(reports []model.PerformanceReport) reportGroups {
	var g reportGroups
	for i := range reports {
		switch reports[i].ReportType {
		case "一季报", "中报", "三季报":
			g.Quarterly = append(g.Quarterly, reports[i])
		case "年报":
			g.Annual = append(g.Annual, reports[i])
		}
		// 其他类型静默舍弃
	}
	return g
}

// findLatestEffectiveReport 从预处理后的财报分组中查找 notice_date <= trade_date 的最新一期财报
//
// 查找优先级：季度组(一季报/中报/三季报) 和 年报组合并后，取 notice_date 最大且 <= trade_date 的那条。
// 用于动态 PE（判断最新季报类型）、PB（BVPS）、PS（TotalRevenue）。
//
// 返回 nil 表示当天无可用财报。
func (g *reportGroups) findLatestEffectiveReport(tradeDate int) *model.PerformanceReport {
	var latest *model.PerformanceReport
	latestNoticeDate := 0

	// 遍历两组，找 notice_date 最大且不超过 trade_date 的记录
	for i := range g.Quarterly {
		if g.Quarterly[i].NoticeDate <= tradeDate && g.Quarterly[i].NoticeDate > latestNoticeDate {
			latest = &g.Quarterly[i]
			latestNoticeDate = g.Quarterly[i].NoticeDate
		}
	}
	for i := range g.Annual {
		if g.Annual[i].NoticeDate <= tradeDate && g.Annual[i].NoticeDate > latestNoticeDate {
			latest = &g.Annual[i]
			latestNoticeDate = g.Annual[i].NoticeDate
		}
	}

	return latest
}

// quarterSingleProfit 单季度利润+营收条目（用于 TTM 计算）
type quarterSingleProfit struct {
	Year         int     // 年份
	Quarter      int     // 季度 1~4
	SingleProfit float64 // 该季度归母净利润（差值法推导）
	Revenue      float64 // 该季度营业总收入（差值法推导）
}

// calcQuarterlyProfits 从预处理的财报分组中推导各单季度归母净利润
//
// 差值公式：
//
//	Q1 = 一季报.ParentNetProfit
//	Q2 = 中报.ParentNetProfit − 一季报.ParentNetProfit
//	Q3 = 三季报.ParentNetProfit − 中报.ParentNetProfit
//	Q4 = 年报.ParentNetProfit − 三季报.ParentNetProfit
//
// 返回按(年份,季度)升序排列的单季度利润列表。
func (s *SnapshotService) calcQuarterlyProfits(groups *reportGroups) []quarterSingleProfit {

	// 按报告期合并排序（年报和季报都参与排序，保证时间顺序）
	type taggedReport struct {
		r          *model.PerformanceReport
		reportDate int
		isAnnual   bool
	}
	var merged []taggedReport
	for i := range groups.Quarterly {
		merged = append(merged, taggedReport{r: &groups.Quarterly[i], reportDate: groups.Quarterly[i].ReportDate})
	}
	for i := range groups.Annual {
		merged = append(merged, taggedReport{r: &groups.Annual[i], reportDate: groups.Annual[i].ReportDate, isAnnual: true})
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].reportDate < merged[j].reportDate })

	// 逐年维护累计利润+营收状态，用差值推算单季度值
	type yearState struct {
		q1                           float64 // 一季报累计 (=Q1单季)
		h1                           float64 // 中报累计   (=Q1+Q2)
		q3                           float64 // 三季报累计 (=Q1+Q2+Q3)
		full                         float64 // 年报累计   (=Q1+Q2+Q3+Q4)
		rq1                          float64 // 一季报营收累计 (=Q1单季营收)
		rh1                          float64 // 中报营收累计   (=Q1+Q2营收)
		rq3                          float64 // 三季报营收累计 (=Q1+Q2+Q3营收)
		rfull                        float64 // 年报营收累计   (=Q1+Q2+Q3+Q4营收)
		hasQ1, hasH1, hasQ3, hasFull bool
	}
	states := make(map[int]*yearState)

	var result []quarterSingleProfit

	for _, item := range merged {
		year := item.r.ReportDate / 10000
		st, ok := states[year]
		if !ok {
			st = &yearState{}
			states[year] = st
		}

		switch item.r.ReportType {
		case "一季报":
			st.q1 = item.r.ParentNetProfit
			st.rq1 = item.r.TotalRevenue
			st.hasQ1 = true
			result = append(result, quarterSingleProfit{Year: year, Quarter: 1, SingleProfit: item.r.ParentNetProfit, Revenue: item.r.TotalRevenue})

		case "中报":
			st.h1 = item.r.ParentNetProfit
			st.rh1 = item.r.TotalRevenue
			st.hasH1 = true
			if st.hasQ1 {
				q2 := item.r.ParentNetProfit - st.q1
				r2 := item.r.TotalRevenue - st.rq1
				result = append(result, quarterSingleProfit{Year: year, Quarter: 2, SingleProfit: q2, Revenue: r2})
			}

		case "三季报":
			st.q3 = item.r.ParentNetProfit
			st.rq3 = item.r.TotalRevenue
			st.hasQ3 = true
			if st.hasH1 {
				q3 := item.r.ParentNetProfit - st.h1
				r3 := item.r.TotalRevenue - st.rh1
				result = append(result, quarterSingleProfit{Year: year, Quarter: 3, SingleProfit: q3, Revenue: r3})
			}

		case "年报":
			st.full = item.r.ParentNetProfit
			st.rfull = item.r.TotalRevenue
			st.hasFull = true
			if st.hasQ3 {
				q4 := item.r.ParentNetProfit - st.q3
				r4 := item.r.TotalRevenue - st.rq3
				result = append(result, quarterSingleProfit{Year: year, Quarter: 4, SingleProfit: q4, Revenue: r4})
			}
		}
	}

	return result
}
