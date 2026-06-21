package exit

import (
	"encoding/json"
	"time"

	"stock-ai/internal/backtest/types"
)

type TimeExitChecker struct{ holdDays, priority int }
type timeExitState struct{ expiryDate string }

func NewTimeExitChecker(params json.RawMessage) (*TimeExitChecker, error) {
	var p struct{ HoldDays int `json:"hold_days"` }
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return &TimeExitChecker{holdDays: p.HoldDays, priority: 4}, nil
}

func (t *TimeExitChecker) Name() string     { return "time_exit" }
func (t *TimeExitChecker) Priority() int     { return t.priority }
func (t *TimeExitChecker) SetPriority(p int)  { t.priority = p }

func (t *TimeExitChecker) OnEntry(pos *types.HoldingPosition, _ float64, entryDate string) {
	entryTime, _ := time.Parse("2006-01-02", entryDate)
	expiry := entryTime.AddDate(0, 0, t.holdDays)
	pos.SetState("time_exit", &timeExitState{expiryDate: expiry.Format("2006-01-02")})
}

func (t *TimeExitChecker) Check(pos *types.HoldingPosition, bar types.DayBar) *types.ExitDecision {
	st, ok := pos.GetState("time_exit").(*timeExitState)
	if !ok {
		return nil
	}
	if bar.Date >= st.expiryDate {
		return &types.ExitDecision{Reason: "time_exit", Price: bar.Close}
	}
	return nil
}

func init() {
	types.RegisterExitChecker("time_exit", func(params json.RawMessage) (types.ExitChecker, error) {
		return NewTimeExitChecker(params)
	})
}
