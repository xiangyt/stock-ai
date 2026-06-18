package exit

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

type StopLossChecker struct {
	thresholdPct float64
	slippagePct  float64
	priority     int
}

type stopLossState struct{ preStopLoss float64 }

func NewStopLossChecker(params json.RawMessage) (*StopLossChecker, error) {
	var p struct{ ThresholdPct float64 `json:"threshold_pct"` }
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return &StopLossChecker{thresholdPct: p.ThresholdPct, slippagePct: 0.3, priority: 1}, nil
}

func (s *StopLossChecker) Name() string      { return "stop_loss" }
func (s *StopLossChecker) Priority() int       { return s.priority }
func (s *StopLossChecker) SetPriority(p int)   { s.priority = p }
func (s *StopLossChecker) SetSlippage(p float64) { s.slippagePct = p }

func (s *StopLossChecker) OnEntry(pos *types.HoldingPosition, entryPrice float64, _ string) {
	preStopLoss := entryPrice * (1 + s.thresholdPct/100)
	pos.SetState("stop_loss", &stopLossState{preStopLoss: preStopLoss})
}

func (s *StopLossChecker) Check(pos *types.HoldingPosition, bar types.DayBar) *types.ExitDecision {
	st, ok := pos.GetState("stop_loss").(*stopLossState)
	if !ok {
		return nil
	}
	if bar.Low <= st.preStopLoss {
		execPrice := st.preStopLoss * (1 - s.slippagePct/100)
		if execPrice < bar.Low {
			execPrice = bar.Low
		}
		return &types.ExitDecision{Reason: "stop_loss", Price: execPrice}
	}
	return nil
}

func init() {
	types.RegisterExitChecker("stop_loss", func(params json.RawMessage) (types.ExitChecker, error) {
		return NewStopLossChecker(params)
	})
}
