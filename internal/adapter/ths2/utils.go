package ths2

import (
	"strings"

	"stock-ai/internal/adapter"
)

// codeToMarket 根据股票代码返回市场编码
// "17" = 上海, "33" = 深圳
func codeToMarket(code string) string {
	switch {
	case strings.HasPrefix(code, "60"), strings.HasPrefix(code, "68"):
		return "17" // 上海
	case strings.HasPrefix(code, "00"), strings.HasPrefix(code, "30"):
		return "33" // 深圳
	default:
		return ""
	}
}

// adjTypeToAdjustType 将内部复权类型映射为 API 参数
// adapter.AdjQFQ("1") → "forward"（前复权）
// adapter.AdjNone("0") → "forward"（quota-h API 不支持不复权，降级为前复权）
// adapter.AdjBQQ("2") → "backward"（后复权）
func adjTypeToAdjustType(adjType string) string {
	switch adjType {
	case adapter.AdjQFQ:
		return "forward"
	case adapter.AdjBQQ:
		return "backward"
	default:
		// quota-h API adjust_type 仅支持 "forward"/"backward"，
		// "none"、"" 等均返回空数据，故降级为前复权
		return "forward"
	}
}
