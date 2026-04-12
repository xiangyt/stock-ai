package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"gorm.io/gorm"
)

// ========== 数据采集服务 ==========

// DataCollectService 数据采集服务
type DataCollectService struct {
	db      *gorm.DB
	mu      sync.Mutex // 防止并发执行
	registry *adapter.Registry // 数据源注册中心
}

func NewDataCollectService() *DataCollectService {
	return &DataCollectService{
		db:       db.GetDB(),
		registry: adapter.GetRegistry(),
	}
}

// ========== 采集任务相关方法 ==========

type CreateTaskRequest struct {
	Type       string                 `json:"type" binding:"required"`                // 任务类型
	SourceName string                 `json:"source_name"`                            // 指定数据源(可选)
	TargetCode string                 `json:"target_code"`                            // 目标股票代码
	Params     map[string]interface{} `json:"params"`                                 // 额外参数
	CreatedBy  string                 `json:"created_by"`
}

type TaskListQuery struct {
	Type   string `form:"type"`
	Status string `form:"status"`
	Limit  int    `form:"limit,default:20"`
	Offset int    `form:"offset,default:0"`
}

type TaskResponse struct {
	model.CollectTask
	DurationMs int64  `json:"duration_ms"`
	Progress   float64 `json:"progress"`
}

// CreateTask 创建采集任务
func (s *DataCollectService) CreateTask(req CreateTaskRequest) (*model.CollectTask, error) {
	task := model.CollectTask{
		TaskID:     generateUUID(),
		Type:       req.Type,
		Status:     model.TaskPending,
		SourceName: req.SourceName,
		TargetCode: req.TargetCode,
		CreatedBy:  req.CreatedBy,
	}

	if req.Params != nil {
		paramsBytes, _ := json.Marshal(req.Params)
		task.Params = string(paramsBytes)
	}

	if err := s.db.Create(&task).Error; err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	return &task, nil
}

// ListTasks 列出采集任务
func (s *DataCollectService) ListTasks(query TaskListQuery) ([]model.CollectTask, int64, error) {
	var tasks []model.CollectTask
	var total int64

	tx := s.db.Model(&model.CollectTask{})

	if query.Type != "" {
		tx = tx.Where("type = ?", query.Type)
	}
	if query.Status != "" {
		tx = tx.Where("status = ?", query.Status)
	}

	tx.Count(&total).Order("created_at DESC").Limit(query.Limit).Offset(query.Offset).Find(&tasks)

	return tasks, total, nil
}

// GetTaskDetail 获取任务详情
func (s *DataCollectService) GetTaskDetail(id uint) (*model.CollectTask, error) {
	var task model.CollectTask
	if err := s.db.Preload("Logs", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).First(&task, id).Error; err != nil {
		return nil, fmt.Errorf("任务不存在: %w", err)
	}
	return &task, nil
}

// DeleteTask 删除采集任务
func (s *DataCollectService) DeleteTask(id uint) error {
	var task model.CollectTask

	result := s.db.First(&task, id)
	if result.Error != nil {
		return result.Error
	}

	if task.Status == model.TaskRunning {
		return fmt.Errorf("不能删除正在运行的任务")
	}

	s.db.Where("task_id = ?", id).Delete(&model.CollectLog{})
	return s.db.Delete(&task).Error
}

// ========== 执行采集任务 ==========

// RunStockList 运行股票列表采集
func (s *DataCollectService) RunStockList(sourceName string) (*model.CollectTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.CreateTask(CreateTaskRequest{
		Type:       model.TaskTypeStockList,
		SourceName: sourceName,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	task.Status = model.TaskRunning
	task.StartedAt = &now
	s.db.Save(task)

	log.Printf("[采集] 开始采集股票列表, task_id=%s", source=%s, taskID)

	// 获取适配器
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		s.markTaskFailed(task.ID, err.Error())
		return nil, err
	}

	// 执行采集（带进度回调）
	ctx := context.Background()
	stocks, err := adp.GetStockList(ctx, func(current, total int, message string) {
		s.updateTaskProgress(task.ID, current, total, message)
	})
	if err != nil {
		s.markTaskFailed(task.ID, err.Error())
		return nil, err
	}

	// 写入数据库（upsert）
	successCount := 0
	for _, stock := range stocks {
		var existing model.Stock
		result := s.db.Where("code = ?", stock.Code).First(&existing)

		if result.Error == gorm.ErrRecordNotFound {
			newStock := model.Stock{
				Code:        stock.Code,
				Name:        stock.Name,
				FullName:    stock.FullName,
				Exchange:    stock.Exchange,
				ListingBoard: stock.ListingBoard,
				ListDate:    stock.ListDate,
				IssuePrice:  stock.IssuePrice,
				IssuePE:     stock.IssuePE,
				IssuePB:     stock.IssuePB,
				IssueShares: stock.IssueShares,
				Industry:    stock.Industry,
				Sector:      stock.Sector,
				Status:      "normal",
			}
			if err := s.db.Create(&newStock).Error; err != nil {
				s.addTaskLog(task.ID, "error", "插入失败: "+stock.Name+" - "+err.Error(), stock.Code)
				continue
			}
		} else if result.Error == nil {
			// 已存在，更新可变字段
			s.db.Model(&existing).Updates(map[string]interface{}{
				"name":          stock.Name,
				"full_name":     stock.FullName,
				"industry":      stock.Industry,
				"sector":        stock.Sector,
				"update_time":   time.Now().Format("2006-01-02 15:04:05"),
			})
		}
		successCount++

		if successCount%200 == 0 {
			s.addTaskLog(task.ID, "info", fmt.Sprintf("已处理 %d/%d 条", successCount, len(stocks)), "")
		}
	}

	finishedAt := time.Now()
	s.db.Model(&model.CollectTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":        model.TaskCompleted,
		"finished_at":   &finishedAt,
		"total_count":   len(stocks),
		"success_count": successCount,
		"fail_count":    len(stocks) - successCount,
	})

	s.addTaskLog(task.ID, "info", fmt.Sprintf("采集完成: 总计 %d, 成功 %d, 失败 %d", len(stocks), successCount, len(stocks)-successCount), "")
	log.Printf("[采集] 股票列表采集完成, total=%d, success=%d, source=%s", len(stocks), successCount, sourceName)

	return s.GetTaskDetail(task.ID)
}

// RunSingleStockPrice 运行单只股票价格采集
func (s *DataCollectService) RunSingleStockPrice(sourceName, code string) (*model.CollectTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.CreateTask(CreateTaskRequest{
		Type:       model.TaskTypeStockPrice,
		SourceName: sourceName,
		TargetCode: code,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	task.Status = model.TaskRunning
	task.StartedAt = &now
	s.db.Save(task)

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		s.markTaskFailed(task.ID, err.Error())
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -120).Format("2006-01-02") // 近4个月数据

	prices, err := adp.GetDailyKLine(ctx, code, startDate, today, func(progress adapter.ProgressCallback) {
		s.updateTaskProgress(task.ID, progress)
	})
	if err != nil {
		s.markTaskFailed(task.ID, err.Error())
		return nil, err
	}

	// 写入价格数据
	for i := range prices {
		priceModel := toModelPrice(prices[i])
		if err := s.db.Create(&priceModel).Error; err != nil {
			// 忽略重复键错误
			continue
		}
	}

	finishedAt := time.Now()
	s.db.Model(&model.CollectTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":        model.TaskCompleted,
		"finished_at":   &finishedAt,
		"total_count":   len(prices),
		"success_count": len(prices),
	})

	return s.GetTaskDetail(task.ID)
}

// RunAllPrices 运行全量价格采集
func (s *DataCollectService) RunAllPrices(sourceName string) (*model.CollectTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.CreateTask(CreateTaskRequest{
		Type:       model.TaskTypeAllPrices,
		SourceName: sourceName,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	task.Status = model.TaskRunning
	task.StartedAt = &now
	s.db.Save(task)

	// 获取所有在市股票代码
	var stocks []model.Stock
	s.db.Where("status = ?", "normal").Select("code").Find(&stocks)

	codes := make([]string, len(stocks))
	for i, st := range stocks {
		codes[i] = st.Code
	}

	log.Printf("[采集] 开始全量实时行情采集, 共 %d 只股票, source=%s", len(codes), sourceName)

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		s.markTaskFailed(task.ID, err.Error())
		return nil, err
	}

	today := time.Now().Format("2006-01-02")

	pricesMap, err := adp.GetRealtimeData(ctx, codes, func(progress adapter.ProgressCallback) {
		s.updateTaskProgress(task.ID, progress)
	})
	if err != nil {
		s.markTaskFailed(task.ID, err.Error())
		return nil, err
	}

	totalSuccess := 0
	for code, price := range pricesMap {
		priceModel := toModelPrice(price)
		if err := s.db.Where("stock_code = ? AND date = ?", code, today).
			Assign(priceModel).FirstOrCreate(&priceModel).Error; err != nil {
			continue
		}
		totalSuccess++
	}

	failed := len(codes) - totalSuccess
	finishedAt := time.Now()
	s.db.Model(&model.CollectTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":        model.TaskCompleted,
		"finished_at":   &finishedAt,
		"total_count":   len(codes),
		"success_count": totalSuccess,
		"fail_count":    failed,
	})

	log.Printf("[采集] 全量行情采集完成, total=%d, success=%d, fail=%d", len(codes), totalSuccess, failed)

	return s.GetTaskDetail(task.ID)
}

// ========== 状态查询 ==========

// CollectorStatus 采集器状态
type CollectorStatus struct {
	IsRunning  bool               `json:"is_running"`
	Sources    []DataSourceStatus `json:"sources"`
	LastTasks  []TaskResponse     `json:"last_tasks"`
	StockTotal int64              `json:"stock_total"`
	LastUpdate string             `json:"last_update"`
}

type DataSourceStatus struct {
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	IsAvailable   bool   `json:"is_available"`
	RateRemaining int    `json:"rate_remaining"`
}

// GetStatus 获取采集器状态
func (s *DataCollectService) GetStatus() (*CollectorStatus, error) {
	status := &CollectorStatus{
		Sources:   make([]DataSourceStatus, 0),
		LastTasks: make([]TaskResponse, 0),
	}

	// 从注册中心获取已注册的数据源
	registeredAdapters := s.registry.List()
	registeredNames := make(map[string]bool)
	for _, adp := range registeredAdapters {
		registeredNames[adp.Name()] = true
		status.Sources = append(status.Sources, DataSourceStatus{
			Name:        adp.Name(),
			DisplayName: adp.DisplayName(),
			Type:        adp.Type(),
			Status:      "active",
			IsAvailable: true,
		})
	}

	// 合并数据库中的配置信息
	var configs []model.DataSourceConfig
	s.db.Find(&configs)
	for _, cfg := range configs {
		found := false
		for i, ds := range status.Sources {
			if ds.Name == cfg.Name {
				status.Sources[i].Status = cfg.Status
				status.Sources[i].IsAvailable = cfg.IsActive() && cfg.IsQuotaAvailable()
				if cfg.DailyQuota > 0 {
					status.Sources[i].RateRemaining = cfg.DailyQuota - cfg.UsedQuota
				} else {
					status.Sources[i].RateRemaining = -1
				}
				found = true
				break
			}
		}
		if !found && !registeredNames[cfg.Name] {
			// 数据库有但未注册的源
			status.Sources = append(status.Sources, DataSourceStatus{
				Name:          cfg.Name,
				DisplayName:   cfg.DisplayName,
				Type:          cfg.Type,
				Status:        cfg.Status,
				IsAvailable:   false,
				RateRemaining: 0,
			})
		}
	}

	var runningCount int64
	s.db.Model(&model.CollectTask{}).Where("status = ?", model.TaskRunning).Count(&runningCount)
	status.IsRunning = runningCount > 0

	var tasks []model.CollectTask
	s.db.Order("created_at DESC").Limit(5).Find(&tasks)
	for _, t := range tasks {
		status.LastTasks = append(status.LastTasks, TaskResponse{
			CollectTask: t,
			DurationMs:  t.Duration(),
			Progress:    t.Progress(),
		})
	}

	s.db.Model(&model.Stock{}).Where("status = ?", "normal").Count(&status.StockTotal)

	var lastPrice model.StockPrice
	s.db.Order("date DESC").First(&lastPrice)
	if lastPrice.Date != "" {
		status.LastUpdate = lastPrice.Date
	}

	return status, nil
}

// GetLogs 获取采集日志
func (s *DataCollectService) GetLogs(taskID uint, limit int) ([]model.CollectLog, error) {
	var logs []model.CollectLog
	query := s.db.Where("task_id = ?", taskID).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	query.Find(&logs)
	return logs, nil
}

// ========== 数据源管理 ==========

// ListSources 列出数据源配置
func (s *DataCollectService) ListSources() ([]model.DataSourceConfig, error) {
	var sources []model.DataSourceConfig
	s.db.Order("priority ASC").Find(&sources)
	return sources, nil
}

// UpdateSource 更新数据源配置
func (s *DataCollectService) UpdateSource(name string, config map[string]interface{}) error {
	var source model.DataSourceConfig
	if err := s.db.Where("name = ?", name).First(&source).Error; err != nil {
		return fmt.Errorf("数据源不存在: %s", name)
	}

	configBytes, _ := json.Marshal(config)
	configStr := string(configBytes)

	return s.db.Model(&source).Updates(map[string]interface{}{
		"config": configStr,
	}).Error
}

// ========== 内部辅助函数 ==========

// getAdapter 从注册中心获取指定名称的适配器
func (s *DataCollectService) getAdapter(preferredName string) (adapter.DataSource, error) {
	if preferredName != "" {
		adp, ok := s.registry.Get(preferredName)
		if ok {
			return adp, nil
		}
		return nil, fmt.Errorf("数据源未注册或不可用: %s", preferredName)
	}

	// 没指定则按优先级从数据库取可用源
	var configs []model.DataSourceConfig
	s.db.Where("status = ?", "active").Order("priority ASC").Find(&configs)

	for _, cfg := range configs {
		adp, ok := s.registry.Get(cfg.Name)
		if ok && cfg.IsActive() {
			return adp, nil
		}
	}

	// fallback: 注册中心的第一个可用源
	for _, name := range s.registry.Names() {
		adp, ok := s.registry.Get(name)
		if ok {
			return adp, nil
		}
	}

	return nil, fmt.Errorf("无可用数据源，请先在 config.yaml 中启用并注册数据源")
}

// updateTaskProgress 更新任务进度
func (s *DataCollectService) updateTaskProgress(taskID uint, progress adapter.ProgressCallback) {
	if progress.Message != "" {
		s.addTaskLog(taskID, "info", progress.Message, "")
	}
	if progress.Total > 0 {
		pct := float64(progress.Current) / float64(progress.Total) * 100
		s.db.Model(&model.CollectTask{}).Where("id = ?", taskID).Update("progress", pct)
	}
}

func (s *DataCollectService) markTaskFailed(taskID uint, errorMsg string) {
	finishedAt := time.Now()
	s.db.Model(&model.CollectTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":     model.TaskFailed,
		"finished_at": &finishedAt,
		"error_msg":  errorMsg,
	})
	s.addTaskLog(taskID, "error", errorMsg, "")
}

func (s *DataCollectService) addTaskLog(taskID uint, level, message, code string) {
	logEntry := model.CollectLog{
		TaskID:  taskID,
		Level:   level,
		Message: message,
		Code:    code,
	}
	s.db.Create(&logEntry)
}

// toModelPrice 将适配器的 StockPriceDaily 转换为 GORM 模型
func toModelPrice(p adapter.StockPriceDaily) model.StockPrice {
	return model.StockPrice{
		StockCode:  p.Code,
		Date:       p.Date,
		Open:       p.Open,
		Close:      p.Close,
		High:       p.High,
		Low:        p.Low,
		Volume:     p.Volume,
		Amount:     p.Amount,
		Turnover:   p.Turnover,
		Amplitude:  p.Amplitude,
		ChangePct:  p.ChangePct,
		Change:     p.Change,
	}
}

func generateUUID() string {
	return fmt.Sprintf("%d%s", time.Now().UnixNano(), randomString(8))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range b {
		b[i] = letters[r.Int63()%int64(len(letters))]
	}
	return string(b)
}
