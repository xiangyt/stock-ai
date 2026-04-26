package indicators

import (
	"fmt"
	"stock-ai/internal/model"
	"stock-ai/internal/screener/operators"
)

func init() {
	// MACD 交叉信号：金叉/死叉 + 底背离/顶背离
	RegisterOperators("macd_cross", operators.SeriesOps())
	// MACD 柱状图：红柱/绿柱的数值大小比较
	RegisterOperators("macd_hist", operators.NumberOps())
	// MACD 背离：独立指标，也支持序列型操作符
	RegisterOperators("macd_divergence", operators.SeriesOps())
}

// ============================================================================
//  MACD 指标评估器
//
//  覆盖信号:
//    - macd_golden_cross     MACD金叉 (OpCrossUp)
//    - macd_death_cross      MACD死叉 (OpCrossDown)
//    - macd_bull_divergence  MACD底背离 (OpDivergencePos)
//    - macd_hist_positive    MACD红柱   (OpGT) — 使用通用工厂，不在此实现
// ============================================================================

// EvaluateMACDCross MACD 金叉/死叉检测（★ 支持动态时间窗口）
//
// 参数说明（来自 Signal 的可变字段）：
//   - WithinDays: 时间窗口大小（天）。1=仅今天/昨天, 3=近3天内, 0=不限制窗口
//   - AgoFromDays: 起始偏移。0=从今天开始, 1=排除今天从昨天开始
//   - LookbackDays: 历史数据回看深度（用于确保有足够数据做交叉判断）
//
// 检测逻辑：
//   在指定的时间窗口 [agoFrom, agoFrom+within) 内逐日扫描，
//   判断每天是否发生了金叉(DIF上穿DEA)或死叉(DIF下穿DEA)。
func EvaluateMACDCross(stock *model.StockData, signal *model.Signal) *model.EvaluateResult {
	start := timeNow()
	result := &model.EvaluateResult{SignalID: signal.ID}

	hist := stock.Historical

	// 确定回看深度：至少需要 LookbackDays+1 天的数据
	lookback := signal.LookbackDays
	if lookback <= 0 {
		lookback = 5 // 默认回看5天
	}
	requiredLen := lookback + 1 // 当天 + 至少1天前（判断交叉需要连续两天）
	if len(hist) < requiredLen {
		result.Error = fmt.Errorf("历史数据不足(需>=%d天, 实际%d天)", requiredLen, len(hist))
		result.CostNanos = timeSince(start)
		return result
	}

	// 获取当前值
	difToday := stock.Current["macd"]
	deaToday := stock.Current["macd_signal"]

	// 获取时间窗口
	winStart, winEnd := signal.GetWithinRange()

	var pass bool
	crossDay := -1

	switch signal.Operator {
	case model.OpCrossUp:
		// ★ 金叉：在时间窗口内找任意一天出现 DIF 上穿 DEA
		if winEnd == 0 {
			pass = difToday >= deaToday
			crossDay = 0
			result.Detail = fmt.Sprintf("MACD金叉状态? DIF=%.4f DEA=%.4f → %v",
				difToday, deaToday, PassMark(pass))
			break
		}
		for offset := winStart; offset < winEnd; offset++ {
			if offset >= len(hist) {
				break
			}
			difCurr, deaCurr := getMACDValuesAt(stock, offset)
			difPrev, deaPrev := getMACDValuesAt(stock, offset+1)
			if difCurr >= deaCurr && difPrev <= deaPrev {
				pass = true
				crossDay = offset
				break
			}
		}
		if pass {
			dayLabel := DayOffsetLabel(crossDay)
			result.Detail = fmt.Sprintf("MACD%s金叉✓ DIF=%.4f≥DEA=%.4f | 前日DIF=%.4f≤DEA=%.4f",
				dayLabel, difToday, deaToday,
				getMACDValueAtHist(hist, min(crossDay+1, len(hist)-1), "macd"),
				getMACDValueAtHist(hist, min(crossDay+1, len(hist)-1), "macd_signal"))
		} else {
			result.Detail = fmt.Sprintf("MACD近%d日内未出现金叉 ✗ (窗口[%d,%d))",
				signal.WithinDays, winStart, winEnd)
		}

	case model.OpCrossDown:
		// ★ 死叉：与金叉对称
		if winEnd == 0 {
			pass = difToday <= deaToday
			crossDay = 0
			result.Detail = fmt.Sprintf("MACD死叉状态? DIF=%.4f DEA=%.4f → %v",
				difToday, deaToday, PassMark(pass))
			break
		}
		for offset := winStart; offset < winEnd; offset++ {
			if offset >= len(hist) {
				break
			}
			difCurr, deaCurr := getMACDValuesAt(stock, offset)
			difPrev, deaPrev := getMACDValuesAt(stock, offset+1)
			if difCurr <= deaCurr && difPrev >= deaPrev {
				pass = true
				crossDay = offset
				break
			}
		}
		if pass {
			dayLabel := DayOffsetLabel(crossDay)
			result.Detail = fmt.Sprintf("MACD%s死叉✓", dayLabel)
		} else {
			result.Detail = fmt.Sprintf("MACD近%d日内未出现死叉 ✗ (窗口[%d,%d))",
				signal.WithinDays, winStart, winEnd)
		}

	default:
		pass = false
		result.Detail = fmt.Sprintf("MACD交叉: 不支持的操作符 %s", signal.Operator)
	}

	result.Pass = pass
	result.CostNanos = timeSince(start)
	return result
}

// EvaluateMACDDivergence MACD 背离检测
func EvaluateMACDDivergence(stock *model.StockData, signal *model.Signal) *model.EvaluateResult {
	start := timeNow()
	result := &model.EvaluateResult{SignalID: signal.ID}

	hist := stock.Historical
	days := signal.LookbackDays
	if days <= 0 {
		days = 26
	}
	if len(hist) < days {
		result.Error = fmt.Errorf("历史数据不足(需>=%d天)", days)
		result.CostNanos = timeSince(start)
		return result
	}

	// 找出最近 N 天内的价格低点和对应的 MACD 低点
	window := hist[:min(days, len(hist))]
	lows := FindValleys(window, "close")
	macdLows := FindValleyIndicesFromHistorical(stock, days, "macd_hist")

	switch signal.Operator {
	case model.OpDivergencePos:
		pass := DetectBullDivergence(lows, macdLows)
		result.Pass = pass
		result.Detail = fmt.Sprintf("MACD底背离? 价格低点%d个 MACD低点%d个 → %v",
			len(lows), len(macdLows), PassMark(pass))
	case model.OpDivergenceNeg:
		pass := DetectBearDivergence(lows, macdLows)
		result.Pass = pass
		result.Detail = fmt.Sprintf("MACD顶背离? → %v", PassMark(pass))
	default:
		result.Pass = false
	}

	result.CostNanos = timeSince(start)
	return result
}

// ============================================================================
//  MACD 辅助函数（package 内共享）
// ============================================================================

// getMACDValuesAt 获取第 offset 天前（offset=0=今天, offset=1=昨天）的 DIF 和 DEA 值
func getMACDValuesAt(stock *model.StockData, offset int) (dif, dea float64) {
	if offset == 0 {
		return stock.Current["macd"], stock.Current["macd_signal"]
	}
	if offset <= len(stock.Historical) {
		dayData := stock.Historical[offset-1] // hist[0]=最新一天(昨天)
		if d, ok := dayData["macd"]; ok {
			dif = d
		}
		if d, ok := dayData["macd_signal"]; ok {
			dea = d
		}
	}
	return
}

// getMACDValueAtHist 从 historical 数据中获取某天的某个字段值
func getMACDValueAtHist(hist []map[string]float64, idx int, field string) float64 {
	if idx >= 0 && idx < len(hist) {
		if v, ok := hist[idx][field]; ok {
			return v
		}
	}
	return 0
}
