package backtest

import (
	"stock-ai/internal/model"
)

// DayBar 日K线数据（回测引擎使用的简化结构）
type DayBar struct {
	Date  string
	Open  float64
	High  float64
	Low   float64
	Close float64
}

// ============================================================================
//  卖出规则引擎 — ExitChecker 可插拔接口
// ============================================================================

// ExitChecker 卖出规则检查器接口
type ExitChecker interface {
	Check(pos *HoldingPosition, bar DayBar, rules *model.ExitRules) *ExitDecision
	Priority() int
	Name() string
}

// ExitDecision 卖出判定结果
type ExitDecision struct {
	Reason string  // "stop_loss" | "take_profit" | "time_exit" | "signal"
	Price  float64 // 执行价
}

// HoldingPosition 回测持仓（内存对象）
type HoldingPosition struct {
	StockCode     string
	Quantity      int
	EntryPrice    float64
	EntryDate     string
	PreStopLoss   float64
	PreTakeProfit float64
	ExpiryDate    string
	HoldDays      int
}

// exitCheckerChain 优先级链
type exitCheckerChain []ExitChecker

func (c exitCheckerChain) Check(pos *HoldingPosition, bar DayBar, rules *model.ExitRules) *ExitDecision {
	for _, checker := range c {
		if d := checker.Check(pos, bar, rules); d != nil {
			return d
		}
	}
	return nil
}

// StopLossChecker 止损检查（P1，最高优先级）
type StopLossChecker struct{}

func (s *StopLossChecker) Priority() int { return 1 }
func (s *StopLossChecker) Name() string  { return "stop_loss" }

func (s *StopLossChecker) Check(pos *HoldingPosition, bar DayBar, rules *model.ExitRules) *ExitDecision {
	if rules == nil || rules.StopLoss == nil || !rules.StopLoss.Enabled {
		return nil
	}
	if bar.Low <= pos.PreStopLoss {
		execPrice := pos.PreStopLoss * (1 - rules.SlippagePct/100)
		if execPrice < bar.Low {
			execPrice = bar.Low
		}
		return &ExitDecision{Reason: "stop_loss", Price: execPrice}
	}
	return nil
}

// TakeProfitChecker 止盈检查（P2）
type TakeProfitChecker struct{}

func (t *TakeProfitChecker) Priority() int { return 2 }
func (t *TakeProfitChecker) Name() string  { return "take_profit" }

func (t *TakeProfitChecker) Check(pos *HoldingPosition, bar DayBar, rules *model.ExitRules) *ExitDecision {
	if rules == nil || rules.TakeProfit == nil || !rules.TakeProfit.Enabled {
		return nil
	}
	if bar.High >= pos.PreTakeProfit {
		return &ExitDecision{Reason: "take_profit", Price: pos.PreTakeProfit}
	}
	return nil
}

// TimeExitChecker 时间到期检查（P3）
type TimeExitChecker struct{}

func (t *TimeExitChecker) Priority() int { return 3 }
func (t *TimeExitChecker) Name() string  { return "time_exit" }

func (t *TimeExitChecker) Check(pos *HoldingPosition, bar DayBar, rules *model.ExitRules) *ExitDecision {
	if rules == nil || rules.TimeExit == nil || !rules.TimeExit.Enabled {
		return nil
	}
	if bar.Date >= pos.ExpiryDate {
		return &ExitDecision{Reason: "time_exit", Price: bar.Close}
	}
	return nil
}