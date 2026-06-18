package alloc

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

type EqualAllocator struct{}

func NewEqualAllocator() *EqualAllocator { return &EqualAllocator{} }
func (e *EqualAllocator) Name() string { return "equal" }

func (e *EqualAllocator) Allocate(req *types.AllocRequest) *types.AllocResult {
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
	maxInvest := req.MaxInvest()
	perStock := req.MaxPerStock(maxInvest, len(candidates))
	var orders []types.AllocOrder
	remain := req.Cash
	for _, c := range candidates {
		amount, qty := types.CalcBuyAmount(c.Price, perStock)
		if qty < 100 || amount > remain {
			continue
		}
		orders = append(orders, types.AllocOrder{Code: c.Code, Price: c.Price, Amount: amount, Quantity: qty})
		remain -= amount
	}
	return &types.AllocResult{Orders: orders, Remain: remain}
}

func init() {
	types.RegisterAllocator("equal", func(_ json.RawMessage) (types.Allocator, error) {
		return NewEqualAllocator(), nil
	})
}
