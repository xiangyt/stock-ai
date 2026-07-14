package datacollect

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/notifier"
	"stock-ai/internal/subscription/watchlist"
	"stock-ai/utils"

	"github.com/robfig/cron/v3"
)

// ============================================================================
//  变更事件
// ============================================================================

// ChangeType 任务变更事件类型
type ChangeType string

const (
	ChangeUpdated  ChangeType = "updated"
	ChangeEnabled  ChangeType = "enabled"
	ChangeDisabled ChangeType = "disabled"
)

// TaskChange 任务变更事件
type TaskChange struct {
	Type   ChangeType
	TaskID uint
}

// ============================================================================
//  Scheduler 接口
// ============================================================================

// Scheduler 数据采集调度引擎接口
type Scheduler interface {
	// Start 启动调度引擎
	Start() error

	// Stop 停止调度引擎
	Stop()

	// NotifyChange 接收任务变更通知（由 Service 层调用）
	NotifyChange(change TaskChange)
}

// ============================================================================
//  TaskRunner — 任务执行器（按 TaskID 分发到具体 Handler）
// ============================================================================

// TaskResult 统一的任务执行结果，包含摘要文本和失败股票代码列表。
//
// Summary 为成功或部分成功的执行摘要文本；
// FailedCodes 记录本次执行中具体失败的股票代码（用于企微通知）。
type TaskResult struct {
	Summary     string   // 执行结果摘要文本
	FailedCodes []string // 失败的股票代码列表（无失败时为 nil）
}

// TaskHandler 单个采集任务的处理函数。
// 返回 TaskResult（含摘要和失败代码列表），error 表示整个任务是否异常终止。
type TaskHandler func(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error)

// TaskRunner 任务执行器接口（策略模式，方便测试时替换）
type TaskRunner interface {
	Run(ctx context.Context, task *model.DataCollectTask, bots []model.PushBot) error
}

// DataCollectRunner 数据采集运行器，按内置任务 ID 分发到对应的 TaskHandler。
// 执行完成后，通过关联的机器人推送执行结果通知。
type DataCollectRunner struct {
	handlers     map[uint]TaskHandler  // taskID → 处理函数
	notifier     *subscriptionNotifier // 机器人推送封装
	watchlistMgr *watchlist.Manager    // 关注列表管理器（用于高优先级任务，可选）
}

// subscriptionNotifier 对 notifier 包的轻量封装，仅暴露机器人推送能力。
type subscriptionNotifier struct {
	ntf notifier.Notifier
}

func (s *subscriptionNotifier) Push(ctx context.Context, bots []model.PushBot, message string) {
	for _, bot := range bots {
		if bot.Status == 0 {
			continue
		}
		if err := s.ntf.Send(ctx, &bot, message); err != nil {
			log.Printf("[DataCollect] 推送失败 bot=%d(%s): %v", bot.ID, bot.Name, err)
		}
	}
}

// NewDataCollectRunner 创建数据采集运行器，注册所有任务 Handler。
// mgr 为 nil 时高优先级任务不可用。
func NewDataCollectRunner(mgr *watchlist.Manager) *DataCollectRunner {
	r := &DataCollectRunner{
		handlers:     make(map[uint]TaskHandler),
		notifier:     &subscriptionNotifier{ntf: notifier.NewNotifier()},
		watchlistMgr: mgr,
	}

	// === 注册内置任务 Handler ===

	// Task 1: 股票列表同步 → CollectStockList
	r.handlers[model.TaskStockDetailSync] = r.handleStockDetailSync

	// Task 2: 每日增量同步K线数据 → SyncDailyForAll
	r.handlers[model.TaskDailyKlineIncSync] = r.handleDailyKlineInc

	// Task 3: 东财数据全量补全 → FillMissingAmount
	r.handlers[model.TaskEastmoneyFullSync] = r.handleEastmoneyFull

	// Task 4: 财报同步 → CollectPerformanceReportsBatch
	r.handlers[model.TaskFinanceReportSync] = r.handleFinanceReportSync

	// Task 5: 股本变动同步 → CollectShareChangesBatch
	r.handlers[model.TaskShareChangeSync] = r.handleShareChangeSync

	// Task 6: 股东户数同步 → CollectShareholderCountsBatch
	r.handlers[model.TaskShareholderSync] = r.handleShareholderSync

	// Task 7: 每日快照计算 → calcAllStocksAllDates
	r.handlers[model.TaskDailySnapshotSync] = r.handleDailySnapshotSync

	// Task 8: 每周增量同步K线数据（weekly）
	r.handlers[model.TaskWeeklyKlineIncSync] = r.handleWeeklyKlineInc

	// Task 9: 每月增量同步K线数据（monthly）
	r.handlers[model.TaskMonthlyKlineIncSync] = r.handleMonthlyKlineInc

	// Task 10: 分红历史同步 → RunDividendHistoryBatch
	r.handlers[model.TaskDividendSync] = r.handleDividendSync

	// Task 11: 除权K线同步（日/周/月）→ 检查除权日后 dividend 模式刷新
	r.handlers[model.TaskDividendKlineSync] = r.handleDividendKlineSync

	// Task 12: 名称变更同步 → RunNameChangesBatch
	r.handlers[model.TaskNameChangeSync] = r.handleNameChangeSync

	// Task 13: 重置行情缓存高优先级
	r.handlers[model.TaskReloadHighPriority] = r.handleReloadHighPriority

	return r
}

// Run 执行采集任务：按 TaskID 分发执行 → 推送结果到关联机器人
func (r *DataCollectRunner) Run(ctx context.Context, task *model.DataCollectTask, bots []model.PushBot) error {
	handler, ok := r.handlers[task.ID]
	if !ok {
		return fmt.Errorf("未知任务 %d(%s)，无对应 Handler", task.ID, task.Name)
	}

	result, err := handler(ctx, task)
	if err != nil {
		msg := r.formatFailureMessage(task.Name, err, result)
		log.Printf("[DataCollect] 任务 %d(%s) 执行失败: %v", task.ID, task.Name, err)
		if len(bots) > 0 {
			r.notifier.Push(ctx, bots, msg)
		}
		return err
	}

	log.Printf("[DataCollect] 任务 %d(%s) 执行成功: %s", task.ID, task.Name, result.Summary)
	if len(bots) > 0 && result.Summary != "" {
		r.notifier.Push(ctx, bots, r.formatSuccessMessage(task.Name, result))
	}
	return nil
}

// formatSuccessMessage 格式化成功通知（含部分失败场景）。
//
// 当存在失败股票时，在成功摘要下方追加失败代码列表。
func (r *DataCollectRunner) formatSuccessMessage(taskName string, result *TaskResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "✅ [%s] 执行完成\n\n%s", taskName, result.Summary)

	if len(result.FailedCodes) > 0 {
		fmt.Fprintf(&sb, "\n%s", formatFailedCodes(result.FailedCodes))
	}
	return sb.String()
}

// formatFailureMessage 格式化失败通知，包含错误原因和可用的失败股票代码。
func (r *DataCollectRunner) formatFailureMessage(taskName string, err error, result *TaskResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "❌ [%s] 执行失败\n\n错误: %v", taskName, err)

	if result != nil && len(result.FailedCodes) > 0 {
		fmt.Fprintf(&sb, "\n%s", formatFailedCodes(result.FailedCodes))
	}
	return sb.String()
}

// maxFailedCodesInMsg 企微消息中展示的最大失败股票数量（超出则截断）。
const maxFailedCodesInMsg = 30

// formatFailedCodes 将失败股票代码列表格式化为文本行。
// 超过 maxFailedCodesInMsg 时截断并标注总数。
func formatFailedCodes(codes []string) string {
	n := len(codes)
	if n == 0 {
		return ""
	}

	header := "失败股票:"
	if n <= maxFailedCodesInMsg {
		return fmt.Sprintf("%s %s", header, strings.Join(codes, " "))
	}
	truncated := codes[:maxFailedCodesInMsg]
	return fmt.Sprintf("%s %s ...共%d只", header, strings.Join(truncated, " "), n)
}

// ============================================================================
//  Task Handler 实现
// ============================================================================

// handleStockDetailSync 处理【股票列表同步】(Task 1)
// 参数 JSON: {"source":"tencentstock"} — 可选，默认按优先级自动选源
// 流程: 数据源 → 获取全量股票列表 → 遍历获取详情 → upsert入库
func (r *DataCollectRunner) handleStockDetailSync(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	sourceName := parseSourceFromParams(task.Params)
	registry := adapter.GetRegistry()

	adp, err := ResolveAdapter(registry, sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	log.Printf("[DataCollect] 开始股票列表同步, source=%s", adp.Name())

	allStocks, err := adp.GetStockList(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取股票列表失败: %w", err)
	}

	total := len(allStocks)
	newCount, updCount, failCount := 0, 0, 0
	var failCodes []string

	for i, stock := range allStocks {
		detail, detailErr := adp.GetStockDetail(ctx, stock.Code)
		if detailErr != nil {
			log.Printf("[DataCollect] 获取详情失败 [%s]: %v", stock.Code, detailErr)
			failCount++
			failCodes = append(failCodes, stock.Code)
			continue
		}

		s := ToStockModel(stock.Code, detail)
		// 保留退市日期：优先从 name_changes 获取，否则保留 DB 中已有值
		s.DelistDate = getDelistDate(stock.Code)
		if db.UpsertStock(s) == 0 {
			newCount++
		} else {
			updCount++
		}

		if (i+1)%100 == 0 || i == total-1 {
			log.Printf("[DataCollect] 详情进度: %d/%d (新增=%d, 更新=%d)", i+1, total, newCount, updCount)
		}
	}

	result := &TaskResult{
		Summary: formatStockSummary(total, newCount, updCount, failCount),
		FailedCodes: failCodes,
	}
	return result, nil
}

// handleDailyKlineInc 处理【每日增量同步K线数据】(Task 2)
// 参数 JSON: {"periods":"daily"} — 可选，默认 daily
func (r *DataCollectRunner) handleDailyKlineInc(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	periods := parsePeriodsFromParams(task.Params)
	results := GetSyncKLineService().SyncDailyForAll(ctx, periods)
	return formatSyncResults(periods, results), nil
}

// handleEastmoneyFull 处理【东财数据全量补全】(Task 3)
// 参数 JSON: {"periods":"daily"} — 可选，默认 daily
// 避开三个增量同步时间点：日线(周一至五 18:00)、周线(周六 01:00)、月线(每月1号 01:30)
func (r *DataCollectRunner) handleEastmoneyFull(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	now := time.Now()
	if utils.IsTradingDay() && now.Hour() > 6 && now.Hour() < 16 {
		return &TaskResult{Summary: "交易时段跳过"}, nil
	}
	// 避开日线增量同步 (周一至五 18:00)
	if now.Hour() == 18 && now.Minute() == 0 && now.Weekday() >= time.Monday && now.Weekday() <= time.Friday {
		return &TaskResult{Summary: "避开日线增量时段，跳过"}, nil
	}
	// 避开周线增量同步 (周六 01:00)
	if now.Hour() == 1 && now.Minute() == 0 && now.Weekday() == time.Saturday {
		return &TaskResult{Summary: "避开周线增量时段，跳过"}, nil
	}
	// 避开月线增量同步 (每月 1 号 01:30)
	if now.Day() == 1 && now.Hour() == 1 && now.Minute() == 30 {
		return &TaskResult{Summary: "避开月线增量时段，跳过"}, nil
	}
	periods := parsePeriodsFromParams(task.Params)
	results := GetSyncKLineService().FillMissingAmount(ctx, periods)
	return formatSyncResults(periods, results), nil
}

// handleFinanceReportSync 处理【财报同步】(Task 4)
// 参数 JSON: {"source":"tencentstock"} — 可选，默认按优先级自动选源
func (r *DataCollectRunner) handleFinanceReportSync(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	sourceName := parseSourceFromParams(task.Params)
	adp, err := ResolveAdapter(adapter.GetRegistry(), sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	res, err := RunPerformanceReportsBatch(ctx, adp)
	if err != nil {
		return nil, err
	}
	return &TaskResult{
		Summary:     formatCollectSummary(res.Total, res.NewCount, res.UpdCount, res.FailCount),
		FailedCodes: res.FailedCodes,
	}, nil
}

// handleShareChangeSync 处理【股本变动同步】(Task 5)
func (r *DataCollectRunner) handleShareChangeSync(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	sourceName := parseSourceFromParams(task.Params)
	adp, err := ResolveAdapter(adapter.GetRegistry(), sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	res, err := RunShareChangesBatch(ctx, adp)
	if err != nil {
		return nil, err
	}
	return &TaskResult{
		Summary:     formatCollectSummary(res.Total, res.NewCount, res.UpdCount, res.FailCount),
		FailedCodes: res.FailedCodes,
	}, nil
}

// handleShareholderSync 处理【股东户数同步】(Task 6)
func (r *DataCollectRunner) handleShareholderSync(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	sourceName := parseSourceFromParams(task.Params)
	adp, err := ResolveAdapter(adapter.GetRegistry(), sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	res, err := RunShareholderCountsBatch(ctx, adp)
	if err != nil {
		return nil, err
	}
	return &TaskResult{
		Summary:     formatCollectSummary(res.Total, res.NewCount, res.UpdCount, res.FailCount),
		FailedCodes: res.FailedCodes,
	}, nil
}

// handleDailySnapshotSync 处理【每日快照计算】(Task 7)
func (r *DataCollectRunner) handleDailySnapshotSync(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	res := GetSnapshotService().calcAllStocksAllDates(ctx)
	return &TaskResult{
		Summary: fmt.Sprintf(
			"  扫描股票 : %s    成功 : %s    失败 : %s\n  快照写入 : 成功=%s条 失败=%s条\n  耗时     : %.1fs",
			formatInt(res.TotalStocks), formatInt(res.SuccessStocks), formatInt(res.FailStocks),
			formatInt(res.SuccessSnapshots), formatInt(res.FailSnapshots),
			res.CostSeconds,
		),
		FailedCodes: res.FailedCodes,
	}, nil
}

// handleWeeklyKlineInc 处理【每周增量同步K线数据】(Task 8)
// cron: 0 0 1 * * 6（每周六凌晨 1:00）
// 参数 JSON: {"periods":"weekly"} — 固定 weekly，params 中的 periods 字段将被忽略
func (r *DataCollectRunner) handleWeeklyKlineInc(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	results := GetSyncKLineService().SyncDailyForAll(ctx, []db.KLinePeriod{db.KLinePeriodWeekly})
	return formatSyncResults([]db.KLinePeriod{db.KLinePeriodWeekly}, results), nil
}

// handleMonthlyKlineInc 处理【每月增量同步K线数据】(Task 9)
// cron: 0 30 1 1 * ?（每月 1 号凌晨 1:30）
func (r *DataCollectRunner) handleMonthlyKlineInc(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	results := GetSyncKLineService().SyncDailyForAll(ctx, []db.KLinePeriod{db.KLinePeriodMonthly})
	return formatSyncResults([]db.KLinePeriod{db.KLinePeriodMonthly}, results), nil
}

// handleDividendSync 处理【分红历史同步】(Task 10)
func (r *DataCollectRunner) handleDividendSync(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	sourceName := parseSourceFromParams(task.Params)
	adp, err := ResolveAdapter(adapter.GetRegistry(), sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	res, err := RunDividendHistoryBatch(ctx, adp)
	if err != nil {
		return nil, err
	}
	return &TaskResult{
		Summary:     formatCollectSummary(res.Total, res.NewCount, res.UpdCount, res.FailCount),
		FailedCodes: res.FailedCodes,
	}, nil
}

// handleDividendKlineSync 处理【除权K线同步】(Task 11)
// 遍历所有股票，对每只股票检查最新除权日是否与今天在同一周期（日/周/月）。
// 若在同一周期，则以 dividend 模式同步该股票的对应周期 K 线数据。
// 仅处理日/周/月三个周期。
func (r *DataCollectRunner) handleDividendKlineSync(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	periods := parseDividendPeriods(task.Params)
	stocks := db.LoadAllStockCodes()
	today := utils.TodayTradeDate()

	type dividendHit struct {
		code   string
		period db.KLinePeriod
	}
	var hits []dividendHit

	// 第一遍：收集所有需要除权同步的 (股票, 周期) 组合
	for _, stock := range stocks {
		dividend, err := db.FindLatestDividend(stock.Code)
		if err != nil {
			continue
		}
		for _, p := range periods {
			if db.IsSamePeriod(p, today, dividend.ExDividendDate) {
				hits = append(hits, dividendHit{code: stock.Code, period: p})
				log.Printf("[DataCollect] 除权检测: %s(%s) 除权日=%d, 今日=%d, 周期=%s → 触发同步",
					stock.Code, stock.Name, dividend.ExDividendDate, today, db.KLineLabel(p))
			}
		}
	}

	if len(hits) == 0 {
		return &TaskResult{Summary: "无股票处于除权周期，跳过"}, nil
	}

	// 第二遍：执行 dividend 模式同步
	svc := GetSyncKLineService()
	success, fail := 0, 0
	var failCodes []string
	for _, h := range hits {
		sr := svc.SyncSingleDividend(ctx, h.code, h.period)
		if sr.Error != nil {
			fail++
			failCodes = append(failCodes, h.code)
			log.Printf("[DataCollect] 除权同步失败 %s(%s): %v", h.code, db.KLineLabel(h.period), sr.Error)
		} else {
			success++
		}
	}

	result := &TaskResult{
		Summary: fmt.Sprintf(
			"  除权命中 : %d个    成功 : %d个    失败 : %d个",
			len(hits), success, fail,
		),
		FailedCodes: failCodes,
	}
	return result, nil
}

// handleNameChangeSync 处理【名称变更同步】(Task 12)
// 参数 JSON: {"source":"eastmoney"} — 可选，默认按优先级自动选源
func (r *DataCollectRunner) handleNameChangeSync(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	sourceName := parseSourceFromParams(task.Params)
	adp, err := ResolveAdapter(adapter.GetRegistry(), sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	res, err := RunNameChangesBatch(ctx, adp)
	if err != nil {
		return nil, err
	}
	return &TaskResult{
		Summary:     formatCollectSummary(res.Total, res.NewCount, res.UpdCount, res.FailCount),
		FailedCodes: res.FailedCodes,
	}, nil
}

// parseDividendPeriods 从 params JSON 中解析除权同步的周期列表。
// 默认返回 [daily, weekly, monthly]，支持逗号分隔如 "daily,weekly,monthly"。
func parseDividendPeriods(paramsJSON string) []db.KLinePeriod {
	params, err := ParseTaskParams(paramsJSON)
	if err != nil {
		return []db.KLinePeriod{db.KLinePeriodDaily, db.KLinePeriodWeekly, db.KLinePeriodMonthly}
	}
	periodsStr, _ := params["periods"].(string)
	if periodsStr == "" {
		return []db.KLinePeriod{db.KLinePeriodDaily, db.KLinePeriodWeekly, db.KLinePeriodMonthly}
	}

	parts := strings.Split(periodsStr, ",")
	periods := make([]db.KLinePeriod, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch db.KLinePeriod(p) {
		case db.KLinePeriodDaily, db.KLinePeriodWeekly, db.KLinePeriodMonthly:
			periods = append(periods, db.KLinePeriod(p))
		}
	}
	if len(periods) == 0 {
		return []db.KLinePeriod{db.KLinePeriodDaily, db.KLinePeriodWeekly, db.KLinePeriodMonthly}
	}
	return periods
}

// parseSourceFromParams 从任务 params JSON 中提取 source 参数
func parseSourceFromParams(paramsJSON string) string {
	params, err := ParseTaskParams(paramsJSON)
	if err != nil {
		return ""
	}
	s, _ := params["source"].(string)
	return s
}

// ResolveAdapter 按优先级解析数据源适配器（导出供 Handler 使用）
func ResolveAdapter(registry *adapter.Registry, preferredName string) (adapter.DataSource, error) {
	if preferredName != "" {
		adp, ok := registry.Get(preferredName)
		if !ok {
			return nil, fmt.Errorf("数据源未注册: %s", preferredName)
		}
		return adp, nil
	}

	// 未指定则从 DB 按优先级取可用源
	var configs []model.DataSourceConfig
	db.GetDB().Where("status = ?", "active").Order("priority ASC").Find(&configs)
	for _, cfg := range configs {
		adp, ok := registry.Get(cfg.Name)
		if ok && cfg.IsActive() {
			return adp, nil
		}
	}

	// fallback: 注册中心的第一个源
	for _, name := range registry.Names() {
		adp, ok := registry.Get(name)
		if ok {
			return adp, nil
		}
	}

	return nil, fmt.Errorf("无可用数据源")
}

// parsePeriodsFromParams 从任务 params JSON 中提取 periods 参数
func parsePeriodsFromParams(paramsJSON string) []db.KLinePeriod {
	params, err := ParseTaskParams(paramsJSON)
	if err != nil {
		return AllPeriods
	}
	periodsStr, _ := params["periods"].(string)
	if periodsStr == "" {
		return []db.KLinePeriod{db.KLinePeriodDaily}
	}
	return []db.KLinePeriod{db.KLinePeriod(periodsStr)}
}

// formatSyncResults 格式化 K 线批量同步结果，返回 *TaskResult。
//
// 每个周期一行，键值对齐；同时从 SyncBatchResult.Details 中提取失败股票代码。
func formatSyncResults(periods []db.KLinePeriod, results []SyncBatchResult) *TaskResult {
	var lines []string
	var allFailCodes []string

	for i, r := range results {
		label := db.KLineLabel(periods[i])
		lines = append(lines, fmt.Sprintf("  [%-6s] 同步=%s只  跳过=%s只  失败=%s只",
			label,
			formatInt(r.Success),
			formatInt(r.SkipNoDelta),
			formatInt(r.Fail),
		))

		// 从 Details 中提取失败股票代码
		for _, detail := range r.Details {
			if detail.Error != nil {
				allFailCodes = append(allFailCodes, detail.Code)
			}
		}
	}

	summary := strings.Join(lines, "\n")
	return &TaskResult{Summary: summary, FailedCodes: allFailCodes}
}

// formatStockSummary 格式化"股票维度 + 数据条数维度"双行汇总。
//
// 适用于每只股票对应一条记录的场景（如股票列表同步），
// 输出示例：
//
//	扫描股票 : 5,000    成功 : 4,990    失败 : 10
//	新增数据 : 10       更新 : 4,980
func formatStockSummary(total, newCount, updCount, failCount int) string {
	successStocks := total - failCount
	return fmt.Sprintf(
		"  扫描股票 : %s    成功 : %s    失败 : %d\n  新增数据 : %d       更新 : %d",
		formatInt(total), formatInt(successStocks), failCount,
		newCount, updCount,
	)
}

// formatCollectSummary 将通用批量采集结果（财报/股本/股东户数/分红等）格式化为双行文本。
//
// 第一行为股票维度（扫描/成功/失败），第二行为数据记录维度（新增/更新）。
// 一只股票可能产生多条数据记录，因此两个维度的数值通常不同。
func formatCollectSummary(total, newCount, updCount, failCount int) string {
	successStocks := total - failCount
	return fmt.Sprintf(
		"  扫描股票 : %s    成功 : %s    失败 : %d\n  新增数据 : %s      更新 : %s",
		formatInt(total), formatInt(successStocks), failCount,
		formatInt(newCount), formatInt(updCount),
	)
}

// formatInt 格式化整数，添加千分位分隔符。
func formatInt(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	runes := []rune(s)
	var result []rune
	for i, r := range runes {
		if (len(runes)-i)%3 == 0 && i > 0 {
			result = append(result, ',')
		}
		result = append(result, r)
	}
	return string(result)
}

// ============================================================================
//  Scheduler 实现
// ============================================================================

const changeChannelSize = 64

type schedulerImpl struct {
	cron     *cron.Cron
	runner   TaskRunner
	changeCh chan TaskChange
	mu       sync.Mutex
	entryMap map[uint]cron.EntryID // taskID -> cron entry
}

// NewScheduler 创建数据采集调度引擎。
// runner 为 nil 时会创建一个不含任何 Handler 的空 DataCollectRunner（仅打日志）。
func NewScheduler(runner TaskRunner) Scheduler {
	if runner == nil {
		runner = &DataCollectRunner{
			handlers: make(map[uint]TaskHandler),
			notifier: &subscriptionNotifier{},
		}
	}
	return &schedulerImpl{
		runner:   runner,
		changeCh: make(chan TaskChange, changeChannelSize),
		entryMap: make(map[uint]cron.EntryID),
	}
}

// Start 启动调度引擎
func (s *schedulerImpl) Start() error {
	s.cron = cron.New(cron.WithSeconds())

	// 加载所有启用任务
	tasks, err := db.GetActiveDataCollectTasks()
	if err != nil {
		return fmt.Errorf("加载活跃任务失败: %w", err)
	}

	registered := 0
	for i := range tasks {
		task := &tasks[i]
		if s.addTask(task) {
			registered++
		}
	}

	s.cron.Start()
	log.Printf("[DataCollect] 启动成功，已注册 %d 个任务", registered)

	// 启动变更监听 goroutine
	go s.changeLoop()

	return nil
}

// Stop 停止调度引擎
func (s *schedulerImpl) Stop() {
	close(s.changeCh)

	s.mu.Lock()
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
		log.Println("[DataCollect] 已停止")
	}
	s.mu.Unlock()
}

// NotifyChange 接收任务变更通知（非阻塞）
func (s *schedulerImpl) NotifyChange(change TaskChange) {
	select {
	case s.changeCh <- change:
	default:
		log.Printf("[DataCollect] 变更通道已满，丢弃事件: %s (id=%d)", change.Type, change.TaskID)
	}
}

// changeLoop 消费变更事件，动态更新 cron 注册
func (s *schedulerImpl) changeLoop() {
	for change := range s.changeCh {
		log.Printf("[DataCollect] 收到变更事件: %s (id=%d)", change.Type, change.TaskID)

		switch change.Type {
		case ChangeEnabled:
			task, err := db.GetDataCollectTaskByID(change.TaskID)
			if err != nil {
				log.Printf("[DataCollect] 加载任务 %d 失败: %v", change.TaskID, err)
				continue
			}
			if task.IsActive {
				s.addTask(task)
			}

		case ChangeUpdated:
			// 先移除，再按当前状态决定是否重新注册
			s.removeTask(change.TaskID)
			task, err := db.GetDataCollectTaskByID(change.TaskID)
			if err != nil {
				log.Printf("[DataCollect] 加载任务 %d 失败: %v", change.TaskID, err)
				continue
			}
			if task.IsActive {
				s.addTask(task)
			}

		case ChangeDisabled:
			s.removeTask(change.TaskID)
		}
	}
}

// addTask 注册单个任务到 cron
// 如果该 taskID 已存在，先移除旧的 cron entry 再重新注册（防止重复注册导致任务并行执行）
func (s *schedulerImpl) addTask(task *model.DataCollectTask) bool {
	if task.CronExpr == "" {
		log.Printf("[DataCollect] 任务 %d(%s) cron 表达式为空，跳过", task.ID, task.Name)
		return false
	}

	// 防止重复注册：如果已存在，先移除旧的
	s.mu.Lock()
	if oldEntryID, ok := s.entryMap[task.ID]; ok {
		s.cron.Remove(oldEntryID)
		delete(s.entryMap, task.ID)
	}
	s.mu.Unlock()

	taskID := task.ID
	entryID, err := s.cron.AddFunc(task.CronExpr, func() {
		s.runTask(taskID)
	})
	if err != nil {
		log.Printf("[DataCollect] 注册任务 %d(%s) cron 失败: %v (expr=%s)", task.ID, task.Name, err, task.CronExpr)
		return false
	}

	s.mu.Lock()
	s.entryMap[task.ID] = entryID
	s.mu.Unlock()

	log.Printf("[DataCollect] 已注册任务 %d(%s), cron=%s", task.ID, task.Name, task.CronExpr)
	return true
}

// removeTask 从 cron 移除单个任务
func (s *schedulerImpl) removeTask(taskID uint) {
	s.mu.Lock()
	entryID, ok := s.entryMap[taskID]
	delete(s.entryMap, taskID)
	s.mu.Unlock()

	if !ok {
		return
	}

	s.cron.Remove(entryID)
	log.Printf("[DataCollect] 已移除任务 %d", taskID)
}

// runTask 执行单次任务（cron job 回调）
func (s *schedulerImpl) runTask(taskID uint) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[DataCollect] 任务 %d 执行 panic 已恢复: %v", taskID, r)
		}
	}()

	// 从 DB 重新加载最新配置
	task, err := db.GetDataCollectTaskByID(taskID)
	if err != nil {
		log.Printf("[DataCollect] 加载任务 %d 失败: %v", taskID, err)
		return
	}

	// 检查 is_active
	if !task.IsActive {
		log.Printf("[DataCollect] 任务 %d 已禁用，跳过", taskID)
		return
	}

	// 加载关联机器人
	bots, err := db.GetDataCollectBots(taskID)
	if err != nil {
		log.Printf("[DataCollect] 加载任务 %d 关联机器人失败: %v", taskID, err)
		bots = nil // 无机器人也继续执行
	}

	log.Printf("[DataCollect] 开始执行任务 %d(%s), params=%s", taskID, task.Name, task.Params)

	// 记录开始时间
	startTime := time.Now()

	// TODO: 记录执行日志到数据库（后续实现）

	// 1小时超时
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	if err := s.runner.Run(ctx, task, bots); err != nil {
		log.Printf("[DataCollect] 任务 %d 执行失败: %v", taskID, err)
		// TODO: 记录失败日志
		return
	}

	elapsed := time.Since(startTime)
	log.Printf("[DataCollect] 任务 %d 执行完成, 耗时 %v", taskID, elapsed)

	// TODO: 记录成功日志
}

// ============================================================================
//  辅助函数
// ============================================================================

// ParseTaskParams 解析任务参数 JSON
func ParseTaskParams(paramsJSON string) (map[string]interface{}, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("解析任务参数失败: %w", err)
	}
	return params, nil
}

// ============================================================================
//  股票模型转换工具函数
// ============================================================================

// ToStockModel 将适配器的 StockBasic 转换为 GORM 模型 model.Stock
func ToStockModel(code string, detail *adapter.StockBasic) model.Stock {
	if detail == nil {
		return model.Stock{Code: code}
	}
	name := detail.Name
	return model.Stock{
		Code:         code,
		Name:         name,
		FullName:     detail.FullName,
		EnglishName:  detail.FullNameEn,
		Exchange:     detail.Exchange,
		ExchangeName: GetExchangeName(detail.Exchange),
		ListingBoard: detail.ListingBoard,
		BoardName:    GetBoardName(detail.ListingBoard),
		ListDate:     detail.ListDate,
		IssuePrice:   detail.IssuePrice,
		IssuePE:      detail.IssuePE,
		IssueShares:  detail.TotalIssueNum,
		Industry:     detail.Industry,
		Sector:  detail.Sector,
	}
}

// getDelistDate 获取股票的退市日期。
//
// 优先从 name_changes 表判断：若最新名称变更记录包含"退"且原因为"进入退市整理"，
// 则返回对应的变更日期作为退市日期。否则保留 DB 中已有的 delist_date，避免被空值覆盖。
func getDelistDate(code string) string {
	// 1. 查 name_changes 最新记录判断是否退市
	latest, err := db.FindLatestNameChange(code)
	if err == nil && latest.ChangeDate > 0 {
		if isDelistNameChange(latest.SecurityName, latest.ChangeReason) {
			// change_date 格式为 YYYYMMDD (int)，转为 YYYY-MM-DD
			return formatDate8To10(latest.ChangeDate)
		}
	}

	// 2. 非退市情况：保留 DB 中已有的 delist_date
	exist, err := db.FindStockByCode(code)
	if err == nil && exist.DelistDate != "" {
		return exist.DelistDate
	}

	return ""
}

// formatDate8To10 将 YYYYMMDD (int) 转为 "YYYY-MM-DD" 字符串
func formatDate8To10(d int) string {
	s := fmt.Sprintf("%08d", d)
	return s[:4] + "-" + s[4:6] + "-" + s[6:]
}

// GetExchangeName 获取交易所中文名
func GetExchangeName(exchange string) string {
	switch exchange {
	case "SSE":
		return "上海证券交易所"
	case "SZSE":
		return "深圳证券交易所"
	case "BSE":
		return "北京证券交易所"
	default:
		return exchange
	}
}

// GetBoardName 获取板块中文名
func GetBoardName(board string) string {
	switch board {
	case "main":
		return "主板"
	case "chinext":
		return "创业板"
	case "star":
		return "科创板"
	case "bse":
		return "北交所"
	default:
		return board
	}
}

// ============================================================================
//  Task 13: 重置行情缓存高优先级
// ============================================================================

// handleReloadHighPriority 每日 8:00 委托 watchlist.Manager 重置并重新加载高优先级。
func (r *DataCollectRunner) handleReloadHighPriority(ctx context.Context, task *model.DataCollectTask) (*TaskResult, error) {
	if r.watchlistMgr == nil {
		return &TaskResult{Summary: "watchlist Manager 未注入，跳过"}, nil
	}
	r.watchlistMgr.ReloadAll()
	return &TaskResult{Summary: "优先级已重置并重新加载"}, nil
}
