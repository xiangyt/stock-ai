package handler

import (
	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/financial"
	"stock-ai/internal/indicator/fundamental"
	"stock-ai/internal/indicator/market"
	"stock-ai/internal/indicator/technical"
)

// allBuiltins 聚合所有内置指标实例
// 放在 handler 层避免 indicator 子包循环依赖:
//
//	子包(financial等) → indicator(NumberFieldIndicator/BaseIndicator)
//	handler → 所有子包 (单向，无回路)
//
// AllBuiltins 是 allBuiltins 的导出版本，供 main.go 使用
func allBuiltins() []indicator.Indicator {
	return AllBuiltins()
}

// AllBuiltins 聚合所有内置指标实例（导出版本，供 main.go 使用）
func AllBuiltins() []indicator.Indicator {
	var result []indicator.Indicator
	result = append(result, technical.All()...)
	result = append(result, market.All()...)
	result = append(result, fundamental.All()...)
	result = append(result, financial.All()...)
	return result
}
