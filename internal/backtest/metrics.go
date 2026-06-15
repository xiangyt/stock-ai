package backtest

import (
	"math"
)

// ============================================================================
//  绩效指标计算
// ============================================================================

// CalcMetrics 从每日快照和交易记录计算绩效指标
func CalcMetrics(snapshots []DailySnapshot, trades []BacktestTrade, initialCapital float64, tradingDays int) MetricsResult {
	if len(snapshots) == 0 {
		return MetricsResult{}
	}
	// 累计收益
	finalEquity := snapshots[len(snapshots)-1].TotalEquity
	totalReturn := (finalEquity - initialCapital) / initialCapital * 100

	// 年化收益
	annualReturn := (math.Pow(finalEquity/initialCapital, 252.0/float64(tradingDays)) - 1) * 100

	// 最大回撤
	maxDrawdown := calcMaxDrawdown(snapshots)

	// 夏普比率
	sharpeRatio := calcSharpeRatio(snapshots, tradingDays)

	// 胜率和盈亏比
	winRate, profitFactor := calcWinRate(trades)

	return MetricsResult{
		TotalReturn:  totalReturn,
		AnnualReturn: annualReturn,
		MaxDrawdown:  maxDrawdown,
		SharpeRatio:  sharpeRatio,
		WinRate:      winRate,
		ProfitFactor: profitFactor,
		TradeCount:   len(trades),
	}
}

// MetricsResult 绩效指标结果
type MetricsResult struct {
	TotalReturn  float64
	AnnualReturn float64
	MaxDrawdown  float64
	SharpeRatio  float64
	WinRate      float64
	ProfitFactor float64
	TradeCount   int
}

// calcMaxDrawdown 计算最大回撤
func calcMaxDrawdown(snapshots []DailySnapshot) float64 {
	if len(snapshots) == 0 {
		return 0
	}
	peak := snapshots[0].TotalEquity
	maxDD := 0.0
	for _, s := range snapshots {
		if s.TotalEquity > peak {
			peak = s.TotalEquity
		}
		dd := (s.TotalEquity - peak) / peak * 100
		if dd < maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// calcSharpeRatio 计算夏普比率
func calcSharpeRatio(snapshots []DailySnapshot, tradingDays int) float64 {
	if len(snapshots) < 2 || tradingDays == 0 {
		return 0
	}
	var sum float64
	returns := make([]float64, 0, len(snapshots)-1)
	for i := 1; i < len(snapshots); i++ {
		if snapshots[i-1].TotalEquity > 0 {
			r := (snapshots[i].TotalEquity - snapshots[i-1].TotalEquity) / snapshots[i-1].TotalEquity
			sum += r
			returns = append(returns, r)
		}
	}
	if len(returns) == 0 {
		return 0
	}
	mean := sum / float64(len(returns))
	var varianceSum float64
	for _, r := range returns {
		varianceSum += (r - mean) * (r - mean)
	}
	stdDev := math.Sqrt(varianceSum / float64(len(returns)))
	if stdDev == 0 {
		return 0
	}
	rfDaily := 0.02 / 252
	return (mean - rfDaily) / stdDev * math.Sqrt(252)
}

// calcWinRate 计算胜率和盈亏比
func calcWinRate(trades []BacktestTrade) (float64, float64) {
	sellTrades := 0
	winTrades := 0
	totalProfit := 0.0
	totalLoss := 0.0
	for _, t := range trades {
		if t.TradeType == 2 && t.ProfitLoss != nil {
			sellTrades++
			if *t.ProfitLoss > 0 {
				winTrades++
				totalProfit += *t.ProfitLoss
			} else {
				totalLoss += math.Abs(*t.ProfitLoss)
			}
		}
	}
	if sellTrades == 0 {
		return 0, 0
	}
	winRate := float64(winTrades) / float64(sellTrades) * 100
	profitFactor := 0.0
	if totalLoss > 0 {
		profitFactor = totalProfit / totalLoss
	}
	return winRate, profitFactor
}