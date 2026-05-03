package handler

import (
	"stock-ai/internal/screener/indicator"
	"stock-ai/internal/screener/indicator/fundamental"
	"stock-ai/internal/screener/indicator/market"
	"stock-ai/internal/screener/indicator/technical"
	"stock-ai/internal/screener/indicator/financial"
)

// allBuiltins 聚合所有内置指标实例
// 放在 handler 层避免 indicator 子包循环依赖:
//
//	子包(financial等) → indicator(NumberFieldIndicator/BaseIndicator)
//	handler → 所有子包 (单向，无回路)
func allBuiltins() []indicator.Indicator {
	var result []indicator.Indicator
	result = append(result, technical.All()...)
	result = append(result, market.All()...)
	result = append(result, fundamental.All()...)
	result = append(result, financial.All()...)
	return result
}
