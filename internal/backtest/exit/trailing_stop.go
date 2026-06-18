package exit

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

type TrailingStopChecker struct{ trailPct, activationPct float64; priority int }
type trailingState struct{ highWater float64; activated bool }

func NewTrailingStopChecker(params json.RawMessage) (*TrailingStopChecker, error) {
	var p struct {
		TrailPct      float64 `json:"trail_pct"`
		ActivationPct float64 `json:"activation_pct"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return &TrailingStopChecker{trailPct: p.TrailPct, activationPct: p.ActivationPct, priority: 2}, nil
}

func (t *TrailingStopChecker) Name() string     { return "trailing_stop" }
func (t *TrailingStopChecker) Priority() int     { return t.priority }
func (t *TrailingStopChecker) SetPriority(p int)  { t.priority = p }

func (t *TrailingStopChecker) OnEntry(pos *types.HoldingPosition, entryPrice float64, _ string) {
	pos.SetState("trailing_stop", &trailingState{highWater: entryPrice})
}

func (t *TrailingStopChecker) OnDailyUpdate(pos *types.HoldingPosition, bar types.DayBar) {
	st, ok := pos.GetState("trailing_stop").(*trailingState)
	if !ok {
		return
	}
	if bar.High > st.highWater {
		st.highWater = bar.High
	}
	if !st.activated && bar.Close >= pos.EntryPrice*(1+t.activationPct/100) {
		st.activated = true
	}
}

func (t *TrailingStopChecker) Check(pos *types.HoldingPosition, bar types.DayBar) *types.ExitDecision {
	st, ok := pos.GetState("trailing_stop").(*trailingState)
	if !ok || !st.activated {
		return nil
	}
	triggerPrice := st.highWater * (1 - t.trailPct/100)
	if bar.Low <= triggerPrice {
		return &types.ExitDecision{Reason: "trailing_stop", Price: triggerPrice}
	}
	return nil
}

func init() {
	types.RegisterExitChecker("trailing_stop", func(params json.RawMessage) (types.ExitChecker, error) {
		return NewTrailingStopChecker(params)
	})
}
