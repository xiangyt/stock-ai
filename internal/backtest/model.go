package backtest

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// DateOnly 纯日期类型 (YYYY-MM-DD)，用于 DB date 列与 JSON 序列化
type DateOnly struct {
	time.Time
}

// Scan 实现 sql.Scanner，从 DB 读取 date 列
func (d *DateOnly) Scan(value interface{}) error {
	if value == nil {
		d.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		d.Time = v
	case string:
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return err
		}
		d.Time = t
	default:
		return fmt.Errorf("DateOnly.Scan: unsupported type %T", value)
	}
	return nil
}

// Value 实现 driver.Valuer，写入 DB date 列
func (d DateOnly) Value() (driver.Value, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Time.Format("2006-01-02"), nil
}

// MarshalJSON 序列化为 "YYYY-MM-DD"
func (d DateOnly) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	s := fmt.Sprintf("\"%s\"", d.Time.Format("2006-01-02"))
	return []byte(s), nil
}

// UnmarshalJSON 从 "YYYY-MM-DD" 反序列化
func (d *DateOnly) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" || s == `""` {
		d.Time = time.Time{}
		return nil
	}
	// 去掉引号
	s = s[1 : len(s)-1]
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// String 返回 YYYY-MM-DD
func (d DateOnly) String() string {
	if d.Time.IsZero() {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// NewDateOnly 从 "YYYY-MM-DD" 字符串创建 DateOnly
func NewDateOnly(s string) DateOnly {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return DateOnly{}
	}
	return DateOnly{Time: t}
}

// ============================================================================
//  回测数据模型
// ============================================================================

// RunStatus 回测运行状态
type RunStatus string

const (
	StatusPending RunStatus = "pending"
	StatusRunning RunStatus = "running"
	StatusDone    RunStatus = "done"
	StatusFailed  RunStatus = "failed"
	StatusStopped RunStatus = "stopped" // 手动或前端断连停止
)

// BacktestRun 回测运行记录
type BacktestRun struct {
	ID              uint64    `gorm:"primarykey" json:"id"`
	StrategyID      uint64    `gorm:"not null;index" json:"strategy_id"`
	UID             uint64    `gorm:"default:0" json:"uid"`
	StockPool       string    `gorm:"type:json;not null" json:"stock_pool"`
	StartDate       string    `gorm:"type:date;not null" json:"start_date"`
	EndDate         string    `gorm:"type:date;not null" json:"end_date"`
	InitialCapital  float64   `gorm:"type:decimal(20,4);not null" json:"initial_capital"`
	FinalEquity     *float64  `gorm:"type:decimal(20,4)" json:"final_equity"`
	ExitRules       string    `gorm:"type:json" json:"exit_rules"`
	PositionRules   string    `gorm:"type:json" json:"position_rules"`
	Status          string    `gorm:"size:20;default:pending" json:"status"`
	ProgressPct     int       `gorm:"default:0" json:"progress_pct"`
	ErrorMessage    string    `gorm:"type:text" json:"error_message,omitempty"`
	TotalReturn     *float64  `gorm:"type:decimal(10,4)" json:"total_return"`
	AnnualReturn    *float64  `gorm:"type:decimal(10,4)" json:"annual_return"`
	MaxDrawdown     *float64  `gorm:"type:decimal(10,4)" json:"max_drawdown"`
	SharpeRatio     *float64  `gorm:"type:decimal(10,4)" json:"sharpe_ratio"`
	WinRate         *float64  `gorm:"type:decimal(10,4)" json:"win_rate"`
	ProfitFactor    *float64  `gorm:"type:decimal(10,4)" json:"profit_factor"`
	TradeCount      int       `gorm:"default:0" json:"trade_count"`
	StopLossCount   int       `gorm:"default:0" json:"stop_loss_count"`
	TakeProfitCount int       `gorm:"default:0" json:"take_profit_count"`
	TimeExitCount   int       `gorm:"default:0" json:"time_exit_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (BacktestRun) TableName() string { return "backtest_runs" }

// BacktestTrade 回测交易明细
type BacktestTrade struct {
	ID            uint64    `gorm:"primarykey" json:"id"`
	RunID         uint64    `gorm:"not null;index" json:"run_id"`
	StockCode     string    `gorm:"size:6;not null" json:"stock_code"`
	StockName     string    `gorm:"-" json:"stock_name"` // 非DB字段，查询时填充
	TradeType     int8      `gorm:"not null" json:"trade_type"` // 1=买入 2=卖出
	Quantity      int       `gorm:"not null" json:"quantity"`
	Price         float64   `gorm:"type:decimal(14,4);not null" json:"price"`
	Amount        float64   `gorm:"type:decimal(20,4);not null" json:"amount"`
	Commission    float64   `gorm:"type:decimal(14,4);default:0" json:"commission"`
	StampTax      float64   `gorm:"type:decimal(14,4);default:0" json:"stamp_tax"`
	TradeDate     DateOnly  `gorm:"type:date;not null" json:"trade_date"`
	ExitReason    *string   `gorm:"size:20" json:"exit_reason"`
	PreExitPrice  *float64  `gorm:"type:decimal(14,4)" json:"pre_exit_price"`
	ProfitLoss    *float64  `gorm:"type:decimal(14,4)" json:"profit_loss"`
	ProfitLossPct *float64  `gorm:"type:decimal(10,4)" json:"profit_loss_pct"`
	CreatedAt     time.Time `json:"created_at"`
}

func (BacktestTrade) TableName() string { return "backtest_trades" }

// DailySnapshot 每日净值快照
type DailySnapshot struct {
	ID               uint64    `gorm:"primarykey" json:"id"`
	RunID            uint64    `gorm:"not null;index" json:"run_id"`
	SnapDate         string    `gorm:"type:date;not null" json:"snap_date"`
	TotalEquity      float64   `gorm:"type:decimal(20,4);not null" json:"total_equity"`
	Cash             float64   `gorm:"type:decimal(20,4);not null" json:"cash"`
	MarketValue      float64   `gorm:"type:decimal(20,4);not null" json:"market_value"`
	PositionCount    int       `gorm:"default:0" json:"position_count"`
	DailyReturn      *float64  `gorm:"type:decimal(10,4)" json:"daily_return"`
	CumulativeReturn *float64  `gorm:"type:decimal(10,4)" json:"cumulative_return"`
	BenchmarkValue   *float64  `gorm:"type:decimal(20,4)" json:"benchmark_value"`
	CreatedAt        time.Time `json:"created_at"`
}

func (DailySnapshot) TableName() string { return "daily_snapshots" }