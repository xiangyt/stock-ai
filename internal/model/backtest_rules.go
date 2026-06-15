package model

// ============================================================================
//  回测 — 卖出规则与仓位管理类型
//  持久化: strategies.exit_rules / strategies.position_rules (JSON)
// ============================================================================

// ExitRules 卖出规则集
type ExitRules struct {
	StopLoss    *StopLossRule  `json:"stop_loss"`
	TakeProfit  *TakeProfitRule `json:"take_profit"`
	TimeExit    *TimeExitRule  `json:"time_exit"`
	ExitSignals []any    `json:"exit_signals"` // P2 预留，存 JSON 数组
	SlippagePct float64        `json:"slippage_pct"` // 滑点百分比，仅止损生效，默认 0.3
}

// StopLossRule 止损规则
type StopLossRule struct {
	Enabled      bool    `json:"enabled"`
	ThresholdPct float64 `json:"threshold_pct"` // 负数, 如 -8.0
}

// TakeProfitRule 止盈规则
type TakeProfitRule struct {
	Enabled      bool    `json:"enabled"`
	ThresholdPct float64 `json:"threshold_pct"` // 正数, 如 20.0
}

// TimeExitRule 时间退出规则
type TimeExitRule struct {
	Enabled  bool `json:"enabled"`
	HoldDays int  `json:"hold_days"` // 0 表示不启用
}

// PositionRules 仓位管理规则
type PositionRules struct {
	MaxPositions int     `json:"max_positions"`  // 0=不限制
	MaxSinglePct float64 `json:"max_single_pct"` // 0=不限制
	Allocation   string  `json:"allocation"`      // "equal" | 预留 "signal_strength"
}