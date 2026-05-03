package fundamental

import (
	"stock-ai/internal/screener/indicator"
)

// All 返回基本面指标实例
func All() []indicator.Indicator {
	return []indicator.Indicator{
		NewListingBoard(),
		NewTotalShares(),
		NewFloatShares(),
	}
}
