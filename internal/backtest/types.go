package backtest

import "context"

// ExitExecutor 卖股执行器接口。
type ExitExecutor interface {
	Check(ctx context.Context, pos *HoldingPosition, bar DayBar) *ExitDecision
}

// ExitEntryHook 退场入场钩子（可选）。
type ExitEntryHook interface{ OnEntry(pos *HoldingPosition, entryPrice float64, date string) }

// ExitDailyHook 退场每日更新钩子（可选）。
type ExitDailyHook interface{ OnDailyUpdate(pos *HoldingPosition, bar DayBar) }

// PositionManager 仓位分配器接口（有状态，管理 Account）。
type PositionManager interface {
	Allocate(ctx context.Context, candidates []CandidateInfo) ([]Order, error)
	Account() *Account
}

// FeeCalculator 交易费用计算器接口。
type FeeCalculator interface {
	BuyFee(amount float64) float64
	SellFee(amount float64) float64
}

// TradeRecorder 交易记录管理器接口。
type TradeRecorder interface {
	RecordTrade(ctx context.Context, runID uint64, trade Trade) error
	RecordSnapshot(ctx context.Context, snap EquitySnapshot) error
	RecordMetrics(ctx context.Context, runID uint64, metrics RunResult) error
	UpdateProgress(ctx context.Context, runID uint64, pct int) error
}

// HoldingPosition 持仓信息。
type HoldingPosition struct {
	Code          string  // 股票代码
	Quantity      int     // 持仓数量（股）
	EntryPrice    float64 // 入场价格（元）
	EntryDate     string  // 入场日期 YYYY-MM-DD
	HoldDays      int     // 持有天数
	StopLossPrice float64 // 止损触发价（元）
	HighestPrice  float64 // 入场后最高价（元）
}

// Account 资金账户，追踪现金和持仓。
type Account struct {
	Cash     float64             // 可用现金（元）
	Holdings map[string]*HoldingPosition // 当前持仓
}

// NewAccount 创建资金账户。
func NewAccount(initialCash float64) *Account {
	return &Account{Cash: initialCash, Holdings: make(map[string]*HoldingPosition)}
}

// AddCash 增加现金（元）。
func (a *Account) AddCash(amount float64) { a.Cash += amount }

// DeductCash 扣除现金（元）。
func (a *Account) DeductCash(amount float64) { a.Cash -= amount }

// AddHolding 添加持仓。
func (a *Account) AddHolding(p *HoldingPosition) { a.Holdings[p.Code] = p }

// RemoveHolding 移除持仓。
func (a *Account) RemoveHolding(code string) { delete(a.Holdings, code) }

// UpdateHoldingQty 更新持仓数量。
func (a *Account) UpdateHoldingQty(code string, qty int) {
	if p, ok := a.Holdings[code]; ok { p.Quantity = qty }
}

// HoldingCount 返回持仓数量。
func (a *Account) HoldingCount() int { return len(a.Holdings) }

// MarketValue 计算持仓市值（元），需传入当前行情价格。
func (a *Account) MarketValue(prices map[string]float64) float64 {
	mv := 0.0
	for code, pos := range a.Holdings {
		if p, ok := prices[code]; ok { mv += float64(pos.Quantity) * p }
	}
	return mv
}

// TotalEquity 总权益 = 现金 + 市值。
func (a *Account) TotalEquity(prices map[string]float64) float64 { return a.Cash + a.MarketValue(prices) }

// CandidateInfo 候选股票信息。
type CandidateInfo struct{ Code string; Price float64; SignalScore float64; Volatility float64 }

// Order 下单指令。
type Order struct{ Code string; Quantity int; Amount float64 }

// ExitDecision 卖出决策。
type ExitDecision struct{ Price float64; Quantity int; Reason string }
