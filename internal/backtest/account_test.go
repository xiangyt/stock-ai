package backtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccount_NewAccount(t *testing.T) {
	a := NewAccount(100000)
	assert.Equal(t, 100000.0, a.Cash)
	assert.Empty(t, a.Holdings)
}

func TestAccount_AddDeductCash(t *testing.T) {
	a := NewAccount(100000)
	a.AddCash(5000)
	assert.Equal(t, 105000.0, a.Cash)
	a.DeductCash(3000)
	assert.Equal(t, 102000.0, a.Cash)
}

func TestAccount_Holdings(t *testing.T) {
	a := NewAccount(100000)

	a.AddHolding(&HoldingPosition{Code: "000001", Quantity: 100, EntryPrice: 10.0})
	a.AddHolding(&HoldingPosition{Code: "000002", Quantity: 200, EntryPrice: 5.0})

	assert.Equal(t, 2, a.HoldingCount())

	a.UpdateHoldingQty("000001", 50)
	assert.Equal(t, 50, a.Holdings["000001"].Quantity)

	a.RemoveHolding("000002")
	assert.Equal(t, 1, a.HoldingCount())
	assert.Nil(t, a.Holdings["000002"])
}

func TestAccount_MarketValue(t *testing.T) {
	a := NewAccount(100000)
	a.AddHolding(&HoldingPosition{Code: "000001", Quantity: 100})
	a.AddHolding(&HoldingPosition{Code: "000002", Quantity: 200})

	prices := map[string]float64{
		"000001": 10.0, // 100 × 10 = 1000
		"000002": 5.0,  // 200 × 5 = 1000
	}

	assert.Equal(t, 2000.0, a.MarketValue(prices))
}

func TestAccount_MarketValue_PartialPrices(t *testing.T) {
	a := NewAccount(100000)
	a.AddHolding(&HoldingPosition{Code: "000001", Quantity: 100})
	a.AddHolding(&HoldingPosition{Code: "000002", Quantity: 200})

	// 只有 000001 有价格
	prices := map[string]float64{"000001": 10.0}
	assert.Equal(t, 1000.0, a.MarketValue(prices))
}

func TestAccount_TotalEquity(t *testing.T) {
	a := NewAccount(100000) // 现金 100000
	a.AddHolding(&HoldingPosition{Code: "000001", Quantity: 100})

	prices := map[string]float64{"000001": 10.0} // 市值 1000

	assert.Equal(t, 101000.0, a.TotalEquity(prices))
}
