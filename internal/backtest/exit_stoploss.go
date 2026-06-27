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

// OnEntry 入场钩子（无操作，目标价在 Check 中计算）。
func (e *TakeProfitExecutor) OnEntry(_ *HoldingPosition, _ float64, _ string) {}

// OnDailyUpdate 每日更新（无操作）。
func (e *TakeProfitExecutor) OnDailyUpdate(_ *HoldingPosition, _ DayBar) {}

// Check 实现 ExitExecutor。当日最高价 >= 止盈目标价时触发。
func (e *TakeProfitExecutor) Check(_ context.Context, pos *HoldingPosition, bar DayBar) *ExitDecision {
	target := pos.EntryPrice * (1 + e.profitPct/100)
	if bar.High < target {
		return nil
	}
	execPrice := target * (1 - e.slippagePct/100)
	if execPrice < bar.Low {
		execPrice = bar.Low
	}
	return &ExitDecision{Price: execPrice, Quantity: 0, Reason: "take_profit"}
}

// NextDayExitExecutor 次日卖出执行器。
type NextDayExitExecutor struct{ MinHoldDays int }

// NewNextDayExitExecutor 创建次日卖出执行器。
func NewNextDayExitExecutor() *NextDayExitExecutor { return &NextDayExitExecutor{MinHoldDays: 1} }

// NewNextDayExitExecutorWithDays 创建指定持有天数的退出执行器。
func NewNextDayExitExecutorWithDays(days int) *NextDayExitExecutor { return &NextDayExitExecutor{MinHoldDays: days} }

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
func (e *NeverExitExecutor) Check(_ context.Context, _ *HoldingPosition, _ DayBar) *ExitDecision {
	return nil
}

// ============================================================================
//  TrailingStopExecutor 移动止盈执行器
//
//  盈利达到 activationPct% 后激活，此后从最高价回撤 trailPct% 触发卖出。
// ============================================================================

// TrailingStopExecutor 移动止盈执行器。
type TrailingStopExecutor struct{ trailPct, activationPct float64 }

// NewTrailingStopExecutor 创建移动止盈执行器。
func NewTrailingStopExecutor(trailPct, activationPct float64) *TrailingStopExecutor {
	return &TrailingStopExecutor{trailPct: trailPct, activationPct: activationPct}
}

// OnEntry 入场时记录初始最高价为入场价，未激活。
func (e *TrailingStopExecutor) OnEntry(pos *HoldingPosition, _ float64, _ string) {
	pos.HighestPrice = 0
	pos.TrailActive = false
}

// OnDailyUpdate 每日更新最高价，判断是否激活。
func (e *TrailingStopExecutor) OnDailyUpdate(pos *HoldingPosition, bar DayBar) {
	if pos.HighestPrice == 0 {
		pos.HighestPrice = pos.EntryPrice
	}
	if bar.High > pos.HighestPrice {
		pos.HighestPrice = bar.High
	}
	if !pos.TrailActive && pos.EntryPrice > 0 {
		profitPct := (pos.HighestPrice - pos.EntryPrice) / pos.EntryPrice * 100
		if profitPct >= e.activationPct {
			pos.TrailActive = true
		}
	}
}

// Check 实现 ExitExecutor。激活后从最高价回撤 trailPct% 触发卖出。
func (e *TrailingStopExecutor) Check(_ context.Context, pos *HoldingPosition, bar DayBar) *ExitDecision {
	if !pos.TrailActive || pos.HighestPrice <= 0 {
		return nil
	}
	triggerPrice := pos.HighestPrice * (1 - e.trailPct/100)
	if bar.Close > triggerPrice {
		return nil
	}
	return &ExitDecision{Price: bar.Close, Quantity: 0, Reason: "trailing_stop"}
}

// ============================================================================
//  SegmentProfitExecutor 分段止盈执行器
//
//  在多个盈利阈值处分批卖出不同比例，每层只触发一次。
// ============================================================================

// segmentLevel 单层止盈参数。
type segmentLevel struct{ sellRatio, thresholdPct float64 }

// SegmentProfitExecutor 分段止盈执行器。
type SegmentProfitExecutor struct{ levels []segmentLevel }

// NewSegmentProfitExecutor 创建分段止盈执行器。
// levels 示例: [{sellRatio: 0.5, thresholdPct: 10}, {sellRatio: 0.5, thresholdPct: 20}]
func NewSegmentProfitExecutor(levels []struct{ SellRatio, ThresholdPct float64 }) *SegmentProfitExecutor {
	e := &SegmentProfitExecutor{levels: make([]segmentLevel, len(levels))}
	for i, l := range levels {
		e.levels[i] = segmentLevel{sellRatio: l.SellRatio, thresholdPct: l.ThresholdPct}
	}
	return e
}

// OnEntry 入场时重置层级计数。
func (e *SegmentProfitExecutor) OnEntry(pos *HoldingPosition, _ float64, _ string) {
	pos.SegTriggered = 0
}

// OnDailyUpdate 每日更新（无操作）。
func (e *SegmentProfitExecutor) OnDailyUpdate(_ *HoldingPosition, _ DayBar) {}

// Check 实现 ExitExecutor。检查是否达到下一个未触发的盈利层级。
func (e *SegmentProfitExecutor) Check(_ context.Context, pos *HoldingPosition, bar DayBar) *ExitDecision {
	if pos.EntryPrice <= 0 {
		return nil
	}
	for i, lv := range e.levels {
		mask := 1 << i
		if pos.SegTriggered&mask != 0 {
			continue // 该层已触发
		}
		profitPct := (bar.Close - pos.EntryPrice) / pos.EntryPrice * 100
		if profitPct >= lv.thresholdPct {
			pos.SegTriggered |= mask
			qty := int(float64(pos.Quantity) * lv.sellRatio)
			if qty <= 0 {
				qty = pos.Quantity
			}
			return &ExitDecision{Price: bar.Close, Quantity: qty, Reason: "segment_profit"}
		}
	}
	return nil
}
