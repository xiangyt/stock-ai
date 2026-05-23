package service

import (
	"encoding/json"
	"fmt"

	"stock-ai/internal/datacollect"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// ============================================================================
//  DataCollectTaskService 数据采集任务管理业务逻辑层
//  注意：与 data_collect_service.go 不同，该服务负责前端定时任务配置管理
// ============================================================================

// DataCollectTaskService 数据采集任务管理服务
type DataCollectTaskService struct {
	notifyChangeFn func(datacollect.ChangeType, uint) // 通知 Scheduler 变更的回调
}

// NewDataCollectTaskService 创建数据采集任务管理服务
func NewDataCollectTaskService() *DataCollectTaskService {
	return &DataCollectTaskService{}
}

// SetNotifyChange 设置变更通知回调（由 main.go 注入）
func (s *DataCollectTaskService) SetNotifyChange(fn func(datacollect.ChangeType, uint)) {
	s.notifyChangeFn = fn
}

// ============================================================================
//  CRUD 方法
// ============================================================================

// List 获取所有数据采集任务
func (s *DataCollectTaskService) List() ([]model.DCTaskItem, error) {
	tasks, err := db.ListDataCollectTasks()
	if err != nil {
		return nil, fmt.Errorf("查询任务列表失败: %w", err)
	}

	// 批量加载机器人关联
	taskIDs := make([]uint, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.ID
	}
	botsMap, _ := db.GetDataCollectBotsByTaskIDs(taskIDs)

	items := make([]model.DCTaskItem, 0, len(tasks))
	for _, task := range tasks {
		bots := botsMap[task.ID]
		botInfos := toBotInfos(bots)

		items = append(items, model.DCTaskItem{
			ID:        task.ID,
			Name:      task.Name,
			CronExpr:  task.CronExpr,
			Params:    task.Params,
			IsActive:  task.IsActive,
			Bots:      botInfos,
			CreatedAt: task.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: task.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return items, nil
}

// GetByID 获取任务详情
func (s *DataCollectTaskService) GetByID(id uint) (*model.DCTaskItem, error) {
	task, err := db.GetDataCollectTaskByID(id)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return nil, fmt.Errorf("任务不存在")
		}
		return nil, fmt.Errorf("查询任务失败: %w", err)
	}

	bots, _ := db.GetDataCollectBots(task.ID)
	botInfos := toBotInfos(bots)

	return &model.DCTaskItem{
		ID:        task.ID,
		Name:      task.Name,
		CronExpr:  task.CronExpr,
		Params:    task.Params,
		IsActive:  task.IsActive,
		Bots:      botInfos,
		CreatedAt: task.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: task.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateTask 更新任务配置
func (s *DataCollectTaskService) UpdateTask(id uint, req *model.DCUpdateTaskReq) (*model.DCTaskItem, error) {
	// 先检查任务是否存在
	_, err := db.GetDataCollectTaskByID(id)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return nil, fmt.Errorf("任务不存在")
		}
		return nil, fmt.Errorf("查询任务失败: %w", err)
	}

	updates := make(map[string]interface{})

	if req.CronExpr != nil {
		if *req.CronExpr == "" {
			return nil, fmt.Errorf("cron 表达式不能为空")
		}
		updates["cron_expr"] = *req.CronExpr
	}

	if req.Params != nil {
		if !json.Valid([]byte(*req.Params)) {
			return nil, fmt.Errorf("参数格式无效，必须是合法 JSON")
		}
		updates["params"] = *req.Params
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		return s.GetByID(id)
	}

	if err := db.UpdateDataCollectTask(id, updates); err != nil {
		return nil, fmt.Errorf("更新任务失败: %w", err)
	}

	// 通知 Scheduler 变更
	if s.notifyChangeFn != nil {
		if req.IsActive != nil {
			if *req.IsActive {
				s.notifyChangeFn(datacollect.ChangeEnabled, id)
			} else {
				s.notifyChangeFn(datacollect.ChangeDisabled, id)
			}
		} else {
			s.notifyChangeFn(datacollect.ChangeUpdated, id)
		}
	}

	return s.GetByID(id)
}

// UpdateBots 更新任务关联的机器人
func (s *DataCollectTaskService) UpdateBots(id uint, botIDs []uint) error {
	// 检查任务是否存在
	_, err := db.GetDataCollectTaskByID(id)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return fmt.Errorf("任务不存在")
		}
		return fmt.Errorf("查询任务失败: %w", err)
	}

	// 校验机器人 ID 是否存在且已启用（无归属校验）
	if len(botIDs) > 0 {
		for _, botID := range botIDs {
			bot, err := db.GetBotByID(botID)
			if err != nil || bot == nil {
				return fmt.Errorf("机器人 %d 不存在", botID)
			}
			if bot.Status != 1 {
				return fmt.Errorf("机器人 %d 已禁用，无法关联", botID)
			}
		}
	}

	if err := db.SetDataCollectBots(id, botIDs); err != nil {
		return fmt.Errorf("更新机器人关联失败: %w", err)
	}

	return nil
}

// ============================================================================
//  辅助函数
// ============================================================================

func toBotInfos(bots []model.PushBot) []model.DCBotInfo {
	if len(bots) == 0 {
		return []model.DCBotInfo{}
	}
	infos := make([]model.DCBotInfo, len(bots))
	for i, b := range bots {
		infos[i] = model.DCBotInfo{
			ID:      b.ID,
			Name:    b.Name,
			Channel: b.Channel,
		}
	}
	return infos
}
