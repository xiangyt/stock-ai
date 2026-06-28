// Package screening 选股引擎（go-micro 风格可插拔架构）。
//
// Engine 是最上层接口，通过 Options 模式注入各子组件：
//   - Screener        选股执行器
//   - PositionManager 仓位分配器
//   - ExitExecutor    卖股执行器
//   - FeeCalculator   交易费用计算器
//   - TradeRecorder   交易记录管理器
//   - DataProvider    数据提供者
//
// 使用示例:
//
//	engine := screening.NewEngine(
//	    screening.WithScreener(myScreener),
//	    screening.WithPositionManager(myPM),
//	    screening.WithExitExecutor(myExit),
//	    screening.WithFeeCalculator(myFee),
//	    screening.WithTradeRecorder(myRecorder),
//	    screening.WithDataProvider(myProvider),
//	)
//	result, err := engine.Run(ctx, request)
package backtest

import "context"

// ============================================================================
//  Engine — 回测引擎（顶层接口）
// ============================================================================

// Engine 回测引擎接口，组合所有子组件，对外暴露单一 Run 方法。
type Engine interface {
	// Initiate 创建回测运行记录并异步启动引擎，返回 runID 供轮询。
	Initiate(ctx context.Context, req RunRequest) (uint64, error)

	// Run 同步执行完整回测（供测试和命令行使用）。
	Run(ctx context.Context, req RunRequest) (*RunResult, error)

	// Cancel 取消正在运行的回测。
	// runID 不存在或已结束则静默返回。
	Cancel(runID uint64)

	// CheckIn 记录前端最近一次拉取进度的时间，用于断连检测。
	CheckIn(runID uint64)
}

// ============================================================================
//  RunRequest — 回测请求参数
// ============================================================================

// RunRequest 回测请求参数。
type RunRequest struct {
	StrategyID     uint64         `json:"strategy_id"`      // 策略ID（必填）
	UID            uint64         `json:"uid"`              // 用户ID
	StockPool      []string       `json:"stock_pool"`       // 股票池代码列表
	StartDate      string         `json:"start_date"`       // 起始日期 YYYY-MM-DD
	EndDate        string         `json:"end_date"`         // 结束日期 YYYY-MM-DD
	InitialCapital float64        `json:"initial_capital"`  // 初始资金（元）
	EntryConfigs   []SignalConfig `json:"entry_configs"`    // 买入信号配置
	ExitConfigs    []ExitRule     `json:"exit_rules"`       // 卖出规则配置
	PositionRules  PositionRules  `json:"position_rules"`   // 仓位规则
	CommissionRate float64        `json:"commission_rate"`  // 手续费率（万分之）
	MinCommission  bool           `json:"min_commission"`   // 是否不免五（佣金最低 5 元）

	// Workload 预估总计算量（交易日数 × 股票数），由 Engine.Run 自动填充。
	// 前端可用此值显示进度条分母。
	Workload int `json:"workload,omitempty"`
}

// ============================================================================
//  RunResult — 回测结果
// ============================================================================

// RunResult 回测结果。
type RunResult struct {
	RunID        uint64           `json:"run_id"`         // 回测运行 ID
	TotalReturn  float64          `json:"total_return"`   // 总收益率（%）
	AnnualReturn float64          `json:"annual_return"`  // 年化收益率（%）
	MaxDrawdown  float64          `json:"max_drawdown"`   // 最大回撤（%）
	SharpeRatio  float64          `json:"sharpe_ratio"`   // 夏普比率
	WinRate      float64          `json:"win_rate"`       // 胜率（%）
	ProfitFactor float64          `json:"profit_factor"`  // 盈亏比
	TradeCount   int              `json:"trade_count"`    // 总交易次数
	Trades       []Trade          `json:"trades"`         // 交易明细
	Snapshots    []EquitySnapshot `json:"snapshots"`      // 每日权益快照
}

// ============================================================================
//  PositionRules — 仓位规则
// ============================================================================

// PositionRules 仓位规则。
type PositionRules struct {
	MaxPositions  int     `json:"max_positions"`  // 最大持仓数
	MaxSinglePct  float64 `json:"max_single_pct"` // 单只股票最大仓位（%总权益）
	CashBufferPct float64 `json:"cash_buffer_pct"` // 现金缓冲比例（%总权益）
	Allocation    string  `json:"allocation"`      // 分配策略 "equal" | "market_cap"
}

// ============================================================================
//  Trade — 交易记录
// ============================================================================

// Trade 交易记录。
type Trade struct {
	Code       string  `json:"stock_code"`               // 股票代码
	Type       int8    `json:"trade_type"`               // 交易类型 1=买入 2=卖出
	Quantity   int     `json:"quantity"`                 // 交易数量（股）
	Price      float64 `json:"price"`                    // 交易价格（元）
	Amount     float64 `json:"amount"`                   // 成交金额（元）
	Commission float64 `json:"commission"`               // 佣金（元）
	StampTax   float64 `json:"stamp_tax"`                // 印花税（元）
	Date       string  `json:"trade_date"`               // 交易日期 YYYY-MM-DD
	Profit     float64 `json:"profit_loss,omitempty"`    // 盈亏金额（元，仅卖出）
	ProfitPct  float64 `json:"profit_loss_pct,omitempty"` // 盈亏比例（%，仅卖出）
	Reason     string  `json:"exit_reason,omitempty"`    // 卖出原因
}

// ============================================================================
//  EquitySnapshot — 每日权益快照
// ============================================================================

// EquitySnapshot 每日权益快照。
type EquitySnapshot struct {
	Date            string  `json:"snap_date"`        // 快照日期 YYYY-MM-DD
	TotalEquity     float64 `json:"total_equity"`     // 总权益（元）
	Cash            float64 `json:"cash"`             // 现金余额（元）
	MarketValue     float64 `json:"market_value"`     // 持仓市值（元）
	PositionCount   int     `json:"position_count"`   // 持仓数量
	DailyReturn     float64 `json:"daily_return"`     // 当日收益率（%）
	CumulativeReturn float64 `json:"cumulative_return"` // 累计收益率（%）
}

// ============================================================================
//  ExitRule — 卖出规则
// ============================================================================

// ExitRule 卖出规则。
type ExitRule struct {
	Type    string         `json:"type"`    // 规则类型 "stop_loss" | "take_profit" | "time_exit" | "signal"
	Enabled bool           `json:"enabled"` // 是否启用
	Params  map[string]any `json:"params"`  // 规则参数
}

// ============================================================================
//  DayBar — 当日行情
// ============================================================================

// DayBar 当日行情快照。
type DayBar struct {
	Date  string  // 日期 YYYY-MM-DD
	Open  float64 // 开盘价（元）
	High  float64 // 最高价（元）
	Low   float64 // 最低价（元）
	Close float64 // 收盘价（元）
}
