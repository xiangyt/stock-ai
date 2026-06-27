package backtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// testEngine 创建测试用 defaultEngine。
func testEngine() *defaultEngine {
	return &defaultEngine{
		screener:        &noopScreener{},
		feeCalculator:   NewAShareFeeCalculator(2.5, false),
		positionManager: NewEqualPositionManager(100000, 5),
	}
}

type noopScreener struct{}

func (n *noopScreener) Screen(_ context.Context, _ []*StockSnapshot, _ []SignalConfig) (*ScreenSummary, error) {
	return &ScreenSummary{}, nil
}

// ============================================================================
//  calcMetrics
// ============================================================================

func TestCalcMetrics_Normal(t *testing.T) {
	e := testEngine()
	rc := &runContext{
		initialCapital: 100000,
		dailySnapshots: []EquitySnapshot{
			{TotalEquity: 100000}, {TotalEquity: 101000}, {TotalEquity: 102000},
		},
		trades: []Trade{{Type: 2, Profit: 500}, {Type: 2, Profit: -200}},
	}
	result := e.calcMetrics(rc, 100000, 2)
	assert.InDelta(t, 2.0, result.TotalReturn, 0.01)
	assert.InDelta(t, 50.0, result.WinRate, 0.01)
}

func TestCalcMetrics_ProfitFactor(t *testing.T) {
	e := testEngine()
	rc := &runContext{
		initialCapital: 100000,
		dailySnapshots: []EquitySnapshot{{TotalEquity: 100000}, {TotalEquity: 110000}},
		trades:         []Trade{{Type: 2, Profit: 1000}, {Type: 2, Profit: 500}, {Type: 2, Profit: -300}},
	}
	result := e.calcMetrics(rc, 100000, 1)
	assert.InDelta(t, 5.0, result.ProfitFactor, 0.01)
}

func TestCalcMetrics_MaxDrawdown(t *testing.T) {
	e := testEngine()
	rc := &runContext{
		initialCapital: 100000,
		dailySnapshots: []EquitySnapshot{
			{TotalEquity: 100000}, {TotalEquity: 105000},
			{TotalEquity: 102000}, {TotalEquity: 98000},
		},
	}
	result := e.calcMetrics(rc, 100000, 3)
	assert.InDelta(t, 6.667, result.MaxDrawdown, 0.1)
}

func TestCalcMetrics_AnnualReturn(t *testing.T) {
	e := testEngine()
	rc := &runContext{
		initialCapital: 100000,
		dailySnapshots: []EquitySnapshot{{TotalEquity: 100000}, {TotalEquity: 121000}},
	}
	result := e.calcMetrics(rc, 100000, 125)
	assert.InDelta(t, 46.41, result.AnnualReturn, 0.5)
}

func TestCalcMetrics_Empty(t *testing.T) {
	e := testEngine()
	rc := &runContext{initialCapital: 100000}
	result := e.calcMetrics(rc, 100000, 0)
	assert.Empty(t, result.Trades)
}

func TestCalcMetrics_AllProfit(t *testing.T) {
	e := testEngine()
	rc := &runContext{
		initialCapital: 100000,
		dailySnapshots: []EquitySnapshot{{TotalEquity: 100000}, {TotalEquity: 110000}},
		trades:         []Trade{{Type: 2, Profit: 1000}, {Type: 2, Profit: 2000}},
	}
	result := e.calcMetrics(rc, 100000, 1)
	assert.InDelta(t, 100.0, result.WinRate, 0.01)
}

// ============================================================================
//  executeSell
// ============================================================================

func TestExecuteSell_FullQuantity(t *testing.T) {
	e := testEngine()
	acct := e.positionManager.Account()
	acct.AddHolding(&HoldingPosition{Code: "000001", Quantity: 1000, EntryPrice: 10.0})

	rc := &runContext{}
	decision := &ExitDecision{Price: 15.0, Quantity: 1000, Reason: "take_profit"}
	e.executeSell(rc, "000001", acct.Holdings["000001"], decision, "2024-06-01")

	assert.Equal(t, 0, acct.HoldingCount())
	assert.Equal(t, "take_profit", rc.trades[0].Reason)
}

func TestExecuteSell_PartialQuantity(t *testing.T) {
	e := testEngine()
	acct := e.positionManager.Account()
	acct.AddHolding(&HoldingPosition{Code: "000001", Quantity: 1000, EntryPrice: 10.0})

	rc := &runContext{}
	decision := &ExitDecision{Price: 20.0, Quantity: 500, Reason: "stop_loss"}
	e.executeSell(rc, "000001", acct.Holdings["000001"], decision, "2024-06-01")

	assert.Equal(t, 500, acct.Holdings["000001"].Quantity)
}

func TestExecuteSell_ZeroPrice(t *testing.T) {
	e := testEngine()
	acct := e.positionManager.Account()
	acct.AddHolding(&HoldingPosition{Code: "000001", Quantity: 100, EntryPrice: 10.0})

	rc := &runContext{}
	decision := &ExitDecision{Price: 0, Quantity: 100, Reason: "time_exit"}
	e.executeSell(rc, "000001", acct.Holdings["000001"], decision, "2024-06-01")
	assert.Equal(t, 0, acct.HoldingCount())
}

// ============================================================================
//  executeBuy
// ============================================================================

func TestExecuteBuy_Normal(t *testing.T) {
	e := testEngine()
	rc := &runContext{}

	b := &bar{close: 10.0}
	e.executeBuy(rc, b, Order{Code: "000001", Quantity: 1000, Amount: 10000}, "2024-01-01")

	acct := e.positionManager.Account()
	pos := acct.Holdings["000001"]
	assert.NotNil(t, pos)
	assert.Equal(t, 1000, pos.Quantity)
	assert.Equal(t, int8(1), rc.trades[0].Type)
}

func TestExecuteBuy_InsufficientCash(t *testing.T) {
	e := testEngine()
	e.positionManager.Account().DeductCash(99900) // 只剩 100
	rc := &runContext{}

	b := &bar{close: 10.0}
	e.executeBuy(rc, b, Order{Code: "000001", Quantity: 1000, Amount: 10000}, "2024-01-01")

	assert.Equal(t, 0, e.positionManager.Account().HoldingCount())
}

func TestExecuteBuy_ZeroQuantity(t *testing.T) {
	e := testEngine()
	rc := &runContext{}

	b := &bar{close: 10.0}
	e.executeBuy(rc, b, Order{Code: "000001", Quantity: 0, Amount: 0}, "2024-01-01")

	assert.Equal(t, 0, e.positionManager.Account().HoldingCount())
}

// ============================================================================
//  Run 边界测试
// ============================================================================

func TestRun_NoTradingDays(t *testing.T) {
	e := testEngine()
	_, err := e.Run(t.Context(), RunRequest{
		StockPool:      []string{"000001"},
		StartDate:      "2024-13-01",
		EndDate:        "2024-13-31",
		InitialCapital: 100000,
	})
	assert.Error(t, err)
}

func TestRun_NoStocks(t *testing.T) {
	e := testEngine()
	result, err := e.Run(t.Context(), RunRequest{
		StockPool:      []string{},
		StartDate:      "2024-01-02",
		EndDate:        "2024-01-05",
		InitialCapital: 100000,
	})
	if err == nil {
		assert.Equal(t, 0.0, result.TotalReturn)
	}
}

// ============================================================================
//  NewEngine 校验
// ============================================================================

func TestNewEngine_MissingRequired(t *testing.T) {
	_, err := NewEngine()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Screener is required")
}

func TestNewEngine_MissingFeeCalculator(t *testing.T) {
	_, err := NewEngine(WithScreener(NewScreener(nil, 1)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FeeCalculator is required")
}

func TestNewEngine_Valid(t *testing.T) {
	engine, err := NewEngine(
		WithScreener(NewScreener(nil, 1)),
		WithFeeCalculator(NewAShareFeeCalculator(2.5, true)),
	)
	assert.NoError(t, err)
	assert.NotNil(t, engine)
}

func TestRun_Integration(t *testing.T) {
	t.Skip("integration test needs real DB")
}
