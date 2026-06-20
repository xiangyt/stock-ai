package model

import "time"

// ============================================================================
//  数据采集模块 — 数据模型
// ============================================================================

// DataCollectTask 数据采集任务配置表
// 参考 strategies 表，不使用 UID（全局配置）
type DataCollectTask struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	CronExpr  string    `gorm:"size:100;not null" json:"cron_expr"` // 6 段秒级 cron 表达式
	Params    string    `gorm:"type:text" json:"params"`            // JSON 格式执行参数
	IsActive  bool      `gorm:"default:false" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DataCollectTask) TableName() string { return "data_collect_tasks" }

// DataCollectBot 数据采集任务-机器人关联表（M2M）
type DataCollectBot struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	TaskID    uint      `gorm:"uniqueIndex:idx_dc_task_bot;not null" json:"task_id"`
	BotID     uint      `gorm:"uniqueIndex:idx_dc_task_bot;not null" json:"bot_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (DataCollectBot) TableName() string { return "data_collect_bots" }

// ============================================================================
//  内置任务常量 ID
// ============================================================================

const (
	TaskStockDetailSync   uint = 1 // 股票详情同步
	TaskDailyKlineIncSync uint = 2 // 每日增量同步K线数据
	TaskEastmoneyFullSync  uint = 3 // 东财数据全量补全
	TaskFinanceReportSync  uint = 4 // 同步全量财报
	TaskShareChangeSync   uint = 5 // 同步股本变化
	TaskShareholderSync   uint = 6 // 同步股东人数变化
	TaskDailySnapshotSync    uint = 7 // 同步每日快照
	TaskWeeklyKlineIncSync   uint = 8 // 每周增量同步K线数据（weekly）
	TaskMonthlyKlineIncSync  uint = 9 // 每月增量同步K线数据（monthly）
	TaskDividendSync         uint = 10 // 同步分红历史
	TaskDividendKlineSync    uint = 11 // 除权K线同步（日/周/月）
	TaskNameChangeSync        uint = 12 // 同步名称变更
)

// ============================================================================
//  初始化任务定义
// ============================================================================

// InitialDataCollectTask 初始任务定义
type InitialDataCollectTask struct {
	ID       uint
	Name     string
	CronExpr string
	IsActive bool
	Params   string
}

// ============================================================================
//  请求/响应结构体（供 service + handler 使用）
// ============================================================================

// DCBotInfo 机器人简要信息
type DCBotInfo struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Channel string `json:"channel"`
}

// DCTaskItem 数据采集任务列表项
type DCTaskItem struct {
	ID        uint       `json:"id"`
	Name      string     `json:"name"`
	CronExpr  string     `json:"cron_expr"`
	Params    string     `json:"params"`
	IsActive  bool       `json:"is_active"`
	Bots      []DCBotInfo `json:"bots"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
}

// DCUpdateTaskReq 更新任务请求
type DCUpdateTaskReq struct {
	CronExpr *string `json:"cron_expr"`
	Params   *string `json:"params"`
	IsActive *bool   `json:"is_active"`
}

// DCUpdateBotsReq 更新关联机器人请求
type DCUpdateBotsReq struct {
	BotIDs []uint `json:"bot_ids"`
}

// GetInitialDataCollectTasks 返回所有初始数据采集任务
func GetInitialDataCollectTasks() []InitialDataCollectTask {
	return []InitialDataCollectTask{
		{
			ID:       TaskStockDetailSync,
			Name:     "股票详情同步",
			CronExpr: "0 0 0 1 * *",
			IsActive: false,
			Params:   `{"source":"eastmoney"}`,
		},
		{
			ID:       TaskDailyKlineIncSync,
			Name:     "每日增量同步K线数据",
			CronExpr: "0 0 18 ? * 1-5",
			IsActive: true,
			Params:   `{"periods":"daily"}`,
		},
		{
			ID:       TaskEastmoneyFullSync,
			Name:     "东财数据全量补全",
			CronExpr: "0 */30 * * * *",
			IsActive: true,
			Params:   `{"periods":"daily"}`,
		},
		{
			ID:       TaskFinanceReportSync,
			Name:     "同步全量财报",
			CronExpr: "0 5 1 ? * 2-6",
			IsActive: true,
			Params:   `{"source":"eastmoney"}`,
		},
		{
			ID:       TaskShareChangeSync,
			Name:     "同步股本变化",
			CronExpr: "0 40 1 ? * 2-6",
			IsActive: true,
			Params:   `{"source":"eastmoney"}`,
		},
		{
			ID:       TaskShareholderSync,
			Name:     "同步股东人数变化",
			CronExpr: "0 5 2 ? * 2-6",
			IsActive: true,
			Params:   `{"source":"eastmoney"}`,
		},
		{
			ID:       TaskDailySnapshotSync,
			Name:     "同步每日快照",
			CronExpr: "0 0 3 ? * 2-6",
			IsActive: true,
			Params:   `{"code":""}`,
		},
		{
			ID:       TaskWeeklyKlineIncSync,
			Name:     "每周增量同步K线数据",
			CronExpr: "0 0 1 * * 6",
			IsActive: true,
			Params:   `{"periods":"weekly"}`,
		},
		{
			ID:       TaskMonthlyKlineIncSync,
			Name:     "每月增量同步K线数据",
			CronExpr: "0 30 1 1 * ?",
			IsActive: true,
			Params:   `{"periods":"monthly"}`,
		},
		{
			ID:       TaskDividendSync,
			Name:     "同步分红历史",
			CronExpr: "0 5 0 ? * 1-5",
			IsActive: true,
			Params:   `{"source":"eastmoney"}`,
		},
		{
			ID:       TaskDividendKlineSync,
			Name:     "除权K线同步",
			CronExpr: "0 0 22 * * *",
			IsActive: true,
			Params:   `{"periods":"daily,weekly,monthly"}`,
		},
		{
			ID:       TaskNameChangeSync,
			Name:     "同步名称变更",
			CronExpr: "0 10 2 ? * 1-5",
			IsActive: true,
			Params:   `{"source":"eastmoney"}`,
		},
	}
}
