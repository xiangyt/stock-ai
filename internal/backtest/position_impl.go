package backtest

import (
	"context"
	"math"
)

// EqualPositionManager 等权仓位分配器。
type EqualPositionManager struct {
	acct          *Account // 资金账户
	MaxPositions  int      // 最大持仓数
	MaxSinglePct  float64  // 单只最大仓位（%）
	CashBufferPct float64  // 现金缓冲比例（%）
	MinLot        int      // 最小买入单位（股）
}

// NewEqualPositionManager 创建等权分配器。
func NewEqualPositionManager(initialCash float64, maxPositions int) *EqualPositionManager {
	return &EqualPositionManager{
		acct: NewAccount(initialCash), MaxPositions: maxPositions, MinLot: 100,
	}
}

// Account 实现 PositionManager。
func (e *EqualPositionManager) Account() *Account { return e.acct }

// Allocate 实现 PositionManager，等权分配现金。
func (e *EqualPositionManager) Allocate(
	_ context.Context, candidates []CandidateInfo,
) ([]Order, error) {
	slots := e.MaxPositions - e.acct.HoldingCount()
	if e.MaxPositions == 0 { slots = len(candidates) }
	if slots <= 0 || len(candidates) == 0 { return nil, nil }
	cash := e.acct.Cash * (1 - e.CashBufferPct)
	perStock := cash / float64(slots)
	lot := e.MinLot
	if lot <= 0 { lot = 100 }
	remain := cash
	orders := make([]Order, 0, slots)
	for i := 0; i < len(candidates) && i < slots; i++ {
		c := candidates[i]
		if c.Price <= 0 { continue }
		qty := int(math.Floor(perStock/c.Price/float64(lot))) * lot
		if qty < lot { qty = lot }
		amount := float64(qty) * c.Price
		if amount > remain { continue }
		remain -= amount
		orders = append(orders, Order{Code: c.Code, Quantity: qty, Amount: amount})
	}
	return orders, nil
}

const minBuyAmount = 20000.0

// UnlimitedPositionManager 无限资金仓位分配器。
type UnlimitedPositionManager struct {
	acct   *Account // 资金账户
	MinLot int      // 最小买入单位（股）
}

// NewUnlimitedPositionManager 创建无限资金分配器。
func NewUnlimitedPositionManager(initialCash float64) *UnlimitedPositionManager {
	return &UnlimitedPositionManager{acct: NewAccount(initialCash), MinLot: 100}
}

// Account 实现 PositionManager。
func (u *UnlimitedPositionManager) Account() *Account { return u.acct }

// Allocate 实现 PositionManager，每只候选买入 ≥20000 元的最小整百股。
func (u *UnlimitedPositionManager) Allocate(
	_ context.Context, candidates []CandidateInfo,
) ([]Order, error) {
	if len(candidates) == 0 { return nil, nil }
	lot := u.MinLot
	if lot <= 0 { lot = 100 }
	remain := u.acct.Cash
	orders := make([]Order, 0, len(candidates))
	for _, c := range candidates {
		if c.Price <= 0 || remain <= 0 { continue }
		minQty := int(math.Ceil(minBuyAmount/c.Price/float64(lot))) * lot
		maxQty := int(math.Floor(remain/c.Price/float64(lot))) * lot
		qty := minQty
		if qty > maxQty { qty = maxQty }
		if qty < lot { continue }
		amount := float64(qty) * c.Price
		if amount > remain { continue }
		remain -= amount
		orders = append(orders, Order{Code: c.Code, Quantity: qty, Amount: amount})
	}
	return orders, nil
}

// SignalWeightPositionManager 信号评分加权分配器。
type SignalWeightPositionManager struct {
	acct          *Account // 资金账户
	MaxPositions  int      // 最大持仓数
	CashBufferPct float64  // 现金缓冲比例（%）
	MinLot        int      // 最小买入单位（股）
}

// NewSignalWeightPositionManager 创建信号加权分配器。
func NewSignalWeightPositionManager(initialCash float64, maxPositions int) *SignalWeightPositionManager {
	return &SignalWeightPositionManager{
		acct: NewAccount(initialCash), MaxPositions: maxPositions, MinLot: 100,
	}
}

// Account 实现 PositionManager。
func (s *SignalWeightPositionManager) Account() *Account { return s.acct }

// Allocate 实现 PositionManager，按信号评分加权分配。
func (s *SignalWeightPositionManager) Allocate(
	_ context.Context, candidates []CandidateInfo,
) ([]Order, error) {
	slots := s.MaxPositions - s.acct.HoldingCount()
	if s.MaxPositions == 0 { slots = len(candidates) }
	if slots <= 0 || len(candidates) == 0 { return nil, nil }
	totalScore := 0.0
	for _, c := range candidates {
		sc := c.SignalScore
		if sc <= 0 { sc = 1.0 }
		totalScore += sc
	}
	if totalScore <= 0 { return nil, nil }
	fullCash := s.acct.Cash * (1 - s.CashBufferPct)
	lot := s.MinLot
	if lot <= 0 { lot = 100 }
	if len(candidates) > slots { candidates = candidates[:slots] }
	remain := fullCash
	orders := make([]Order, 0, len(candidates))
	for _, c := range candidates {
		if c.Price <= 0 { continue }
		w := c.SignalScore
		if w <= 0 { w = 1.0 }
		alloc := fullCash * w / totalScore
		qty := int(math.Floor(alloc/c.Price/float64(lot))) * lot
		if qty < lot { continue }
		amount := float64(qty) * c.Price
		if amount > remain { continue }
		remain -= amount
		orders = append(orders, Order{Code: c.Code, Quantity: qty, Amount: amount})
	}
	return orders, nil
}
