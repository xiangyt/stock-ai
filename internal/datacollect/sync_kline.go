package datacollect

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/adapter/tencentstock"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/utils"

	"gorm.io/gorm"
)

// ========== 同步模式常量 ==========

// SyncMode 同步模式
type SyncMode string

const (
	SyncModeInit     SyncMode = "init"     // 初始化：同花顺全量拉取骨架
	SyncModeDaily    SyncMode = "daily"    // 每日增量：同花顺 GetToday 等当日/当周/当月/当年
	SyncModeFill     SyncMode = "fill"     // 补全金额：东财全量拉取补 amount=0 的记录
	SyncModeDividend SyncMode = "dividend" // 除权除息：全量刷新 OHLCV，保留成交额
)

// AllPeriods 所有支持的周期
var AllPeriods = []db.KLinePeriod{
	db.KLinePeriodDaily,
	db.KLinePeriodWeekly,
	db.KLinePeriodMonthly,
	db.KLinePeriodYearly,
}

// ========== 结果结构体 ==========

// SyncResult 单只股票同步结果
type SyncResult struct {
	Code        string         `json:"code"`
	Period      db.KLinePeriod `json:"period"`
	Mode        SyncMode       `json:"mode"`
	LatestDate  string         `json:"latest_date"`   // DB中最新日期（init/fill用）
	SourceUsed  string         `json:"source_used"`   // ths / eastmoney / none
	UpsertCount int            `json:"upsert_count"`  // 实际写入条数
	SkipNoDelta bool           `json:"skip_no_delta"` // 无需更新
	Error       error          `json:"error,omitempty"`
}

// SyncBatchResult 批量同步汇总
type SyncBatchResult struct {
	Total       int          `json:"total"`
	Success     int          `json:"success"`
	SkipNoDelta int          `json:"skip_no_delta"`
	Fail        int          `json:"fail"`
	CostSeconds float64      `json:"cost_seconds"`
	Details     []SyncResult `json:"details,omitempty"`
}

// ========== 核心服务 ==========

// SyncKLineService 多周期 K线同步服务
type SyncKLineService struct {
	registry *adapter.Registry
}

// ============================================================================
//  全局单例
// ============================================================================

var (
	syncKLineOnce     sync.Once
	syncKLineInstance *SyncKLineService
)

// GetSyncKLineService 返回 SyncKLineService 全局单例（线程安全）。
// 所有组件应通过此函数获取实例，而非各自 new。
func GetSyncKLineService() *SyncKLineService {
	syncKLineOnce.Do(func() {
		syncKLineInstance = &SyncKLineService{
			registry: adapter.GetRegistry(),
		}
	})
	return syncKLineInstance
}

// ========== 三种模式入口 ==========

// InitAllStocks 初始化模式：同花顺拉取所有周期全量骨架数据（amount=0）
// 适用场景：首次运行、历史数据缺失时批量补齐
func (s *SyncKLineService) InitAllStocks(ctx context.Context, periods []db.KLinePeriod) []SyncBatchResult {
	var results []SyncBatchResult
	for _, p := range periods {
		results = append(results, s.runBatch(ctx, p, SyncModeInit))
	}
	return results
}

// SyncDailyForAll 每日增量模式：
//
//	日K → 同花顺 GetToday 获取当天完整数据（含Amount）
//	周K/月K/年K → 对应当期聚合数据，同周期则UPDATE否则INSERT
//
// 适用场景：每天定时跑一次
func (s *SyncKLineService) SyncDailyForAll(ctx context.Context, periods []db.KLinePeriod) []SyncBatchResult {
	var results []SyncBatchResult
	for _, p := range periods {
		results = append(results, s.runBatch(ctx, p, SyncModeDaily))
	}
	return results
}

// FillMissingAmount 补全金额模式：
//
//	东财全量拉取，仅覆盖 DB 中 amount=0 的记录
//	东财不稳定，应低频调用（如每周一次），每次可限制处理数量
//
// 适用场景：逐步将同花顺骨架数据的空金额补齐
func (s *SyncKLineService) FillMissingAmount(ctx context.Context, periods []db.KLinePeriod) []SyncBatchResult {
	var results []SyncBatchResult
	for _, p := range periods {
		results = append(results, s.runBatch(ctx, p, SyncModeFill))
	}
	return results
}

// SyncDividendForAll 除权除息模式：
//
//	同花顺全量拉取，已存在记录仅更新 OHLCV 五字段，不存在则插入。
//	保留原始 amount/turnover_rate 不覆盖。
//
// 适用场景：除权除息日自动触发，或手动对指定股票批量刷新
func (s *SyncKLineService) SyncDividendForAll(ctx context.Context, periods []db.KLinePeriod) []SyncBatchResult {
	var results []SyncBatchResult
	for _, p := range periods {
		results = append(results, s.runBatch(ctx, p, SyncModeDividend))
	}
	return results
}

// SyncSingleDividend 对单只股票的指定周期执行 dividend 模式同步。
// 供外部（如除权检测 handler）按需调用，只同步命中的股票。
func (s *SyncKLineService) SyncSingleDividend(ctx context.Context, code string, period db.KLinePeriod) SyncResult {
	result := &SyncResult{Code: code, Period: period}
	return s.syncSingleDividend(ctx, code, period, result)
}

// DebugSyncSingle 调试同步单只股票的逻辑
func (s *SyncKLineService) DebugSyncSingle(ctx context.Context, periods []db.KLinePeriod, code string, mode string) error {
	if len(periods) == 0 || code == "" || mode == "" {
		return nil
	}

	stock, err := db.FindStockByCode(code)
	if err != nil {
		return err
	}

	sr := s.syncSingle(ctx, stock.Code, periods[0], SyncMode(mode))
	if sr.Error != nil {
		return sr.Error
	}
	return nil
}

// runBatch 遍历所有股票执行指定周期和模式的同步
// fill 模式下：单只失败立即终止（东财不稳定，连续失败无意义）
// init/daily 模式下：单只失败不影响其他
func (s *SyncKLineService) runBatch(ctx context.Context, period db.KLinePeriod, mode SyncMode) SyncBatchResult {
	label := db.KLineLabel(period)
	stocks := db.LoadAllStockCodes()
	batch := SyncBatchResult{Total: len(stocks)}

	if len(stocks) == 0 {
		log.Printf("[%s-%s] 数据库中没有股票数据", mode, label)
		return batch
	}

	log.Printf("[%s-%s] 开始同步 %d 只股票...", mode, label, len(stocks))
	start := time.Now()

	for i, stock := range stocks {
		sr := s.syncSingle(ctx, stock.Code, period, mode)
		if sr.Error != nil {
			batch.Fail++
			batch.Details = append(batch.Details, sr)
			log.Printf("  [%d/%d] ❌ %s (%s): %v", i+1, len(stocks), stock.Code, stock.Name, sr.Error)

			if mode == SyncModeFill {
				// fill 模式：东财不稳定，单只失败即终止整个批次
				batch.CostSeconds = time.Since(start).Seconds()
				log.Printf("[%s-%s] ⛔ 单只失败终止! 已处理=%d/%d, 成功=%d 跳过=%d 失败=%d",
					mode, label, i+1, len(stocks), batch.Success, batch.SkipNoDelta, batch.Fail)
				return batch
			}
			continue // init/daily: 继续处理下一只
		}

		batch.Details = append(batch.Details, sr)
		if sr.SkipNoDelta {
			batch.SkipNoDelta++
		} else {
			batch.Success++
		}
		if batch.Success > 50 && mode == SyncModeFill { // fill 模式：单次只处理50只，防止反爬
			batch.CostSeconds = time.Since(start).Seconds()
			log.Printf("[%s-%s] 完成前50只! 成功=%d 跳过=%d 失败=%d 耗时=%.1fs",
				mode, label, batch.Success, batch.SkipNoDelta, batch.Fail, batch.CostSeconds)
			return batch
		}
	}

	batch.CostSeconds = time.Since(start).Seconds()
	log.Printf("[%s-%s] 完成! 成功=%d 跳过=%d 失败=%d 耗时=%.1fs",
		mode, label, batch.Success, batch.SkipNoDelta, batch.Fail, batch.CostSeconds)
	return batch
}

// ========== 单只股票同步逻辑 ==========

// syncSingle 单只股票的核心调度逻辑
func (s *SyncKLineService) syncSingle(ctx context.Context, code string, period db.KLinePeriod, mode SyncMode) SyncResult {
	result := SyncResult{
		Code:   code,
		Period: period,
		Mode:   mode,
	}

	switch mode {
	case SyncModeInit:
		return s.syncSingleInit(ctx, code, period, &result)
	case SyncModeDaily:
		return s.syncSingleDaily(ctx, code, period, &result)
	case SyncModeFill:
		return s.syncSingleFill(ctx, code, period, &result)
	case SyncModeDividend:
		return s.syncSingleDividend(ctx, code, period, &result)
	default:
		result.Error = fmt.Errorf("未知的同步模式: %s", mode)
		return result
	}
}

// ---------- Init 模式 ----------

// syncSingleInit 初始化：同花顺全量骨架（amount=0）
func (s *SyncKLineService) syncSingleInit(ctx context.Context, code string, period db.KLinePeriod, result *SyncResult) SyncResult {
	// 查 DB 最新日期（不限amount）
	lastTradeDate, err := db.FindLatestKlineAny(period, code)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		result.Error = fmt.Errorf("查询失败: %w", err)
		return *result
	}

	var lastDateStr string
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("  [%s][%s] 无历史数据，拉取全量", code, db.KLineLabel(period))
	} else {
		lastDateStr = db.FormatTradeDate(lastTradeDate)
		result.LatestDate = lastDateStr
		log.Printf("  [%s][%s] 最新日期: %s", code, db.KLineLabel(period), lastDateStr)
	}

	// 同花顺全量获取
	data, fetchErr := s.fetchFullKLines(ctx, ths.AdapterName, code, period, lastDateStr)
	if fetchErr != nil {
		result.Error = fetchErr
		return *result
	}
	if len(data) == 0 {
		result.SkipNoDelta = true
		return *result
	}

	// Upsert（同花顺全量数据 amount=0，这是正常的）
	success, failed := s.upsertByPeriod(code, period, data)
	result.UpsertCount = success
	result.SourceUsed = "ths"

	if failed > 0 {
		log.Printf("  [%s][%s] ✅ upsert: 成功%d 失败%d (ths)", code, db.KLineLabel(period), success, failed)
	} else {
		log.Printf("  [%s][%s] ✅ upsert %d 条 (ths)", code, db.KLineLabel(period), success)
	}

	return *result
}

// ---------- Daily 模式 ----------
//
// 策略：以全量数据为基准，对齐截断重写 + 当期精刷
//   ① 除权除息检测 → 若当天为除权除息日且有分红，自动切换 dividend 模式
//   ② DB 最新 N 条               → 本地窗口 W
//   ③ 当期采集 + 同花顺全量采集 + 当期修正尾条  → 完整数据集 A
//   ④ A ∩ W 匹配找锚定日期 D
//   ⑤ DELETE trade_date > D 的所有记录（清除脏/过期数据）
//   ⑥ A 全量 upsert（补齐缺口 + 覆盖旧值）
//
// 这样每天都能自愈：即使前几天失败了，全量对齐也能自动修复

const dailyAlignWindow = 5 // 对齐窗口大小：DB 取最新 N 条和全量数据匹配

// ============================================================================
//  公共辅助：daily / dividend 模式共享
// ============================================================================

// fetchCurrentPeriodWithFallback 获取当期数据：腾讯优先，失败回退同花顺。
// 与 daily Step ② 和 dividend 同逻辑，消除重复。
func (s *SyncKLineService) fetchCurrentPeriodWithFallback(ctx context.Context, code string, period db.KLinePeriod) (*adapter.StockPriceDaily, error) {
	item, err := s.fetchCurrentPeriodData(ctx, tencentstock.AdapterName, code, period)
	if err != nil {
		log.Printf("  [%s][%s] 腾讯失败: %s, 尝试同花顺...", code, db.KLineLabel(period), err)
		item, err = s.fetchCurrentPeriodData(ctx, ths.AdapterName, code, period)
	}
	return item, err
}

// fetchFullDataWithCurrentMerge 同花顺全量采集 + 当期数据修正尾条。
//
// 先拉取同花顺全量 K 线，再用 currentItem 修正最新一条：
//
//	日K: 日期不同则追加，相同则覆盖
//	其他周期: 直接覆盖最后一条
func (s *SyncKLineService) fetchFullDataWithCurrentMerge(ctx context.Context, code string, period db.KLinePeriod, currentItem *adapter.StockPriceDaily) ([]adapter.StockPriceDaily, error) {
	fullData, fetchErr := s.fetchFullKLines(ctx, ths.AdapterName, code, period, "")
	if fetchErr != nil {
		return nil, fetchErr
	}
	if len(fullData) == 0 {
		return nil, fmt.Errorf("同花顺全量采集结果为空")
	}

	// 当期修正尾条
	if period == db.KLinePeriodDaily && fullData[len(fullData)-1].Date != currentItem.Date {
		fullData = append(fullData, *currentItem)
	} else {
		fullData[len(fullData)-1] = *currentItem
	}

	return fullData, nil
}

// syncSingleDaily 每日增量：全量对齐截断 + 当期精刷
func (s *SyncKLineService) syncSingleDaily(ctx context.Context, code string, period db.KLinePeriod, result *SyncResult) SyncResult {
	// 除权除息日检测：如果今天是除权除息日且有分红，切换为 dividend 模式
	if isExDividendDay(code, period) {
		log.Printf("  [%s][%s] 检测到除权除息日，切换为 dividend 模式", code, db.KLineLabel(period))
		return s.syncSingleDividend(ctx, code, period, result)
	}

	// Step ①: DB 最新 N 条
	log.Printf("  [%s][%s] Step ① 查询DB最新%d条...", code, db.KLineLabel(period), dailyAlignWindow)
	dbDates, dbErr := db.FindLatestNKlinesAny(period, code, dailyAlignWindow)
	if dbErr != nil {
		result.Error = fmt.Errorf("查询DB最新数据失败: %w", dbErr)
		return *result
	}
	log.Printf("  [%s][%s] Step ① 完成, dbDates=%v", code, db.KLineLabel(period), dbDates)

	// Step ②: 当期数据采集（腾讯优先，同花顺兜底）
	log.Printf("  [%s][%s] Step ② 调用腾讯 GetTodayData...", code, db.KLineLabel(period))
	currentItem, currErr := s.fetchCurrentPeriodWithFallback(ctx, code, period)
	if currErr != nil {
		result.Error = fmt.Errorf("当期采集失败: %w", currErr)
		return *result
	}
	log.Printf("  [%s][%s] Step ② 完成, date=%s", code, db.KLineLabel(period), currentItem.Date)
	if len(dbDates) > 0 && parseTradeDate(currentItem.Date) == dbDates[0] {
		log.Printf("  [%s][%s] 无需更新 (DB最新=%d, 当期=%s)", code, db.KLineLabel(period), dbDates[0], currentItem.Date)
		result.SourceUsed = "ths"
		return *result
	}

	// Step ③: 同花顺全量采集 + 当期修正尾条
	log.Printf("  [%s][%s] Step ③ 采集同花顺全量数据...", code, db.KLineLabel(period))
	fullData, fetchErr := s.fetchFullDataWithCurrentMerge(ctx, code, period, currentItem)
	if fetchErr != nil {
		result.Error = fmt.Errorf("同花顺全量采集失败: %w", fetchErr)
		return *result
	}

	// Step ③: 找锚定日期 — DB 尾部数据在 fullData 中能找到的最近一条
	//
	// 双指针优化：两个数组均按日期有序
	//   dbDates:     DESC [最新, ..., 最旧]          (SQL ORDER BY trade_date DESC)
	//   fullDates:   ASC  [..., 较旧, 最新]           (API 返回通常升序)
	//
	// 从两端向中间扫描：i 指向 fullDates 最新端，j 指向 dbDates 最新端
	// 找到第一个相等的日期即为锚定点。O(len(dbDates)+len(fullData)), O(1) 额外空间
	var anchorDate int
	if len(dbDates) == 0 {
		// 无历史数据，不需要截断，直接写入全量即可
		log.Printf("  [%s][%s] 无历史数据，直接写入全量", code, db.KLineLabel(period))
	} else {
		anchorDate = s.findAnchorDate(fullData, dbDates)
		result.LatestDate = db.FormatTradeDate(anchorDate)
		log.Printf("  [%s][%s] 锚定日期: %s (DB尾部%d条中匹配)", code, db.KLineLabel(period), result.LatestDate, len(dbDates))
	}

	// Step ④: 截断脏数据（删除锚定之后的所有记录）
	_, delErr := db.DeleteKlinesAfterDate(period, code, anchorDate)
	if delErr != nil {
		result.Error = fmt.Errorf("截断脏数据失败: %w", delErr)
		return *result
	}
	log.Printf("  [%s][%s] 截断完成，清除锚定日期之后的数据", code, db.KLineLabel(period))

	// Step ⑤: 只插入锚定日期 D 之后的数据（增量）
	anchorDateStr := ""
	if anchorDate > 0 {
		anchorDateStr = db.FormatTradeDate(anchorDate)
	}
	incrementalData := filterAfter(fullData, anchorDateStr)
	success, failed := s.upsertByPeriod(code, period, incrementalData)
	result.UpsertCount = success
	result.SourceUsed = "ths"
	if failed > 0 {
		log.Printf("  [%s][%s] 增量upsert: 成功%d 失败%d", code, db.KLineLabel(period), success, failed)
	}

	log.Printf("  [%s][%s] ✅ daily 完成: upsert=%d (源=%s)", code, db.KLineLabel(period), result.UpsertCount, result.SourceUsed)
	return *result
}

// ---------- Fill 模式 ----------

// syncSingleFill 补全金额：东财全量，仅覆盖 amount>0 的记录
func (s *SyncKLineService) syncSingleFill(ctx context.Context, code string, period db.KLinePeriod, result *SyncResult) SyncResult {
	// 先检查是否有缺额数据
	zeroCount, countErr := db.CountZeroAmountKlines(period, code)
	if countErr != nil {
		result.Error = fmt.Errorf("统计零金额失败: %w", countErr)
		return *result
	}

	if zeroCount == 0 {
		result.SkipNoDelta = true
		log.Printf("  [%s][%s] ⏭️  已无缺额数据，跳过", code, db.KLineLabel(period))
		return *result
	}
	log.Printf("  [%s][%s] 发现 %d 条缺额数据，开始补全...", code, db.KLineLabel(period), zeroCount)

	// 东财全量拉取
	emData, fetchErr := s.fetchFullKLines(ctx, eastmoney.AdapterName, code, period, "")
	if fetchErr != nil {
		result.Error = fetchErr
		return *result
	}
	if len(emData) == 0 {
		result.Error = fmt.Errorf("东财返回空数据")
		return *result
	}

	// 仅保留 amount>0 的记录进行 upsert
	validData := filterNonZeroAmount(emData)
	if len(validData) == 0 {
		result.SkipNoDelta = true
		log.Printf("  [%s][⚠️ %s] 东财数据也全是 amount=0", code, db.KLineLabel(period))
		return *result
	}

	success, failed := s.upsertByPeriod(code, period, validData)
	result.UpsertCount = success
	result.SourceUsed = eastmoney.AdapterName

	log.Printf("  [%s][%s] ✅ 补全完成: 有效数据%d, upsert成功%d, 失败%d",
		code, db.KLineLabel(period), len(validData), success, failed)

	return *result
}

// ---------- Dividend 模式 ----------
//
// 除权除息日全量刷新历史价格：
//   ① 获取当期数据（腾讯优先，同花顺兜底）
//   ② 同花顺全量采集 + 当期修正尾条
//   ③ 全量 upsert：已存在记录仅更新 OHLCV 五字段，不存在则插入
//
// 与 daily 模式的区别：
//   - 不做对齐截断（daily 的锚点算法），直接全量写入
//   - upsert 更新列为 OHLCV 而非全字段（保留原有 amount/turnover_rate）

// syncSingleDividend 除权除息：全量刷新 OHLCV（保留成交额）
func (s *SyncKLineService) syncSingleDividend(ctx context.Context, code string, period db.KLinePeriod, result *SyncResult) SyncResult {
	// Step ①: 当期数据采集（腾讯优先，同花顺兜底）
	log.Printf("  [%s][%s] Step ① 调用腾讯 GetTodayData...", code, db.KLineLabel(period))
	currentItem, currErr := s.fetchCurrentPeriodWithFallback(ctx, code, period)
	if currErr != nil {
		result.Error = fmt.Errorf("当期采集失败: %w", currErr)
		return *result
	}
	log.Printf("  [%s][%s] 当期数据完成, date=%s", code, db.KLineLabel(period), currentItem.Date)

	// Step ②: 同花顺全量采集 + 当期修正尾条
	log.Printf("  [%s][%s] Step ② 采集同花顺全量数据...", code, db.KLineLabel(period))
	fullData, fetchErr := s.fetchFullDataWithCurrentMerge(ctx, code, period, currentItem)
	if fetchErr != nil {
		result.Error = fmt.Errorf("同花顺全量采集失败: %w", fetchErr)
		return *result
	}
	log.Printf("  [%s][%s] 全量数据 %d 条（含当期修正）", code, db.KLineLabel(period), len(fullData))

	// Step ③: 全量 upsert（已存在仅更新 OHLCV，不存在则插入）
	success, failed := s.upsertByPeriodOHLCV(code, period, fullData)
	result.UpsertCount = success
	result.SourceUsed = "ths"
	result.Mode = SyncModeDividend

	if failed > 0 {
		log.Printf("  [%s][%s] OHLCV upsert: 成功%d 失败%d", code, db.KLineLabel(period), success, failed)
	}
	log.Printf("  [%s][%s] ✅ dividend 完成: upsert=%d (源=%s)", code, db.KLineLabel(period), result.UpsertCount, result.SourceUsed)
	return *result
}

// ========== 内部方法：采集器调用 ==========

// fetchFullKLines 从指定采集器获取某周期的全量 K 线数据并按日期过滤
func (s *SyncKLineService) fetchFullKLines(ctx context.Context, provider, code string, period db.KLinePeriod, afterDate string) ([]adapter.StockPriceDaily, error) {
	ds, ok := s.registry.Get(provider)
	if !ok {
		return nil, fmt.Errorf("数据源未注册: %s", provider)
	}

	allData, err := s.callKLineAPI(ctx, ds, code, period)
	if err != nil {
		return nil, err
	}
	if len(allData) == 0 {
		return nil, fmt.Errorf("%s 返回空数据", ds.DisplayName())
	}

	if afterDate == "" {
		return allData, nil
	}

	return filterAfter(allData, afterDate), nil
}

// fetchCurrentPeriodData 获取当前周期的一条聚合数据（用于 daily 模式）
func (s *SyncKLineService) fetchCurrentPeriodData(ctx context.Context, provider, code string, period db.KLinePeriod) (*adapter.StockPriceDaily, error) {
	ds, ok := s.registry.Get(provider)
	if !ok {
		return nil, fmt.Errorf("数据源未注册: %s", provider)
	}

	var (
		item *adapter.StockPriceDaily
		err  error
	)

	switch period {
	case db.KLinePeriodDaily:
		item, err = ds.GetTodayData(ctx, code)
	case db.KLinePeriodWeekly:
		item, err = ds.GetThisWeekData(ctx, code)
	case db.KLinePeriodMonthly:
		item, err = ds.GetThisMonthData(ctx, code)
	case db.KLinePeriodYearly:
		item, err = ds.GetThisYearData(ctx, code)
	default:
		return nil, fmt.Errorf("不支持该周期的当日数据: %s", period)
	}

	if err != nil {
		return nil, fmt.Errorf("%s 获取%s数据失败: %w", ds.DisplayName(), db.KLineLabel(period), err)
	}
	if item == nil {
		return nil, fmt.Errorf("%s 返回空%s数据", ds.DisplayName(), db.KLineLabel(period))
	}

	return item, nil
}

// callKLineAPI 根据周期调用对应的 GetXxxKLine 方法
func (s *SyncKLineService) callKLineAPI(ctx context.Context, ds adapter.DataSource, code string, period db.KLinePeriod) ([]adapter.StockPriceDaily, error) {
	switch period {
	case db.KLinePeriodDaily:
		return ds.GetDailyKLine(ctx, code, adapter.AdjQFQ)
	case db.KLinePeriodWeekly:
		return ds.GetWeeklyKLine(ctx, code, adapter.AdjQFQ)
	case db.KLinePeriodMonthly:
		return ds.GetMonthlyKLine(ctx, code, adapter.AdjQFQ)
	case db.KLinePeriodYearly:
		return ds.GetYearlyKLine(ctx, code, adapter.AdjQFQ)
	default:
		return nil, fmt.Errorf("不支持的周期: %s", period)
	}
}

// ========== 内部方法：数据转换与写入 ==========

// upsertByPeriod 根据周期选择对应的 DAO 进行批量写入
// 遇到第一条写入失败立即停止，后续全部标记为失败，等待下次同步刷入
// 这样保证已写入的数据是连续的，不会产生数据空洞
func (s *SyncKLineService) upsertByPeriod(code string, period db.KLinePeriod, data []adapter.StockPriceDaily) (int, int) {
	success, failed := 0, 0

	for _, item := range data {
		td := parseTradeDate(item.Date)
		if td == 0 {
			failed++
			continue
		}

		rows := s.upsertOne(code, period, td, item)
		if rows < 0 {
			failed++
			break // 首次失败立即停止，后续数据留待下次同步
		}
		success++
	}

	// 如果中途失败，剩余未处理的全算作待重试
	if len(data) > success+failed {
		pending := len(data) - success - failed
		failed += pending
	}

	return success, failed
}

// upsertOne 写入单条 K 线到对应周期的表
func (s *SyncKLineService) upsertOne(code string, period db.KLinePeriod, tradeDate int, item adapter.StockPriceDaily) int64 {
	m := model.DailyKline{
		StockCode:    code,
		TradeDate:    tradeDate,
		Open:         int(item.Open),
		High:         int(item.High),
		Low:          int(item.Low),
		Close:        int(item.Close),
		Volume:       item.Volume,
		Amount:       item.Amount,
		TurnoverRate: item.Turnover,
	}

	switch period {
	case db.KLinePeriodDaily:
		return db.UpsertDailyKline(m)
	case db.KLinePeriodWeekly:
		wm := model.WeeklyKline{
			StockCode: m.StockCode, TradeDate: m.TradeDate,
			Open: m.Open, High: m.High, Low: m.Low, Close: m.Close,
			Volume: m.Volume, Amount: m.Amount, TurnoverRate: m.TurnoverRate,
		}
		return db.UpsertWeeklyKline(wm)
	case db.KLinePeriodMonthly:
		mm := model.MonthlyKline{
			StockCode: m.StockCode, TradeDate: m.TradeDate,
			Open: m.Open, High: m.High, Low: m.Low, Close: m.Close,
			Volume: m.Volume, Amount: m.Amount, TurnoverRate: m.TurnoverRate,
		}
		return db.UpsertMonthlyKline(mm)
	case db.KLinePeriodYearly:
		ym := model.YearlyKline{
			StockCode: m.StockCode, TradeDate: m.TradeDate,
			Open: m.Open, High: m.High, Low: m.Low, Close: m.Close,
			Volume: m.Volume, Amount: m.Amount, TurnoverRate: m.TurnoverRate,
		}
		return db.UpsertYearlyKline(ym)
	default:
		return -1
	}
}

// ========== Dividend 模式辅助 ==========

// isExDividendDay 检测本周期是否包含该股票的除权除息日（且有分红）。
// 仅对日K周期有效，其他周期始终返回 false。
func isExDividendDay(code string, period db.KLinePeriod) bool {
	// 查该股票最新一次除权除息记录（按 ex_dividend_date 降序）
	dividend, err := db.FindLatestDividend(code)
	if err != nil {
		return false
	}

	// 查询结果已限 is_unassign=false + 除权除息日与今天同一周期
	return db.IsSamePeriod(period, utils.TodayTradeDate(), dividend.ExDividendDate)
}

// upsertByPeriodOHLCV 根据周期选用 OHLCV-only upsert 进行批量写入。
// 已存在记录仅更新 open/high/low/close/volume，不存在则全字段插入。
// 其余行为与 upsertByPeriod 一致（首次失败即停止）。
func (s *SyncKLineService) upsertByPeriodOHLCV(code string, period db.KLinePeriod, data []adapter.StockPriceDaily) (int, int) {
	success, failed := 0, 0

	for _, item := range data {
		td := parseTradeDate(item.Date)
		if td == 0 {
			failed++
			continue
		}

		rows := s.upsertOneOHLCV(code, period, td, item)
		if rows < 0 {
			failed++
			break // 首次失败立即停止
		}
		success++
	}

	if len(data) > success+failed {
		pending := len(data) - success - failed
		failed += pending
	}

	return success, failed
}

// upsertOneOHLCV 写入单条 K 线（OHLCV-only 更新）
func (s *SyncKLineService) upsertOneOHLCV(code string, period db.KLinePeriod, tradeDate int, item adapter.StockPriceDaily) int64 {
	m := model.DailyKline{
		StockCode:    code,
		TradeDate:    tradeDate,
		Open:         int(item.Open),
		High:         int(item.High),
		Low:          int(item.Low),
		Close:        int(item.Close),
		Volume:       item.Volume,
		Amount:       item.Amount,
		TurnoverRate: item.Turnover,
	}

	switch period {
	case db.KLinePeriodDaily:
		return db.UpsertDailyKlineOHLCV(m)
	case db.KLinePeriodWeekly:
		wm := model.WeeklyKline{
			StockCode: m.StockCode, TradeDate: m.TradeDate,
			Open: m.Open, High: m.High, Low: m.Low, Close: m.Close,
			Volume: m.Volume, Amount: m.Amount, TurnoverRate: m.TurnoverRate,
		}
		return db.UpsertWeeklyKlineOHLCV(wm)
	case db.KLinePeriodMonthly:
		mm := model.MonthlyKline{
			StockCode: m.StockCode, TradeDate: m.TradeDate,
			Open: m.Open, High: m.High, Low: m.Low, Close: m.Close,
			Volume: m.Volume, Amount: m.Amount, TurnoverRate: m.TurnoverRate,
		}
		return db.UpsertMonthlyKlineOHLCV(mm)
	case db.KLinePeriodYearly:
		ym := model.YearlyKline{
			StockCode: m.StockCode, TradeDate: m.TradeDate,
			Open: m.Open, High: m.High, Low: m.Low, Close: m.Close,
			Volume: m.Volume, Amount: m.Amount, TurnoverRate: m.TurnoverRate,
		}
		return db.UpsertYearlyKlineOHLCV(ym)
	default:
		return -1
	}
}

// findAnchorDate 双指针找锚定日期（有序数组匹配）
//
// fullData: API 返回的全量数据，按 trade_date ASC 排列
// dbDates: DB 最新 N 条，按 trade_date DESC 排列
//
// 算法：
//
//	i 从 fullData 末尾(最新)向左扫描
//	j 从 dbDates 头部(最新)向右扫描
//	因为两者都指向最新端，找到第一个相等的日期即为锚定点
func (s *SyncKLineService) findAnchorDate(fullData []adapter.StockPriceDaily, dbDates []int) int {
	// 预解析 fullData 的日期为整数数组，避免循环内重复 parse
	n := len(fullData)
	fullDates := make([]int, n)
	for idx, item := range fullData {
		fullDates[idx] = parseTradeDate(item.Date)
	}

	i := n - 1 // fullDates 末端(最新)
	j := 0     // dbDates 头端(最新)

	for i >= 0 && j < len(dbDates) {
		if fullDates[i] == dbDates[j] {
			return fullDates[i] // 锚定命中
		}
		if fullDates[i] > dbDates[j] {
			// API 数据比 DB 这条更新 → 看 API 更早的记录能否匹配
			i--
		} else {
			// API 数据比 DB 这条更旧 → 看 DB 更早的记录
			j++
		}
	}

	return 0 // 无匹配
}

// ========== 内部辅助函数 ==========

// filterAfter 过滤出 dateStr 之后的数据
func filterAfter(data []adapter.StockPriceDaily, dateStr string) []adapter.StockPriceDaily {
	result := make([]adapter.StockPriceDaily, 0)
	for _, d := range data {
		if d.Date > dateStr {
			result = append(result, d)
		}
	}
	return result
}

// filterNonZeroAmount 过滤掉 amount=0 的记录（fill 模式专用）
func filterNonZeroAmount(data []adapter.StockPriceDaily) []adapter.StockPriceDaily {
	result := make([]adapter.StockPriceDaily, 0)
	for _, d := range data {
		if d.Amount > 0 {
			result = append(result, d)
		}
	}
	return result
}

// parseInt 安全解析正整数字符串
func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
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
