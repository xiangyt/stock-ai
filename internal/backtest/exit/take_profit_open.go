package exit

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

// TakeProfitOpenChecker 开盘价止盈 — 用开盘价判断是否触发止盈
// 场景：股票高开直接越过止盈价，应在开盘即卖出，避免盘中回落触发止损
// 优先级 1（最高），确保先于止损检查
type TakeProfitOpenChecker struct {
	thresholdPct float64
	priority     int
}

func NewTakeProfitOpenChecker(params json.RawMessage) (*TakeProfitOpenChecker, error) {
	var p struct{ ThresholdPct float64 `json:"threshold_pct"` }
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return &TakeProfitOpenChecker{thresholdPct: p.ThresholdPct, priority: 1}, nil
}

func (t *TakeProfitOpenChecker) Name() string       { return "take_profit_open" }
func (t *TakeProfitOpenChecker) Priority() int        { return t.priority }
func (t *TakeProfitOpenChecker) SetPriority(p int)    { t.priority = p }

// OnEntry 与 TakeProfitChecker 共享 state key "take_profit"，计算相同的 preTakeProfit
func (t *TakeProfitOpenChecker) OnEntry(pos *types.HoldingPosition, entryPrice float64, _ string) {
	preTakeProfit := entryPrice * (1 + t.thresholdPct/100)
	pos.SetState("take_profit", &takeProfitState{preTakeProfit: preTakeProfit})
}

// Check 用开盘价判断：Open >= preTakeProfit → 触发止盈
func (t *TakeProfitOpenChecker) Check(pos *types.HoldingPosition, bar types.DayBar) *types.ExitDecision {
	st, ok := pos.GetState("take_profit").(*takeProfitState)
	if !ok {
		return nil
	}
	if bar.Open >= st.preTakeProfit {
		return &types.ExitDecision{Reason: "take_profit_open", Price: st.preTakeProfit}
	}
	return nil
}

func init() {
	types.RegisterExitChecker("take_profit_open", func(params json.RawMessage) (types.ExitChecker, error) {
		return NewTakeProfitOpenChecker(params)
	})
}
