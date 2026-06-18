package types

import (
	"encoding/json"
	"fmt"

	"stock-ai/internal/model"
)

// ============================================================================
//  仓位管理可插拔架构 — Allocator 接口 (v1.1)
// ============================================================================

// Allocator 资金分配策略接口
type Allocator interface {
	Allocate(req *AllocRequest) *AllocResult
	Name() string
}

// AllocRequest 分配请求
type AllocRequest struct {
	Cash            float64
	TotalEquity     float64
	MaxPositions    int
	CurrentHoldings int
	MaxSinglePct    float64
	CashBufferPct   float64
	Candidates      []CandidateInfo
}

// CandidateInfo 候选股信息
type CandidateInfo struct {
	Code        string
	Price       float64
	SignalScore float64
	Volatility  float64
}

// AllocResult 分配结果
type AllocResult struct {
	Orders []AllocOrder
	Remain float64
}

// AllocOrder 单只股票的分配指令
type AllocOrder struct {
	Code     string
	Price    float64
	Amount   float64
	Quantity int
}

// MaxInvest 计算可投资总额（扣除现金缓冲）
func (r *AllocRequest) MaxInvest() float64 {
	buffer := r.CashBufferPct
	if buffer <= 0 {
		buffer = 5.0
	}
	return r.Cash * (1 - buffer/100)
}

// MaxPerStock 计算单只股票最大投资金额
func (r *AllocRequest) MaxPerStock(totalInvest float64, slots int) float64 {
	if slots <= 0 {
		return totalInvest
	}
	perStock := totalInvest / float64(slots)
	if r.MaxSinglePct > 0 {
		cap := r.TotalEquity * r.MaxSinglePct / 100
		if perStock > cap {
			perStock = cap
		}
	}
	return perStock
}

// CalcBuyAmount 计算买入数量（A股100股整数倍）
func CalcBuyAmount(price float64, budget float64) (amount float64, qty int) {
	if price <= 0 || budget <= 0 {
		return 0, 0
	}
	qty = int(budget/price/100) * 100
	if qty < 100 {
		qty = 100
	}
	amount = float64(qty) * price
	return amount, qty
}

// ============================================================================
//  仓位分配注册表 + 工厂模式
// ============================================================================

// AllocatorFactory 从 JSON params 创建 Allocator
type AllocatorFactory func(params json.RawMessage) (Allocator, error)

var allocatorFactories = map[string]AllocatorFactory{}

// RegisterAllocator 注册分配策略工厂
func RegisterAllocator(allocType string, factory AllocatorFactory) {
	allocatorFactories[allocType] = factory
}

// CreateAllocator 从配置创建 Allocator
func CreateAllocator(cfg model.AllocationConfig) (Allocator, error) {
	factory, ok := allocatorFactories[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("unknown allocation type: %s", cfg.Type)
	}
	return factory(cfg.Params)
}
