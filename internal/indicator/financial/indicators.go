package financial

import (
	"stock-ai/internal/indicator"
)

// All 返回财务面指标实例
func All() []indicator.Indicator {
	return []indicator.Indicator{
		NewPETTM(),
		NewPB(),
		NewPSTTM(),
		NewROE(),
		NewROA(),
		NewGrossMargin(),
		NewNetMargin(),
		NewBVPS(),
		NewBasicEPS(),
		NewDebtRatio(),
		NewParentNetProfit(),
		NewDeductNetProfit(),
		NewTotalRevenue(),
	}
}
