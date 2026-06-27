package backtest

import "context"

// StopLossExecutor 止损执行器。
type StopLossExecutor struct{ lossPct, slippagePct float64 }

// NewStopLossExecutor 创建止损执行器。lossPct 为亏损百分比（正数）。
func NewStopLossExecutor(lossPct, slippagePct float64) *StopLossExecutor {
	return &StopLossExecutor{lossPct: lossPct, slippagePct: slippagePct}
}

// OnEntry 记录止损价格。
func (e *StopLossExecutor) OnEntry(pos *HoldingPosition, entryPrice float64, _ string) {
	pos.StopLossPrice = entryPrice * (1 - e.lossPct/100)
}

// OnDailyUpdate 每日更新（无操作）。
func (e *StopLossExecutor) OnDailyUpdate(_ *HoldingPosition, _ DayBar) {}

// Check 实现 ExitExecutor。
func (e *StopLossExecutor) Check(_ context.Context, pos *HoldingPosition, bar DayBar) *ExitDecision {
	if pos.StopLossPrice <= 0 || bar.Low > pos.StopLossPrice {
		return nil
	}
	execPrice := pos.StopLossPrice * (1 - e.slippagePct/100)
	if execPrice < bar.Low {
		execPrice = bar.Low
	}
	return &ExitDecision{Price: execPrice, Quantity: 0, Reason: "stop_loss"}
}

// TakeProfitExecutor 止盈执行器。
type TakeProfitExecutor struct{ profitPct, slippagePct float64 }

// NewTakeProfitExecutor 创建止盈执行器。profitPct 为盈利百分比（正数）。
func NewTakeProfitExecutor(profitPct, slippagePct float64) *TakeProfitExecutor {
	return &TakeProfitExecutor{profitPct: profitPct, slippagePct: slippagePct}
}

// OnEntry 记录止盈价格。
func (e *TakeProfitExecutor) OnEntry(pos *HoldingPosition, entryPrice float64, _ string) {
	pos.HighestPrice = entryPrice * (1 + e.profitPct/100)
}

// OnDailyUpdate 每日更新（无操作）。
func (e *TakeProfitExecutor) OnDailyUpdate(_ *HoldingPosition, _ DayBar) {}

// Check 实现 ExitExecutor。
func (e *TakeProfitExecutor) Check(_ context.Context, pos *HoldingPosition, bar DayBar) *ExitDecision {
	if pos.HighestPrice <= 0 || bar.High < pos.HighestPrice {
		return nil
	}
	execPrice := pos.HighestPrice * (1 - e.slippagePct/100)
	if execPrice < bar.Low {
		execPrice = bar.Low
	}
	return &ExitDecision{Price: execPrice, Quantity: 0, Reason: "take_profit"}
}

// NextDayExitExecutor 次日卖出执行器。
type NextDayExitExecutor struct{ MinHoldDays int }

// NewNextDayExitExecutor 创建次日卖出执行器。
func NewNextDayExitExecutor() *NextDayExitExecutor { return &NextDayExitExecutor{MinHoldDays: 1} }

// Check 实现 ExitExecutor。
func (e *NextDayExitExecutor) Check(
	_ context.Context, pos *HoldingPosition, bar DayBar,
) *ExitDecision {
	pos.HoldDays++
	if pos.HoldDays <= e.MinHoldDays {
		return nil
	}
	return &ExitDecision{Price: bar.Close, Quantity: 0, Reason: "time_exit"}
}

// NeverExitExecutor 永不出场执行器。
type NeverExitExecutor struct{}

// NewNeverExitExecutor 创建永不出场执行器。
func NewNeverExitExecutor() *NeverExitExecutor { return &NeverExitExecutor{} }

// Check 实现 ExitExecutor，永远不卖出。
func (e *NeverExitExecutor) Check(
	_ context.Context, _ *HoldingPosition, _ DayBar,
) *ExitDecision {
	return nil
}
