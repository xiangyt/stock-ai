package model

import (
	"time"
)

// ========== 数据源配置相关 ==========

// DataSourceConfig 数据源配置表
// 支持多个数据源，每个数据源可独立配置
type DataSourceConfig struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	Name         string     `gorm:"uniqueIndex;size:50;not null" json:"name"` // 数据源标识名: tushare / eastmoney / akshare
	DisplayName  string     `gorm:"size:100" json:"display_name"`             // 显示名称: Tushare Pro / 东方财富 / AKShare
	Type         string     `gorm:"size:20;not null" json:"type"`             // 类型: api / sdk / web_crawl
	Status       string     `gorm:"size:20;default:active" json:"status"`     // 状态: active / disabled / error
	Priority     int        `gorm:"default:0" json:"priority"`                // 优先级，数字越小优先级越高
	Config       string     `gorm:"type:text" json:"config"`                  // JSON格式配置 (API Key、URL等)
	RateLimit    int        `gorm:"default:60" json:"rate_limit"`             // 每分钟请求限制
	DailyQuota   int        `gorm:"default:0" json:"daily_quota"`             // 每日调用配额(0=无限制)
	UsedQuota    int        `gorm:"default:0" json:"used_quota"`              // 已使用配额
	QuotaResetAt *time.Time `json:"quota_reset_at"`                           // 配额重置时间
	Description  string     `gorm:"size:500" json:"description"`              // 描述
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName 表名
func (DataSourceConfig) TableName() string { return "data_source_configs" }

// IsActive 判断是否可用
func (d *DataSourceConfig) IsActive() bool {
	return d.Status == "active"
}

// IsQuotaAvailable 检查配额是否充足
func (d *DataSourceConfig) IsQuotaAvailable() bool {
	if d.DailyQuota <= 0 {
		return true // 无限制
	}
	return d.UsedQuota < d.DailyQuota
}

// ========== 采集任务相关 ==========

// CollectTaskStatus 任务状态常量
const (
	TaskPending   = "pending"
	TaskRunning   = "running"
	TaskCompleted = "completed"
	TaskFailed    = "failed"
	TaskCancelled = "cancelled"
)

// CollectTaskType 任务类型常量
const (
	TaskTypeStockList     = "stock_list"     // 股票列表采集
	TaskTypeStockPrice    = "stock_price"    // 价格采集(单只)
	TaskTypeAllPrices     = "all_prices"     // 全量价格采集
	TaskTypeFinancialData = "financial_data" // 财务数据采集
	TaskTypeHotTopics     = "hot_topics"     // 热门题材采集
)

// CollectTask 采集任务表
type CollectTask struct {
	ID         uint   `gorm:"primarykey" json:"id"`
	TaskID     string `gorm:"uniqueIndex;size:64;not null" json:"task_id"` // 唯一任务ID (UUID)
	Type       string `gorm:"size:30;index;not null" json:"type"`          // 任务类型
	Status     string `gorm:"size:20;index;default:pending" json:"status"` // 状态
	SourceName string `gorm:"size:50;index" json:"source_name"`            // 使用的数据源
	TargetCode string `gorm:"size:20" json:"target_code"`                  // 目标股票代码 (单股采集时)
	Params     string `gorm:"type:text" json:"params"`                     // JSON参数

	// 统计信息
	TotalCount   int `gorm:"default:0" json:"total_count"`   // 总数
	SuccessCount int `gorm:"default:0" json:"success_count"` // 成功数
	FailCount    int `gorm:"default:0" json:"fail_count"`    // 失败数
	SkipCount    int `gorm:"default:0" json:"skip_count"`    // 跳过数

	// 时间记录
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	ErrorMsg   string     `gorm:"type:text" json:"error_msg"` // 错误信息
	CreatedBy  string     `gorm:"size:64" json:"created_by"`  // 创建者
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	// 关联
	Logs []CollectLog `json:"logs,omitempty"`
}

// Duration 计算耗时(ms)
func (t *CollectTask) Duration() int64 {
	if t.StartedAt == nil {
		return 0
	}
	end := time.Now()
	if t.FinishedAt != nil {
		end = *t.FinishedAt
	}
	return end.Sub(*t.StartedAt).Milliseconds()
}

// Progress 计算进度百分比
func (t *CollectTask) Progress() float64 {
	if t.TotalCount == 0 {
		return 0
	}
	return float64(t.SuccessCount+t.FailCount+t.SkipCount) / float64(t.TotalCount) * 100
}

func (CollectTask) TableName() string { return "collect_tasks" }

// CollectLog 采集日志表
type CollectLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	TaskID    uint      `gorm:"index;not null" json:"task_id"`     // 关联任务ID
	Level     string    `gorm:"size:10;index" json:"level"`        // 日志级别: info/warn/error/debug
	Message   string    `gorm:"type:text;not null" json:"message"` // 日志内容
	Code      string    `gorm:"size:20" json:"code"`               // 相关代码(股票代码等)
	Metadata  string    `gorm:"type:text" json:"metadata"`         // 额外元数据(JSON)
	CreatedAt time.Time `json:"created_at"`
}

func (CollectLog) TableName() string { return "collect_logs" }
