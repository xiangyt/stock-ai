package alloc

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

type VolatilityWeightedAllocator struct{}

func NewVolatilityWeightedAllocator() *VolatilityWeightedAllocator { return &VolatilityWeightedAllocator{} }
func (v *VolatilityWeightedAllocator) Name() string { return "volatility_weighted" }

func (v *VolatilityWeightedAllocator) Allocate(req *types.AllocRequest) *types.AllocResult {
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
	weights, totalWeight := make([]float64, len(candidates)), 0.0
	for i, c := range candidates {
		vol := c.Volatility
		if vol <= 0 {
			vol = 1.0
		}
		weights[i] = 1.0 / vol
		totalWeight += weights[i]
	}
	if totalWeight <= 0 {
		totalWeight = float64(len(candidates))
	}
	maxInvest := req.MaxInvest()
	var orders []types.AllocOrder
	remain := req.Cash
	for i, c := range candidates {
		budget := maxInvest * weights[i] / totalWeight
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
	types.RegisterAllocator("volatility_weighted", func(_ json.RawMessage) (types.Allocator, error) {
		return NewVolatilityWeightedAllocator(), nil
	})
}
