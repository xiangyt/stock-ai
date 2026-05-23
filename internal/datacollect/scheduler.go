package datacollect

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
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

// TaskHandler 单个采集任务的处理函数。
// 返回结果描述信息（用于推送到机器人），不返回 error 视为执行成功。
type TaskHandler func(ctx context.Context, task *model.DataCollectTask) (summary string, err error)

// TaskRunner 任务执行器接口（策略模式，方便测试时替换）
type TaskRunner interface {
	Run(ctx context.Context, task *model.DataCollectTask, bots []model.PushBot) error
}

// DataCollectRunner 数据采集运行器，按内置任务 ID 分发到对应的 TaskHandler。
// 执行完成后，通过关联的机器人推送执行结果通知。
type DataCollectRunner struct {
	handlers map[uint]TaskHandler  // taskID → 处理函数
	notifier *subscriptionNotifier // 机器人推送封装
}

// subscriptionNotifier 对 subscription/notifier 的轻量封装，仅暴露机器人推送能力。
type subscriptionNotifier struct{}

func (s *subscriptionNotifier) Push(ctx context.Context, bots []model.PushBot, message string) {
	for _, bot := range bots {
		if bot.Status == 0 {
			continue
		}
		// 导入 subscription/notifier 包会造成循环依赖，此处使用内联发送逻辑
		if err := pushOnce(&bot, message); err != nil {
			log.Printf("[DataCollect] 推送失败 bot=%d(%s): %v", bot.ID, bot.Name, err)
		}
	}
}

// NewDataCollectRunner 创建数据采集运行器，注册所有任务 Handler。
func NewDataCollectRunner() *DataCollectRunner {
	r := &DataCollectRunner{
		handlers: make(map[uint]TaskHandler),
		notifier: &subscriptionNotifier{},
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

	return r
}

// Run 执行采集任务：按 TaskID 分发执行 → 推送结果到关联机器人
func (r *DataCollectRunner) Run(ctx context.Context, task *model.DataCollectTask, bots []model.PushBot) error {
	handler, ok := r.handlers[task.ID]
	if !ok {
		return fmt.Errorf("未知任务 %d(%s)，无对应 Handler", task.ID, task.Name)
	}

	summary, err := handler(ctx, task)
	if err != nil {
		errMsg := fmt.Sprintf("❌ [%s] 执行失败\n错误: %v", task.Name, err)
		log.Printf("[DataCollect] 任务 %d 执行失败: %v", task.ID, err)
		if len(bots) > 0 {
			r.notifier.Push(ctx, bots, errMsg)
		}
		return err
	}

	log.Printf("[DataCollect] 任务 %d 执行成功: %s", task.ID, summary)
	if len(bots) > 0 && summary != "" {
		r.notifier.Push(ctx, bots, fmt.Sprintf("✅ [%s] 执行完成\n%s", task.Name, summary))
	}
	return nil
}

// ============================================================================
//  Task Handler 实现
// ============================================================================

// handleStockDetailSync 处理【股票列表同步】(Task 1)
// 参数 JSON: {"source":"tencentstock"} — 可选，默认按优先级自动选源
// 流程: 数据源 → 获取全量股票列表 → 遍历获取详情 → upsert入库
func (r *DataCollectRunner) handleStockDetailSync(ctx context.Context, task *model.DataCollectTask) (string, error) {
	sourceName := parseSourceFromParams(task.Params)
	registry := adapter.GetRegistry()

	adp, err := ResolveAdapter(registry, sourceName)
	if err != nil {
		return "", fmt.Errorf("获取数据源失败: %w", err)
	}

	log.Printf("[DataCollect] 开始股票列表同步, source=%s", adp.Name())

	allStocks, err := adp.GetStockList(ctx)
	if err != nil {
		return "", fmt.Errorf("获取股票列表失败: %w", err)
	}

	total := len(allStocks)
	newCount, updCount, failCount := 0, 0, 0

	for i, stock := range allStocks {
		detail, detailErr := adp.GetStockDetail(ctx, stock.Code)
		if detailErr != nil {
			log.Printf("[DataCollect] 获取详情失败 [%s]: %v", stock.Code, detailErr)
			failCount++
			continue
		}

		s := ToStockModel(stock.Code, detail)
		if db.UpsertStock(s) == 0 {
			newCount++
		} else {
			updCount++
		}

		if (i+1)%100 == 0 || i == total-1 {
			log.Printf("[DataCollect] 详情进度: %d/%d (新增=%d, 更新=%d)", i+1, total, newCount, updCount)
		}
	}

	return fmt.Sprintf("total=%d, 新增=%d, 更新=%d, 失败=%d", total, newCount, updCount, failCount), nil
}

// handleDailyKlineInc 处理【每日增量同步K线数据】(Task 2)
// 参数 JSON: {"periods":"daily"} — 可选，默认 daily
func (r *DataCollectRunner) handleDailyKlineInc(ctx context.Context, task *model.DataCollectTask) (string, error) {
	periods := parsePeriodsFromParams(task.Params)
	results := GetSyncKLineService().SyncDailyForAll(ctx, periods)
	return formatSyncResults(periods, results), nil
}

// handleEastmoneyFull 处理【东财数据全量补全】(Task 3)
// 参数 JSON: {"periods":"daily"} — 可选，默认 daily
func (r *DataCollectRunner) handleEastmoneyFull(ctx context.Context, task *model.DataCollectTask) (string, error) {
	if utils.IsTradingDay() && time.Now().Hour() > 6 && time.Now().Hour() < 16 {
		return "", nil
	}
	periods := parsePeriodsFromParams(task.Params)
	results := GetSyncKLineService().FillMissingAmount(ctx, periods)
	return formatSyncResults(periods, results), nil
}

// handleFinanceReportSync 处理【财报同步】(Task 4)
// 参数 JSON: {"source":"tencentstock"} — 可选，默认按优先级自动选源
func (r *DataCollectRunner) handleFinanceReportSync(ctx context.Context, task *model.DataCollectTask) (string, error) {
	sourceName := parseSourceFromParams(task.Params)
	adp, err := ResolveAdapter(adapter.GetRegistry(), sourceName)
	if err != nil {
		return "", fmt.Errorf("获取数据源失败: %w", err)
	}
	res, err := RunPerformanceReportsBatch(ctx, adp)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("total=%d, 新增=%d, 更新=%d, 失败=%d", res.Total, res.NewCount, res.UpdCount, res.FailCount), nil
}

// handleShareChangeSync 处理【股本变动同步】(Task 5)
func (r *DataCollectRunner) handleShareChangeSync(ctx context.Context, task *model.DataCollectTask) (string, error) {
	sourceName := parseSourceFromParams(task.Params)
	adp, err := ResolveAdapter(adapter.GetRegistry(), sourceName)
	if err != nil {
		return "", fmt.Errorf("获取数据源失败: %w", err)
	}
	res, err := RunShareChangesBatch(ctx, adp)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("total=%d, 新增=%d, 更新=%d, 失败=%d", res.Total, res.NewCount, res.UpdCount, res.FailCount), nil
}

// handleShareholderSync 处理【股东户数同步】(Task 6)
func (r *DataCollectRunner) handleShareholderSync(ctx context.Context, task *model.DataCollectTask) (string, error) {
	sourceName := parseSourceFromParams(task.Params)
	adp, err := ResolveAdapter(adapter.GetRegistry(), sourceName)
	if err != nil {
		return "", fmt.Errorf("获取数据源失败: %w", err)
	}
	res, err := RunShareholderCountsBatch(ctx, adp)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("total=%d, 新增=%d, 更新=%d, 失败=%d", res.Total, res.NewCount, res.UpdCount, res.FailCount), nil
}

// handleDailySnapshotSync 处理【每日快照计算】(Task 7)
func (r *DataCollectRunner) handleDailySnapshotSync(ctx context.Context, task *model.DataCollectTask) (string, error) {
	res := GetSnapshotService().calcAllStocksAllDates(ctx)
	return fmt.Sprintf("股票:总数=%d 成功=%d 失败=%d | 快照:总数=%d 写入成功=%d 写入失败=%d | 耗时=%.1fs",
		res.TotalStocks, res.SuccessStocks, res.FailStocks,
		res.TotalSnapshots, res.SuccessSnapshots, res.FailSnapshots,
		res.CostSeconds), nil
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

// formatSyncResults 格式化批量同步结果，每个周期一行，用 \n 拼接。
func formatSyncResults(periods []db.KLinePeriod, results []SyncBatchResult) string {
	lines := make([]string, 0, len(results))
	for i, r := range results {
		lines = append(lines, fmt.Sprintf("[%s]成功=%d 跳过=%d 失败=%d", periods[i], r.Success, r.SkipNoDelta, r.Fail))
	}
	return strings.Join(lines, "\n")
}

// ============================================================================
//  机器人推送（内联实现，避免循环依赖）
// ============================================================================

// pushOnce 单次推送消息到指定机器人，失败时自动重试一次
func pushOnce(bot *model.PushBot, message string) error {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		err = sendByChannel(bot, message)
		if err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// sendByChannel 根据机器人渠道发送消息
func sendByChannel(bot *model.PushBot, message string) error {
	switch bot.Channel {
	case "dingtalk":
		payload, _ := json.Marshal(map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": message},
		})
		url := bot.WebhookURL
		if bot.Secret != "" {
			timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
			sign := dingTalkSign(timestamp, bot.Secret)
			url = fmt.Sprintf("%s&timestamp=%s&sign=%s", bot.WebhookURL, timestamp, sign)
		}
		return postJSON(url, payload)
	case "feishu":
		payload, _ := json.Marshal(map[string]interface{}{
			"msg_type": "text",
			"content":  map[string]string{"text": message},
		})
		return postJSON(bot.WebhookURL, payload)
	case "wecom":
		payload, _ := json.Marshal(map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": message},
		})
		return postJSON(bot.WebhookURL, payload)
	default:
		return fmt.Errorf("不支持的渠道: %s", bot.Channel)
	}
}

// postJSON HTTP POST JSON
func postJSON(url string, body []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// dingTalkSign 钉钉加签（HMAC-SHA256）
func dingTalkSign(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	return hex.EncodeToString(mac.Sum(nil))
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

	// 30 分钟超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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
	return model.Stock{
		Code:         code,
		Name:         detail.Name,
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
		Sector:       detail.Sector,
	}
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
