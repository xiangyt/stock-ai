package exit

import (
	"encoding/json"
	"math"

	"stock-ai/internal/backtest/types"
)

type SegmentProfitChecker struct{ levels []segmentLevel; priority int }
type segmentLevel struct {
	ThresholdPct float64 `json:"threshold_pct"`
	SellRatio    float64 `json:"sell_ratio"`
}
type segmentState struct{ triggered []bool; originalQty int }

func NewSegmentProfitChecker(params json.RawMessage) (*SegmentProfitChecker, error) {
	var p struct{ Levels []segmentLevel `json:"levels"` }
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return &SegmentProfitChecker{levels: p.Levels, priority: 2}, nil
}

func (s *SegmentProfitChecker) Name() string     { return "segment_profit" }
func (s *SegmentProfitChecker) Priority() int     { return s.priority }
func (s *SegmentProfitChecker) SetPriority(p int)  { s.priority = p }

func (s *SegmentProfitChecker) OnEntry(pos *types.HoldingPosition, _ float64, _ string) {
	pos.SetState("segment_profit", &segmentState{
		triggered: make([]bool, len(s.levels)), originalQty: pos.Quantity,
	})
}

func (s *SegmentProfitChecker) Check(pos *types.HoldingPosition, bar types.DayBar) *types.ExitDecision {
	st, ok := pos.GetState("segment_profit").(*segmentState)
	if !ok {
		return nil
	}
	allTriggered := true
	for i, level := range s.levels {
		if st.triggered[i] {
			continue
		}
		allTriggered = false
		if bar.High >= pos.EntryPrice*(1+level.ThresholdPct/100) {
			st.triggered[i] = true
			sellQty := int(math.Floor(float64(st.originalQty) * level.SellRatio))
			if sellQty < 1 {
				sellQty = 1
			}
			if sellQty >= pos.Quantity {
				return &types.ExitDecision{Reason: "segment_profit", Price: pos.EntryPrice * (1 + level.ThresholdPct/100)}
			}
			return &types.ExitDecision{Reason: "segment_profit", Price: pos.EntryPrice * (1 + level.ThresholdPct/100), Quantity: sellQty}
		}
	}
	if allTriggered && pos.Quantity > 0 {
		return &types.ExitDecision{Reason: "segment_profit", Price: bar.Close}
	}
	return nil
}

func init() {
	types.RegisterExitChecker("segment_profit", func(params json.RawMessage) (types.ExitChecker, error) {
		return NewSegmentProfitChecker(params)
	})
}
