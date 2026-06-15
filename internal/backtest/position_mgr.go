package backtest

import (
	"math"
	"stock-ai/internal/model"
)

// ============================================================================
//  持仓管理器 — 建仓/平仓/预卖出价计算
// ============================================================================

// calcEntryExitPrices 计算建仓时的预止损价和预止盈价
func calcEntryExitPrices(entryPrice float64, rules *model.ExitRules, expiryDate string) (preStopLoss, preTakeProfit float64) {
	if rules.StopLoss != nil && rules.StopLoss.Enabled {
		preStopLoss = entryPrice * (1 + rules.StopLoss.ThresholdPct/100)
		preStopLoss = math.Round(preStopLoss*100) / 100
	}
	if rules.TakeProfit != nil && rules.TakeProfit.Enabled {
		preTakeProfit = entryPrice * (1 + rules.TakeProfit.ThresholdPct/100)
		preTakeProfit = math.Round(preTakeProfit*100) / 100
	}
	return
}

// calcAllocation 计算等权分配买入金额
func calcAllocation(cash float64, maxPositions, currentHoldingCount int, maxSinglePct float64, totalEquity float64) float64 {
	availableCash := cash * 0.95
	maxSlots := maxPositions - currentHoldingCount
	if maxSlots <= 0 {
		return 0
	}
	perStockAmount := availableCash / float64(maxPositions)
	if maxSinglePct > 0 {
		maxAmountPerStock := totalEquity * maxSinglePct / 100
		if perStockAmount > maxAmountPerStock {
			perStockAmount = maxAmountPerStock
		}
	}
	return math.Round(perStockAmount*100) / 100
}