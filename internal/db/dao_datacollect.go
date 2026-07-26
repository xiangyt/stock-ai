package db

import (
	"context"
	"errors"
	"fmt"

	"stock-ai/internal/model"

	"gorm.io/gorm"
)

// ============================================================================
//  DataCollectTask 数据采集任务 DAO
// ============================================================================

// ListDataCollectTasks 获取所有数据采集任务（按 ID 排序）
func ListDataCollectTasks() ([]model.DataCollectTask, error) {
	var tasks []model.DataCollectTask
	err := GetDB().Order("id ASC").Find(&tasks).Error
	return tasks, err
}

// GetDataCollectTaskByID 根据 ID 查询任务
func GetDataCollectTaskByID(id uint) (*model.DataCollectTask, error) {
	var task model.DataCollectTask
	err := GetDB().Where("id = ?", id).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &task, nil
}

// GetActiveDataCollectTasks 获取所有启用的任务（Scheduler 启动时用）
func GetActiveDataCollectTasks() ([]model.DataCollectTask, error) {
	var tasks []model.DataCollectTask
	err := GetDB().Where("is_active = ?", true).Order("id ASC").Find(&tasks).Error
	return tasks, err
}

// UpdateDataCollectTask 更新任务字段（条件更新，只更新非零值字段）
func UpdateDataCollectTask(id uint, updates map[string]interface{}) error {
	result := GetDB().Model(&model.DataCollectTask{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// UpsertDataCollectTask 插入或更新任务（用于初始化数据）。
// ctx 用于透传 trace_id。
func UpsertDataCollectTask(ctx context.Context, task *model.DataCollectTask) error {
	var existing model.DataCollectTask
	err := GetDB().WithContext(ctx).Where("id = ?", task.ID).First(&existing).Error
	if err == nil {
		// 已存在，跳过（初始化数据不应覆盖用户修改）
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	// 不存在，创建
	return GetDB().WithContext(ctx).Create(task).Error
}

// InitDataCollectTasks 初始化内置数据采集任务。
// ctx 用于透传 trace_id。
func InitDataCollectTasks(ctx context.Context) error {
	initTasks := model.GetInitialDataCollectTasks()
	for _, t := range initTasks {
		task := &model.DataCollectTask{
			ID:       t.ID,
			Name:     t.Name,
			CronExpr: t.CronExpr,
			Params:   t.Params,
			IsActive: t.IsActive,
		}
		if err := UpsertDataCollectTask(ctx, task); err != nil {
			return fmt.Errorf("初始化任务 %d(%s) 失败: %w", t.ID, t.Name, err)
		}
	}
	return nil
}

// ============================================================================
//  DataCollectBot 关联表 DAO
// ============================================================================

// SetDataCollectBots 全量替换任务关联的机器人（先删后插，事务）
func SetDataCollectBots(taskID uint, botIDs []uint) error {
	tx := GetDB().Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 删除旧的关联
	if err := tx.Where("task_id = ?", taskID).Delete(&model.DataCollectBot{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 插入新的关联
	if len(botIDs) > 0 {
		bots := make([]model.DataCollectBot, len(botIDs))
		for i, botID := range botIDs {
			bots[i] = model.DataCollectBot{
				TaskID: taskID,
				BotID:  botID,
			}
		}
		if err := tx.Create(&bots).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// GetDataCollectBots 获取任务关联的机器人列表（JOIN 查询）
func GetDataCollectBots(taskID uint) ([]model.PushBot, error) {
	var bots []model.PushBot
	err := GetDB().
		Joins("JOIN data_collect_bots dcb ON dcb.bot_id = push_bots.id").
		Where("dcb.task_id = ?", taskID).
		Find(&bots).Error
	return bots, err
}

// GetDataCollectBotsByTaskIDs 批量获取任务关联的机器人（按 taskID 分组）
func GetDataCollectBotsByTaskIDs(taskIDs []uint) (map[uint][]model.PushBot, error) {
	type row struct {
		TaskID uint
		model.PushBot
	}

	var rows []row
	err := GetDB().
		Select("dcb.task_id, push_bots.*").
		Joins("JOIN data_collect_bots dcb ON dcb.bot_id = push_bots.id").
		Where("dcb.task_id IN ?", taskIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint][]model.PushBot)
	for _, r := range rows {
		result[r.TaskID] = append(result[r.TaskID], r.PushBot)
	}
	return result, nil
}
