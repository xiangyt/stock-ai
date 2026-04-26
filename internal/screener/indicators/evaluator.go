// Package indicators 指标评估器实现包
//
// 本包按指标类别组织评估器实现，每个指标一个文件：
//   - macd.go    MACD 金叉/死叉/背离
//   - ma.go      均线多头/空头排列
//
// 设计原则：
//   - 简单数值比较用通用工厂(engine.go NumberEvaluatorFactory/FinancialEvaluatorFactory)
//   - 复杂序列扫描(交叉/背离/突破等)在此包内实现专用评估器
//   - 所有评估器通过 func(stock *model.StockData, signal *model.Signal) *model.EvaluateResult 签名注册到 Engine
//
// Evaluator 函数类型定义在 engine.go 中（避免循环依赖），本包的函数均遵循该签名。
package indicators

import (
	"fmt"
	"sort"
	"sync"

	"stock-ai/internal/model"
	"stock-ai/internal/screener/operators"
)

// ============================================================================
//  通用工具函数（所有指标子模块共享）
// ============================================================================

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// abs 返回浮点数的绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ============================================================================
//  时间测量（placeholder 实现，避免在热路径引入 time 包开销）
// ============================================================================

var (
	// timeNow 返回当前时间纳秒数（可被测试替换）
	timeNow = func() int64 { return 0 }
	// timeSince 计算耗时纳秒数
	timeSince = func(start int64) int64 { return 0 }
)

// ============================================================================
//  结果标记与格式化
// ============================================================================

// PassMark 通过/未通过的标记符号
func PassMark(pass bool) string {
	if pass {
		return "✓"
	}
	return "✗"
}

// FormatDetailWithOperator 生成含操作符的详情字符串（供各评估器复用）
func FormatDetailWithOperator(fieldName string, value float64, op model.CompareOperator,
	valNum, valLow, valHigh float64, unit string, pass bool) string {

	opSym := model.OperatorMeta[op].Symbol
	formattedVal := formatNumber(value, unit)

	switch op {
	case model.OpGT, model.OpGTE, model.OpLT, model.OpLTE, model.OpEQ, model.OpNEQ:
		return fmt.Sprintf("%s%s %s %s%s → %v",
			formattedVal, unit, opSym, formatNumber(valNum, unit), unit, PassMark(pass))
	case model.OpBetween, model.OpNotBetween:
		return fmt.Sprintf("%s%s %s [%s%s, %s%s] → %v",
			formattedVal, unit, opSym,
			formatNumber(valLow, unit), unit, formatNumber(valHigh, unit), unit, PassMark(pass))
	default:
		return fmt.Sprintf("%s → ? (未知操作符 %s)", formattedVal, op)
	}
}

// formatNumber 格式化数字显示
func formatNumber(v float64, unit string) string {
	switch unit {
	case "%":
		return fmt.Sprintf("%.2f", v)
	case "倍", "":
		return fmt.Sprintf("%.2f", v)
	case "亿":
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

// ============================================================================
//  通用序列算法
// ============================================================================

// FindValleys 在序列中找出局部低点（简化版）
func FindValleys(series []map[string]float64, field string) []int {
	var valleys []int
	for i := 1; i < len(series)-1; i++ {
		prev, _ := series[i-1][field]
		curr, _ := series[i][field]
		next, _ := series[i+1][field]
		if curr <= prev && curr <= next {
			valleys = append(valleys, i)
		}
	}
	return valleys
}

// FindValleyIndicesFromHistorical 从股票历史数据中找某字段的低点索引
func FindValleyIndicesFromHistorical(stock *model.StockData, lookback int, field string) []int {
	// 完整实现需要从 historical 中提取特定字段序列后调用 FindValleys
	// 当前为简化实现，后续可扩展
	return []int{}
}

// DetectBullDivergence 检测底背离（价格新低但指标不新低）
func DetectBullDivergence(priceLows, indicatorLows []int) bool {
	// 完整实现需比较最近两个低点的变化方向
	// 简化版：如果存在价格低点多于指标低点，认为有背离迹象
	return len(priceLows) > 0 && len(indicatorLows) == 0
}

// DetectBearDivergence 检测顶背离（指标新高价但价格不新高）
func DetectBearDivergence(priceHighs, indicatorHighs []int) bool {
	return len(indicatorHighs) > 0 && len(priceHighs) == 0
}

// DayOffsetLabel 将天数偏移转换为可读中文标签
func DayOffsetLabel(offset int) string {
	switch offset {
	case 0:
		return "今日"
	case 1:
		return "昨日"
	case 2:
		return "前天"
	default:
		return fmt.Sprintf("%d天前", offset)
	}
}

// ============================================================================
//  指标操作符注册表 — 每个指标文件在 init() 中注册自己支持的操作符
//
//  设计理念：
//    "具体哪个指标可以选择哪些操作符由 macd.go 去定义"
//    而不是根据 ValueType 自动推导，让每个指标有完全的控制权。
//
//  用法（在各指标文件的 init() 中）：
//
//	func init() {
//	    RegisterOperators("macd_cross", operators.SeriesOps())
//	    RegisterOperators("pe_ttm", operators.NumberOps())
//	}
// ============================================================================

var (
	opRegistry     = map[string][]operators.OperatorOption{}
	opRegistryMu   sync.RWMutex
	opRegistryOnce sync.Once // 用于检测重复注册
)

// RegisterOperators 注册一个指标支持的操作符列表
// 应在每个指标文件的 init() 中调用。重复注册会 panic（防止冲突）
func RegisterOperators(indicatorID string, ops []operators.OperatorOption) {
	opRegistryMu.Lock()
	defer opRegistryMu.Unlock()

	if _, exists := opRegistry[indicatorID]; exists {
		panic(fmt.Sprintf("indicators: 重复注册指标 %q 的操作符", indicatorID))
	}
	opRegistry[indicatorID] = ops
}

// GetRegisteredOperators 查询某指标已注册的操作符
func GetRegisteredOperators(indicatorID string) ([]operators.OperatorOption, bool) {
	opRegistryMu.RLock()
	defer opRegistryMu.RUnlock()
	ops, ok := opRegistry[indicatorID]
	return ops, ok
}

// AllRegisteredOperators 返回所有已注册的 {indicatorID → operators} 映射
// 返回的 map 按 indicatorID 排序
func AllRegisteredOperators() map[string][]operators.OperatorOption {
	opRegistryMu.RLock()
	defer opRegistryMu.RUnlock()

	// 返回副本，避免外部修改
	result := make(map[string][]operators.OperatorOption, len(opRegistry))
	for id, ops := range opRegistry {
		result[id] = ops
	}
	return result
}

// RegisteredIndicatorIDs 返回所有已注册了操作符的指标 ID 列表（排序后）
func RegisteredIndicatorIDs() []string {
	opRegistryMu.RLock()
	defer opRegistryMu.RUnlock()

	ids := make([]string, 0, len(opRegistry))
	for id := range opRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
