package backtest

import (
	"context"
)

// CompositeExitExecutor 组合多个 ExitExecutor，按优先级串行检查。
// 第一个命中的执行器即卖出，后续不再检查。
type CompositeExitExecutor struct {
	executors []ExitExecutor
}

// NewCompositeExitExecutor 创建组合退出执行器。
// 按传入顺序检查，建议按优先级排序（止损 > 止盈 > 到期）。
func NewCompositeExitExecutor(executors ...ExitExecutor) *CompositeExitExecutor {
	return &CompositeExitExecutor{executors: executors}
}

// Check 遍历所有执行器，返回第一个非 nil 的卖出决策。
func (e *CompositeExitExecutor) Check(ctx context.Context, pos *HoldingPosition, bar DayBar) *ExitDecision {
	for _, exec := range e.executors {
		if d := exec.Check(ctx, pos, bar); d != nil {
			return d
		}
	}
	return nil
}

// OnEntry 遍历所有实现了 ExitEntryHook 的执行器，触发入场钩子。
func (e *CompositeExitExecutor) OnEntry(pos *HoldingPosition, entryPrice float64, date string) {
	for _, exec := range e.executors {
		if h, ok := exec.(ExitEntryHook); ok {
			h.OnEntry(pos, entryPrice, date)
		}
	}
}

// OnDailyUpdate 遍历所有实现了 ExitDailyHook 的执行器，触发每日更新钩子。
func (e *CompositeExitExecutor) OnDailyUpdate(pos *HoldingPosition, bar DayBar) {
	for _, exec := range e.executors {
		if h, ok := exec.(ExitDailyHook); ok {
			h.OnDailyUpdate(pos, bar)
		}
	}
}
