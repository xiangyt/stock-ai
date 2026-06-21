package exit

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

type TakeProfitChecker struct {
	thresholdPct float64
	priority     int
}

type takeProfitState struct{ preTakeProfit float64 }

func NewTakeProfitChecker(params json.RawMessage) (*TakeProfitChecker, error) {
	var p struct{ ThresholdPct float64 `json:"threshold_pct"` }
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return &TakeProfitChecker{thresholdPct: p.ThresholdPct, priority: 3}, nil
}

func (t *TakeProfitChecker) Name() string     { return "take_profit" }
func (t *TakeProfitChecker) Priority() int     { return t.priority }
func (t *TakeProfitChecker) SetPriority(p int)  { t.priority = p }

func (t *TakeProfitChecker) OnEntry(pos *types.HoldingPosition, entryPrice float64, _ string) {
	preTakeProfit := entryPrice * (1 + t.thresholdPct/100)
	pos.SetState("take_profit", &takeProfitState{preTakeProfit: preTakeProfit})
}

func (t *TakeProfitChecker) Check(pos *types.HoldingPosition, bar types.DayBar) *types.ExitDecision {
	st, ok := pos.GetState("take_profit").(*takeProfitState)
	if !ok {
		return nil
	}
	if bar.High >= st.preTakeProfit {
		return &types.ExitDecision{Reason: "take_profit", Price: st.preTakeProfit}
	}
	return nil
}

func init() {
	types.RegisterExitChecker("take_profit", func(params json.RawMessage) (types.ExitChecker, error) {
		return NewTakeProfitChecker(params)
	})
}
