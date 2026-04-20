package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"gorm.io/gorm"
)

// ========== 同步模式常量 ==========

// SyncMode 同步模式
type SyncMode string

const (
	SyncModeInit  SyncMode = "init" // 初始化：同花顺全量拉取骨架
	SyncModeDaily SyncMode = "daily" // 每日增量：同花顺 GetToday 等当日/当周/当月/当年
	SyncModeFill  SyncMode = "fill" // 补全金额：东财全量拉取补 amount=0 的记录
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
	Code          string       `json:"code"`
	Period        db.KLinePeriod `json:"period"`
	Mode          SyncMode     `json:"mode"`
	LatestDate    string       `json:"latest_date"`    // DB中最新日期（init/fill用）
	SourceUsed    string       `json:"source_used"`    // ths / eastmoney / none
	UpsertCount   int          `json:"upsert_count"`   // 实际写入条数
	SkipNoDelta   bool         `json:"skip_no_delta"`  // 无需更新
	Error         error        `json:"error,omitempty"`
}

// SyncBatchResult 批量同步汇总
type SyncBatchResult struct {
	Total       int         `json:"total"`
	Success     int         `json:"success"`
	SkipNoDelta int         `json:"skip_no_delta"`
	Fail        int         `json:"fail"`
	CostSeconds float64     `json:"cost_seconds"`
	Details     []SyncResult `json:"details,omitempty"`
}

// ========== 核心服务 ==========

// SyncKLineService 多周期 K线同步服务
type SyncKLineService struct {
	registry *adapter.Registry
}

func NewSyncKLineService() *SyncKLineService {
	return &SyncKLineService{
		registry: adapter.GetRegistry(),
	}
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
//   日K → 同花顺 GetToday 获取当天完整数据（含Amount）
//   周K/月K/年K → 对应当期聚合数据，同周期则UPDATE否则INSERT
// 适用场景：每天定时跑一次
func (s *SyncKLineService) SyncDailyForAll(ctx context.Context, periods []db.KLinePeriod) []SyncBatchResult {
	var results []SyncBatchResult
	for _, p := range periods {
		results = append(results, s.runBatch(ctx, p, SyncModeDaily))
	}
	return results
}

// FillMissingAmount 补全金额模式：
//   东财全量拉取，仅覆盖 DB 中 amount=0 的记录
//   东财不稳定，应低频调用（如每周一次），每次可限制处理数量
// 适用场景：逐步将同花顺骨架数据的空金额补齐
func (s *SyncKLineService) FillMissingAmount(ctx context.Context, periods []db.KLinePeriod) []SyncBatchResult {
	var results []SyncBatchResult
	for _, p := range periods {
		results = append(results, s.runBatch(ctx, p, SyncModeFill))
	}
	return results
}

// runBatch 遍历所有股票执行指定周期和模式的同步
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
			log.Printf("  [%d/%d] ❌ %s (%s): %v", i+1, len(stocks), stock.Code, stock.Name, sr.Error)
		} else if sr.SkipNoDelta {
			batch.SkipNoDelta++
		} else {
			batch.Success++
		}
		batch.Details = append(batch.Details, sr)
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
	data, fetchErr := s.fetchFullKLines(ctx, "ths", code, period, lastDateStr)
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
//   ① 同花顺全量采集 → 完整数据集 A
//   ② DB 最新 N 条      → 本地窗口 W（默认取尾部10条做匹配）
//   ③ A ∩ W 匹配找锚定日期 D（W 中能在 A 里找到对应的那条）
//   ④ DELETE trade_date > D 的所有记录（清除脏/过期数据）
//   ⑤ A 全量 upsert（补齐缺口 + 覆盖旧值）
//   ⑥ GetToday/ThisWeek... 精刷当期（拿到含 Amount 的完整当日数据）
//
// 这样每天都能自愈：即使前几天失败了，全量对齐也能自动修复

const dailyAlignWindow = 10 // 对齐窗口大小：DB 取最新 N 条和全量数据匹配

// syncSingleDaily 每日增量：全量对齐截断 + 当期精刷
func (s *SyncKLineService) syncSingleDaily(ctx context.Context, code string, period db.KLinePeriod, result *SyncResult) SyncResult {
	// Step ①: 同花顺全量采集
	fullData, fetchErr := s.fetchFullKLines(ctx, "ths", code, period, "")
	if fetchErr != nil {
		result.Error = fmt.Errorf("同花顺全量采集失败: %w", fetchErr)
		return *result
	}

	// Step ②: DB 最新 N 条
	dbDates, dbErr := db.FindLatestNKlinesAny(period, code, dailyAlignWindow)

	// Step ③: 找锚定日期 — DB 尾部数据在 fullData 中能找到的最近一条
	//
	// 双指针优化：两个数组均按日期有序
	//   dbDates:     DESC [最新, ..., 最旧]          (SQL ORDER BY trade_date DESC)
	//   fullDates:   ASC  [..., 较旧, 最新]           (API 返回通常升序)
	//
	// 从两端向中间扫描：i 指向 fullDates 最新端，j 指向 dbDates 最新端
	// 找到第一个相等的日期即为锚定点。O(len(dbDates)+len(fullData)), O(1) 额外空间
	var anchorDate int
	if dbErr == nil && len(dbDates) > 0 && len(fullData) > 0 {
		anchorDate = s.findAnchorDate(fullData, dbDates)
		result.LatestDate = db.FormatTradeDate(anchorDate)
		log.Printf("  [%s][%s] 锚定日期: %s (DB尾部%d条中匹配)", code, db.KLineLabel(period), result.LatestDate, len(dbDates))
	} else if errors.Is(dbErr, gorm.ErrRecordNotFound) || len(dbDates) == 0 {
		// 无历史数据，不需要截断，直接写入全量即可
		log.Printf("  [%s][%s] 无历史数据，直接写入全量", code, db.KLineLabel(period))
	} else {
		result.Error = fmt.Errorf("查询DB最新数据失败: %w", dbErr)
		return *result
	}

	// Step ④: 截断脏数据（删除锚定之后的所有记录）
	if anchorDate > 0 {
		_, delErr := db.DeleteKlinesAfterDate(period, code, anchorDate)
		if delErr != nil {
			result.Error = fmt.Errorf("截断脏数据失败: %w", delErr)
			return *result
		}
		log.Printf("  [%s][%s] 截断完成，清除锚定日期之后的数据", code, db.KLineLabel(period))
	}

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

	// Step ⑥: 精刷当期（GetToday/ThisWeek/ThisMonth/ThisYear）— 获取含 Amount 的精确当期数据
	currentItem, currErr := s.fetchCurrentPeriodData(ctx, "ths", code, period)
	if currErr != nil {
		// 当期精刷失败不阻塞主流程，全量数据已经写入了
		log.Printf("  [%s][%s] ⚠️ 当期精刷失败(不影响全量): %v", code, db.KLineLabel(period), currErr)
	} else if currentItem != nil {
		currSuccess, _ := s.upsertByPeriod(code, period, []adapter.StockPriceDaily{*currentItem})
		if currSuccess > 0 {
			log.Printf("  [%s][%s] 当期精刷完成 (trade_date=%s)", code, db.KLineLabel(period), currentItem.Date)
		}
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
	emData, fetchErr := s.fetchFullKLines(ctx, "eastmoney", code, period, "")
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
	result.SourceUsed = "eastmoney"

	log.Printf("  [%s][%s] ✅ 补全完成: 有效数据%d, upsert成功%d, 失败%d",
		code, db.KLineLabel(period), len(validData), success, failed)

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

// findAnchorDate 双指针找锚定日期（有序数组匹配）
//
// fullData: API 返回的全量数据，按 trade_date ASC 排列
// dbDates: DB 最新 N 条，按 trade_date DESC 排列
//
// 算法：
//   i 从 fullData 末尾(最新)向左扫描
//   j 从 dbDates 头部(最新)向右扫描
//   因为两者都指向最新端，找到第一个相等的日期即为锚定点
//   如果 fullData[i] > dbDates[j] → 说明 API 比 DB 更新，j++ 尝试更早的 DB 记录
//   如果 fullData[i] < dbDates[j] → 不可能发生(DB最新不可能比API全量最新还新)，i--
//   相等 → 找到锚定！
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
			// API 数据比 DB 这条更新 → 看 DB 更早的记录能否匹配
			j++
		} else {
			// API 数据比 DB 这条更旧 → 看 API 更早的记录
			i--
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

// parseInt 安全解析正整数字符串（复用 collect_kline 的风格）
// 注意: parseTradeDate 已在 collect_kline.go 中定义，此处直接引用
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
