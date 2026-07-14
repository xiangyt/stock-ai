package datacollect

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ========== 快照计算模式 ==========

// SnapshotMode 快照计算模式
type SnapshotMode string

const (
	SnapshotSingleStockAllDates SnapshotMode = "single_stock_all_dates" // 1只股票 x 所有日期
	SnapshotAllStocksAllDates   SnapshotMode = "all_stocks_all_dates"   // 所有股票 x 所有日期
)

// ========== 结果结构体 ==========

// SnapshotBatchResult 批量计算汇总
type SnapshotBatchResult struct {
	Mode               SnapshotMode `json:"mode"`
	TotalStocks        int          `json:"total_stocks"`        // 总股票数（有快照数据的股票数）
	SuccessStocks      int          `json:"success_stocks"`      // 成功处理的股票数（快照全部写入成功）
	FailStocks         int          `json:"fail_stocks"`         // 失败股票数（获取K线或计算失败）
	TotalSnapshots     int          `json:"total_snapshots"`     // 总快照条数
	SuccessSnapshots   int          `json:"success_snapshots"`   // 成功写入的快照条数
	FailSnapshots      int          `json:"fail_snapshots"`      // 写入失败的快照条数
	CostSeconds        float64      `json:"cost_seconds"`
	FailedCodes        []string     `json:"failed_codes"`        // 失败的股票代码列表
}

// ========== 服务入口 ==========

// SnapshotService 每日估值快照服务
type SnapshotService struct {
	registry *adapter.Registry // 数据源注册中心
}

// ============================================================================
//  全局单例
// ============================================================================

var (
	snapOnce     sync.Once
	snapInstance *SnapshotService
)

// GetSnapshotService 返回 SnapshotService 全局单例（线程安全）。
func GetSnapshotService() *SnapshotService {
	snapOnce.Do(func() {
		snapInstance = &SnapshotService{
			registry: adapter.GetRegistry(),
		}
	})
	return snapInstance
}

// ========== 统一入口 ==========

// Calc 统一快照计算入口，根据 code 和 tradeDate 自动分发
//   - code != "" → 单股票全日期（走 THS 实时采集 + 批量路径）
//   - code == "" → 全股票全日期
func (s *SnapshotService) Calc(ctx context.Context, code string) SnapshotBatchResult {
	switch {
	case code != "":
		return s.calcSingleStockAllDates(ctx, code)
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

	// ---- Step1.5: 从DB加载快照数据，获取trade_date最大的那条记录----
	ss, _ := db.FindSnapshotsByStock(code, 0, 0, 1, false)
	var maxDate int
	if len(ss) > 0 {
		maxDate = ss[0].TradeDate
	}

	snapshots, err := s.calcStockAfterDate(ctx, code, maxDate)
	if err != nil {
		result.FailStocks = 1
		result.CostSeconds = time.Since(start).Seconds()
		return result
	}

	// ---- Step 5: 批量写入数据库 ----
	successCount := 0
	if len(snapshots) > 0 {
		totalRows, batchErr := db.BatchUpsertSnapshots(snapshots)
		if batchErr != nil {
			log.Printf("[snapshot] %s 批量写入失败，err: %v", code, batchErr)
		} else {
			successCount = int(totalRows)
		}
	}

	result.TotalSnapshots = len(snapshots)
	result.SuccessSnapshots = successCount
	result.FailSnapshots = result.TotalSnapshots - successCount
	result.CostSeconds = time.Since(start).Seconds()

	// 股票维度：有快照且全部写入成功才算成功，否则算失败
	if result.TotalSnapshots > 0 {
		result.TotalStocks = 1
		if result.FailSnapshots == 0 {
			result.SuccessStocks = 1
		} else {
			result.FailStocks = 1
		}
	}

	if result.TotalSnapshots > 0 {
		log.Printf("[snapshot] 单股票全日期完成 [%s]: 快照=%d 写入成功=%d 写入失败=%d 耗时=%.1fs",
			code, result.TotalSnapshots, result.SuccessSnapshots, result.FailSnapshots, result.CostSeconds)
	}
	return result
}

func (s *SnapshotService) calcStockAfterDate(ctx context.Context, code string, date int) (
	[]model.StockDailySnapshot, error) {
	// ---- Step 1: 从同花顺获取全部不复权日K线（实时数据）----
	thsAdapter, ok := s.registry.Get(ths.AdapterName)
	if !ok {
		log.Printf("[snapshot] %s THS数据源未注册，无法执行批量路径", code)
		return nil, fmt.Errorf("%s THS数据源未注册，无法执行批量路径", code)
	}
	ths, ok := thsAdapter.(*ths.Adapter)
	if !ok {
		log.Printf("[snapshot] %s THS数据源类型错误", code)
		return nil, fmt.Errorf("[snapshot] %s THS数据源类型错误", code)
	}

	klineRecords, err := s.fetchKlinesFromTHS(ctx, code, ths)
	if err != nil {
		log.Printf("[snapshot] %s 从同花顺获取K线失败: %v", code, err)
		return nil, fmt.Errorf("[snapshot] %s 从同花顺获取K线失败: %v", code, err)
	}
	if len(klineRecords) == 0 {
		log.Printf("[snapshot] %s 无K线数据，跳过", code)
		return nil, nil
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
			return nil, nil
		default:
		}

		tradeDate := kr.TradeDate
		if tradeDate <= date {
			continue
		}
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

	return snapshots, nil
}

// calcAllStocksAllDates 计算所有股票所有日期的快照
func (s *SnapshotService) calcAllStocksAllDates(ctx context.Context) SnapshotBatchResult {
	stocks := db.LoadAllStockCodes()
	result := SnapshotBatchResult{
		Mode: SnapshotAllStocksAllDates,
	}
	allStart := time.Now()

	for _, stock := range stocks {
		select {
		case <-ctx.Done():
			log.Printf("[snapshot] 全股票全日期计算被取消")
			result.CostSeconds = time.Since(allStart).Seconds()
			return result
		default:
		}

		br := s.calcSingleStockAllDates(ctx, stock.Code)
		result.TotalStocks += br.TotalStocks
		result.SuccessStocks += br.SuccessStocks
		result.FailStocks += br.FailStocks
		result.TotalSnapshots += br.TotalSnapshots
		result.SuccessSnapshots += br.SuccessSnapshots
		result.FailSnapshots += br.FailSnapshots

		if br.FailStocks > 0 {
			result.FailedCodes = append(result.FailedCodes, stock.Code)
		}
	}

	result.CostSeconds = time.Since(allStart).Seconds()
	log.Println("==============================")
	log.Printf("全量快照计算完成! 股票:总数=%d 成功=%d 失败=%d | 快照:总数=%d 写入成功=%d 写入失败=%d | 耗时=%.1fs",
		result.TotalStocks, result.SuccessStocks, result.FailStocks,
		result.TotalSnapshots, result.SuccessSnapshots, result.FailSnapshots,
		result.CostSeconds)
	log.Println("==============================")

	return result
}

// ================================================================
//  核心计算：从 THS K线 数据构建快照（批量路径）
// ================================================================

// buildSnapshotFromTHSKline 从同花顺日K线（分） + 股本 + 财报组 构建快照
//
// 薄封装：将 closePriceCents(分) 转为 closePriceYuan(元) 后委托 buildStockSnapshot。
func (s *SnapshotService) buildSnapshotFromTHSKline(code string, tradeDate int, closePriceCents int64,
	share *model.ShareChange, groups *reportGroups) model.StockDailySnapshot {
	return buildStockSnapshot(code, tradeDate, float64(closePriceCents)/100.0, share, groups)
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
// 返回 0 表示年化利润恰好为 0（无法定义PE）。
//
// 注意：年报本身就是全年利润，不需要年化。当最新有效财报是年报
// （通常意味着一季报尚未发布）时，动态PE与静态PE、TTM PE 一致。
func calcDynamicPE(marketCap float64, r *model.PerformanceReport) float64 {
	var annualizedProfit float64
	switch r.ReportType {
	case "一季报":
		annualizedProfit = r.ParentNetProfit * 4
	case "中报":
		annualizedProfit = r.ParentNetProfit * 2
	case "三季报":
		annualizedProfit = r.ParentNetProfit / 3.0 * 4
	case "年报":
		annualizedProfit = r.ParentNetProfit // 年报 = 全年利润，无需年化
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
// 规则：从年报组中查找 notice_date <= tradeDate 的最新一期年报，
//
//	用其归母净利润作为"全年净利润"，总市值 ÷ 该值。
//	如果当日之前无可用的年报（如新股上市不足一年），返回 0。
func calcStaticPE(marketCap float64, groups *reportGroups, tradeDate int) float64 {
	annual := groups.findLatestAnnual(tradeDate)
	if annual == nil || annual.ParentNetProfit == 0 {
		return 0
	}
	return marketCap / annual.ParentNetProfit // 允许负值 → 负PE
}

// ttmValues TTM 计算结果（利润 + 营收）
type ttmValues struct {
	Profit  float64 // TTM 归母净利润
	Revenue float64 // TTM 营业总收入
}

// calcTTMValues 计算 TTM（滚动四季）的归母净利润和营业总收入
//
// 核心公式：TTM = 最新财报累计值 + (上年年报累计值 - 上年同期财报累计值)
//
// 等价于"最近4个单季度之和"，但不需要逐季拆分，只需3个数据点：
//   - 最新一期财报（一季报/中报/三季报/年报）
//   - 上一年年报
//   - 上一年同期类型财报
//
// 示例（最新=2025一季报时）:
//
//	TTM = Q1_2025利润 + (Full_2024利润 − Q1_2024利润)
//	    = 2025Q1 + (2024全年 − 2024Q1)
//	    = 2025Q1 + 2024Q2 + 2024Q3 + 2024Q4   ← 正好是最近4个季度
//
// 各场景:
//
//	最新=一季报 → TTM = 本年一季报   + (上年年报 - 上年一季报)
//	最新=中报   → TTM = 本年中报     + (上年年报 - 上年中报)
//	最新=三季报 → TTM = 本年三季报   + (上年年报 - 上年三季报)
//	最新=年报   → TTM = 年报(=全年)                    ← 四种 PE 一致
//
// 返回零值表示数据不足无法计算。
func calcTTMValues(latest *model.PerformanceReport, groups *reportGroups) ttmValues {
	if latest == nil {
		return ttmValues{}
	}

	switch latest.ReportType {
	case "年报":
		// 年报本身就是全年数据，无需补齐
		return ttmValues{Profit: latest.ParentNetProfit, Revenue: latest.TotalRevenue}

	case "一季报", "中报", "三季报":
		prevAnnual := findReportByYearAndType(groups, latest.ReportDate/10000-1, "年报")
		prevSame := findReportByYearAndType(groups, latest.ReportDate/10000-1, latest.ReportType)

		if prevAnnual == nil || prevSame == nil {
			return ttmValues{} // 缺上年数据，无法计算
		}

		return ttmValues{
			Profit:  latest.ParentNetProfit + (prevAnnual.ParentNetProfit - prevSame.ParentNetProfit),
			Revenue: latest.TotalRevenue + (prevAnnual.TotalRevenue - prevSame.TotalRevenue),
		}

	default:
		return ttmValues{}
	}
}

// findReportByYearAndType 从混合组中按报告期年份+报表类型查找指定财报
//
// 用于 TTM 计算中定位"上年年报"和"上年同期季报"。
func findReportByYearAndType(groups *reportGroups, year int, reportType string) *model.PerformanceReport {
	for i := range groups.All {
		if groups.All[i].ReportDate/10000 == year && groups.All[i].ReportType == reportType {
			return &groups.All[i]
		}
	}
	return nil
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
		td, err := utils.ParseDateToTradeDate(r.Date)
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
	td, err := utils.ParseDateToTradeDate(today.Date)
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
// 将原始财报列表拆分为三组，分别用于三种 PE 及其他指标的精确计算：
//   - Quarterly: 一季报 / 中报 / 三季报（按 notice_date 升序）→ 动态 PE
//   - Annual:    年报（按 notice_date 升序）                     → 静态 PE
//   - All:       季报 + 年报合并（按 notice_date 升序）          → TTM PE / PB / ROE 等
//   - 其他类型（如盈报、快报等）舍弃
type reportGroups struct {
	Quarterly []model.PerformanceReport // 季度财报
	Annual    []model.PerformanceReport // 年报
	All       []model.PerformanceReport // 混合（季报+年报）
}

// preprocessReports 预处理财报：按类型分为季度组、年报组和混合组
func preprocessReports(reports []model.PerformanceReport) reportGroups {
	var g reportGroups
	for i := range reports {
		switch reports[i].ReportType {
		case "一季报", "中报", "三季报":
			g.Quarterly = append(g.Quarterly, reports[i])
			g.All = append(g.All, reports[i])
		case "年报":
			g.Annual = append(g.Annual, reports[i])
			g.All = append(g.All, reports[i])
		}
		// 其他类型静默舍弃
	}

	// 三组均按 notice_date 升序排列，保证二分/线性查找的一致性
	sort.Slice(g.Quarterly, func(i, j int) bool { return g.Quarterly[i].NoticeDate < g.Quarterly[j].NoticeDate })
	sort.Slice(g.Annual, func(i, j int) bool { return g.Annual[i].NoticeDate < g.Annual[j].NoticeDate })
	sort.Slice(g.All, func(i, j int) bool { return g.All[i].NoticeDate < g.All[j].NoticeDate })

	return g
}

// findLatestFromList 从给定报表切片中查找 notice_date <= tradeDate 的最新一条
//
// 通用底层方法，要求列表已按 notice_date 升序排列。
// 返回 nil 表示无匹配记录。
func findLatestFromList(list []model.PerformanceReport, tradeDate int) *model.PerformanceReport {
	var latest *model.PerformanceReport
	for i := range list {
		if list[i].NoticeDate <= tradeDate {
			latest = &list[i]
		} else {
			break // 升序排列，后续日期更大无需继续
		}
	}
	return latest
}

// findLatestQuarterly 从季报组中查找最新一期季报
//
// 用于动态 PE：用最近一期季报归母净利润 × 年化系数。
func (g *reportGroups) findLatestQuarterly(tradeDate int) *model.PerformanceReport {
	return findLatestFromList(g.Quarterly, tradeDate)
}

// findLatestAnnual 从年报组中查找最新一期年报
//
// 用于静态 PE：用最近一期年报归母净利润作为全年利润。
func (g *reportGroups) findLatestAnnual(tradeDate int) *model.PerformanceReport {
	return findLatestFromList(g.Annual, tradeDate)
}

// findLatest 从混合组中查找最新一期任意类型财报
//
// 用于 PB（BVPS）、ROE、营收等基本面指标：取公告日最近的财报数据，
// 不区分季报/年报。
func (g *reportGroups) findLatest(tradeDate int) *model.PerformanceReport {
	return findLatestFromList(g.All, tradeDate)
}

// ================================================================
//  实时快照构建（供 quotecache 等外部调用）
// ================================================================

// BuildDailySnapshot 从 adapter 实时行情 + DB 财报/股本 构建完整实时快照
//
// 供 CachedStock.GetDailySnapshot() 调用，消除 quotecache 与 datacollect 之间的
// 快照计算代码重复。
//
// 流程: 加载 DB 财报 → 加载 DB 股本 → 预处理分组 → 查找当日有效股本 → 构建快照
// 无财报时降级为基础转换（仅 PE/PB/市值）。
func BuildDailySnapshot(code string, price *adapter.StockPriceDaily) (*model.StockDailySnapshot, error) {
	if price == nil || price.Close == 0 {
		return nil, fmt.Errorf("invalid price data for %s", code)
	}

	// 从 DB 加载财报数据
	reports, _ := loadAllReportsForSnapshot(code)

	// 无财报则降级为仅 PE/PB/市值的基础快照
	if len(reports) == 0 {
		return convertToSnapshot(price), nil
	}

	// 从 DB 加载股本变动数据
	shares, _ := loadAllShareChangesForSnapshot(code)

	groups := preprocessReports(reports)
	tradeDate := parseDateToTradeInt(price.Date)
	closePriceYuan := float64(price.Close) / 100.0 // 分 → 元

	// 查找当日有效股本（双指针：找到 <= tradeDate 的最新一条）
	var currentShare *model.ShareChange
	for i := len(shares) - 1; i >= 0; i-- {
		if shares[i].ChangeDate <= tradeDate {
			currentShare = &shares[i]
			break
		}
	}

	snap := buildStockSnapshot(code, tradeDate, closePriceYuan, currentShare, &groups)
	return &snap, nil
}

// loadAllReportsForSnapshot 加载指定股票的全部财报数据（按 notice_date 升序）
// 用于 BuildDailySnapshot，不依赖 SnapshotService receiver。
func loadAllReportsForSnapshot(code string) ([]model.PerformanceReport, error) {
	var reports []model.PerformanceReport
	err := db.GetDB().
		Where("stock_code = ?", code).
		Order("notice_date ASC").
		Find(&reports).Error
	return reports, err
}

// loadAllShareChangesForSnapshot 加载指定股票的全部股本变动（按 change_date 升序）
// 用于 BuildDailySnapshot，不依赖 SnapshotService receiver。
func loadAllShareChangesForSnapshot(code string) ([]model.ShareChange, error) {
	var shares []model.ShareChange
	err := db.GetDB().
		Where("stock_code = ?", code).
		Order("change_date ASC").
		Find(&shares).Error
	return shares, err
}

// convertToSnapshot 将 adapter.StockPriceDaily 转换为基础 StockDailySnapshot
// 仅包含 PETTM/PB/TotalMarketCap，用于无财报时的降级返回。
func convertToSnapshot(price *adapter.StockPriceDaily) *model.StockDailySnapshot {
	return &model.StockDailySnapshot{
		StockCode:      price.Code,
		TradeDate:      parseDateToTradeInt(price.Date),
		PETTM:          price.Pe,
		PB:             price.Pb,
		TotalMarketCap: price.MarketCap * 1e8,
	}
}

// parseDateToTradeInt 将日期字符串 "2025-07-09" 转换为整数 20250709
func parseDateToTradeInt(dateStr string) int {
	if len(dateStr) < 10 {
		return 0
	}
	clean := dateStr[0:4] + dateStr[5:7] + dateStr[8:10]
	val, err := strconv.Atoi(clean)
	if err != nil {
		return 0
	}
	return val
}

// buildStockSnapshot 从收盘价(元) + 股本 + 财报组 构建完整快照（纯函数，无 DB 依赖）
//
// 统一入口：buildSnapshotFromTHSKline(批量路径) 和 BuildDailySnapshot(实时路径)
// 最终都调用此函数完成全部计算。
//
// 输入均为纯数据（无 DB / cache 引用），可独立单测。
func buildStockSnapshot(code string, tradeDate int, closePriceYuan float64,
	share *model.ShareChange, groups *reportGroups) model.StockDailySnapshot {

	snap := model.StockDailySnapshot{
		StockCode: code,
		TradeDate: tradeDate,
	}

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
	report := groups.findLatest(tradeDate)
	if report != nil && snap.TotalMarketCap > 0 {
		marketCap := snap.TotalMarketCap

		snap.BVPS = report.BVPS
		if report.BVPS > 0 {
			snap.PB = closePriceYuan / report.BVPS
		}

		snap.ROE = report.ROEW
		snap.ROA = report.ROA
		snap.GrossMargin = report.GrossMargin
		snap.NetMargin = report.NetMargin

		snap.BasicEPS = report.BasicEPS

		snap.ParentNetProfit = report.ParentNetProfit
		snap.DeductNetProfit = report.DeductNetProfit
		snap.TotalRevenue = report.TotalRevenue

		snap.DebtRatio = report.DebtRatio

		// 市盈率(动态)
		snap.PEDynamic = calcDynamicPE(marketCap, report)

		// 市盈率(静态)
		snap.PEStatic = calcStaticPE(marketCap, groups, tradeDate)

		// TTM
		ttm := calcTTMValues(report, groups)
		if ttm.Profit != 0 {
			snap.PETTM = marketCap / ttm.Profit
		}
		if ttm.Revenue > 0 {
			snap.PSTTM = marketCap / ttm.Revenue
		}
	}

	return snap
}
