package alloc

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

type CustomWeightAllocator struct{ weights map[string]float64 }

type customWeightParams struct{ Weights map[string]float64 `json:"weights"` }

func NewCustomWeightAllocator(params json.RawMessage) (*CustomWeightAllocator, error) {
	var p customWeightParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return &CustomWeightAllocator{weights: p.Weights}, nil
}

func (c *CustomWeightAllocator) Name() string { return "custom_weight" }

func (c *CustomWeightAllocator) Allocate(req *types.AllocRequest) *types.AllocResult {
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
	totalWeight := 0.0
	weights := make([]float64, len(candidates))
	for i, cand := range candidates {
		w, ok := c.weights[cand.Code]
		if !ok || w <= 0 {
			w = 1.0
		}
		weights[i] = w
		totalWeight += w
	}
	if totalWeight <= 0 {
		totalWeight = float64(len(candidates))
	}
	maxInvest := req.MaxInvest()
	var orders []types.AllocOrder
	remain := req.Cash
	for i, cand := range candidates {
		budget := maxInvest * weights[i] / totalWeight
		if req.MaxSinglePct > 0 {
			if cap := req.TotalEquity * req.MaxSinglePct / 100; budget > cap {
				budget = cap
			}
		}
		amount, qty := types.CalcBuyAmount(cand.Price, budget)
		if qty < 100 || amount > remain {
			continue
		}
		orders = append(orders, types.AllocOrder{Code: cand.Code, Price: cand.Price, Amount: amount, Quantity: qty})
		remain -= amount
	}
	return &types.AllocResult{Orders: orders, Remain: remain}
}

func init() {
	types.RegisterAllocator("custom_weight", func(params json.RawMessage) (types.Allocator, error) {
		return NewCustomWeightAllocator(params)
	})
}
