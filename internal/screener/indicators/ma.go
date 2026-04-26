package indicators

import (
	"fmt"
	"stock-ai/internal/model"
	"stock-ai/internal/screener/operators"
)

func init() {
	// 均线多头/空头排列：序列型（连续N天保持）
	RegisterOperators("ma_cross", operators.SeriesOps())
}

// ============================================================================
//  均线(MA)指标评估器
//
//  覆盖信号:
//    - ma_bullish_arrange   均线多头排列 MA5>MA10>MA20 (OpCrossUp)
//    - (空头排列 OpCrossDown 对称支持)
// ============================================================================

// EvaluateMACross 均线交叉/排列检测（★ 支持动态时间窗口）
//
// 多头排列: MA5 > MA10 > MA20（WithinDays=连续 N 天保持）
// 空头排列: MA5 < MA10 < MA20
func EvaluateMACross(stock *model.StockData, signal *model.Signal) *model.EvaluateResult {
	start := timeNow()
	result := &model.EvaluateResult{SignalID: signal.ID}

	_, winEnd := signal.GetWithinRange()

	checkDays := signal.WithinDays
	if checkDays <= 0 {
		checkDays = 1 // 默认至少检查1天
	}
	if winEnd > checkDays {
		checkDays = winEnd
	}

	ma5 := stock.Current["ma5"]
	ma10 := stock.Current["ma10"]
	ma20 := stock.Current["ma20"]

	var pass bool

	switch signal.Operator {
	case model.OpCrossUp:
		// 多头排列检查
		if checkDays <= 1 {
			pass = ma5 > ma10 && ma10 > ma20
			result.Detail = fmt.Sprintf("均线多头? MA5=%.2f>MA10=%.2f>MA20=%.2f → %v",
				ma5, ma10, ma20, PassMark(pass))
		} else {
			consecutiveDays := 0
			if ma5 > ma10 && ma10 > ma20 {
				consecutiveDays = 1
			}
			for i := 0; i < min(checkDays-1, len(stock.Historical)); i++ {
				dayData := stock.Historical[i]
				m5, m10, m20 := dayData["ma5"], dayData["ma10"], dayData["ma20"]
				if m5 > m10 && m10 > m20 {
					consecutiveDays++
				} else {
					break
				}
			}
			pass = consecutiveDays >= checkDays
			result.Detail = fmt.Sprintf("均线多头? 连续%d天/%d天 → %v",
				consecutiveDays, checkDays, PassMark(pass))
		}

	case model.OpCrossDown:
		// 空头排列检查（与多头对称）
		if checkDays <= 1 {
			pass = ma5 < ma10 && ma10 < ma20
			result.Detail = fmt.Sprintf("均线空头? MA5=%.2f<MA10=%.2f<MA20=%.2f → %v",
				ma5, ma10, ma20, PassMark(pass))
		} else {
			consecutiveDays := 0
			if ma5 < ma10 && ma10 < ma20 {
				consecutiveDays = 1
			}
			for i := 0; i < min(checkDays-1, len(stock.Historical)); i++ {
				dayData := stock.Historical[i]
				m5, m10, m20 := dayData["ma5"], dayData["ma10"], dayData["ma20"]
				if m5 < m10 && m10 < m20 {
					consecutiveDays++
				} else {
					break
				}
			}
			pass = consecutiveDays >= checkDays
			result.Detail = fmt.Sprintf("均线空头? 连续%d天/%d天 → %v",
				consecutiveDays, checkDays, PassMark(pass))
		}

	default:
		pass = false
		result.Detail = fmt.Sprintf("均线排列: 不支持的操作符 %s", signal.Operator)
	}

	result.Pass = pass
	result.CostNanos = timeSince(start)
	return result
}
