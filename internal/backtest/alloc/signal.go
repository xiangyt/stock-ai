package alloc

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

type SignalWeightedAllocator struct{}

func NewSignalWeightedAllocator() *SignalWeightedAllocator { return &SignalWeightedAllocator{} }
func (s *SignalWeightedAllocator) Name() string { return "signal_weighted" }

func (s *SignalWeightedAllocator) Allocate(req *types.AllocRequest) *types.AllocResult {
	availableSlots := req.MaxPositions - req.CurrentHoldings
	if req.MaxPositions == 0 {
		availableSlots = len(req.Candidates)
	}
	if availableSlots <= 0 || len(req.Candidates) == 0 {
		return &types.AllocResult{Remain: req.Cash}
	}
	candidates := req.Candidates
	if len(candidates) > availableSlots {
		candidates = candidates[:availableSlots]
	}
	totalScore := 0.0
	for i := range candidates {
		if candidates[i].SignalScore <= 0 {
			candidates[i].SignalScore = 1.0
		}
		totalScore += candidates[i].SignalScore
	}
	if totalScore <= 0 {
		totalScore = float64(len(candidates))
	}
	maxInvest := req.MaxInvest()
	var orders []types.AllocOrder
	remain := req.Cash
	for _, c := range candidates {
		budget := maxInvest * c.SignalScore / totalScore
		if req.MaxSinglePct > 0 {
			if cap := req.TotalEquity * req.MaxSinglePct / 100; budget > cap {
				budget = cap
			}
		}
		amount, qty := types.CalcBuyAmount(c.Price, budget)
		if qty < 100 || amount > remain {
			continue
		}
		orders = append(orders, types.AllocOrder{Code: c.Code, Price: c.Price, Amount: amount, Quantity: qty})
		remain -= amount
	}
	return &types.AllocResult{Orders: orders, Remain: remain}
}

func init() {
	types.RegisterAllocator("signal_weighted", func(_ json.RawMessage) (types.Allocator, error) {
		return NewSignalWeightedAllocator(), nil
	})
}
