package fundamental

import (
	"stock-ai/internal/backtest/indicator"
)

// All 返回基本面指标实例
func All() []indicator.Indicator {
	return []indicator.Indicator{
		NewListingBoard(),
		NewTotalShares(),
		NewFloatShares(),
		NewTotalMarketCap(),
		NewCirculateMarketCap(),
		NewFloatRatio(),
		NewFreeHoldRatio(),
		NewHoldRatio(),
		NewHolderNum(),
		NewAvgHoldShares(),
		NewIsSt(),
	}
}
