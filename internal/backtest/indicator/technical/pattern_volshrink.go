package technical

import (
	"fmt"

	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  SignalVolumeShrinkLimitDown — 放量上涨后连续跌停缩量
//  Pattern 内置信号，序号 03 → 完整 ID 01006003
//
//  klines[0]=最新（倒序约定与 pattern.go 一致）。
//
//  ★ 判定模型（A 日默认今日，可用 a_ago 前移）：
//
//	跌停段 [A-1, A]：
//	    D1 A 日跌幅 >= drop_min（默认 9.9%）
//	    D2 A-1 日跌幅 >= drop_min（默认 9.9%）
//	    D3 |V(A) - V(A-1)| / min(V(A), V(A-1)) <= vol_diff_max（默认 30%）
//	    v1 = (V(A) + V(A-1)) / 2
//
//	上涨段 [A-3, A-2]：
//	    U1 A-2 日涨幅 > rise_min（默认 0%）
//	    U2 A-3 日涨幅 > rise_min（默认 0%）
//	    U3 |V(A-2) - V(A-3)| / min(V(A-2), V(A-3)) <= vol_diff_max（默认 30%）
//	    v2 = (V(A-2) + V(A-3)) / 2
//
//	量能对比：v2 / v1 > vol_ratio_min（默认 7）
//
//  ★ 口径说明：
//    1. 涨跌幅统一用"收盘价相对前一日收盘价"（Close(i)/Close(i+1)-1），不用开盘价，与
//       涨停/跌停口径一致；代价是 A-3 的涨幅需要 A-4 收盘价，最少 5 根 K 线（含 A 日）。
//    2. 成交量"相差"以较小者为分母（|a-b|/min(a,b)）：130 vs 100 视为相差 30%，与口语
//       一致，且不受两日先后次序影响。段内成交量为 0 时直接拒绝（比值无意义）。
//    3. v2/v1 > 7 的语义：跌停段相对上涨段极度缩量，说明放量拉升后抛压在缩量跌停中
//       集中释放（跌停封死、接盘稀少），而非趋势性出货。
//    4. drop_min 取 9.9% 而非 10%：为复权与价格取整留出容差，判定用 >=（跌停即达标）。
// ============================================================================

// 放量上涨后连续跌停缩量的参数键
const (
	paramVSAgo         = "a_ago"         // A 日距今日的交易日数，0=今日
	paramVSDropMin     = "drop_min"      // 跌停段单日跌幅下限（%）
	paramVSVolDiffMax  = "vol_diff_max"  // 段内两日成交量相差上限（%）
	paramVSRiseMin     = "rise_min"      // 上涨段单日涨幅下限（%）
	paramVSVolRatioMin = "vol_ratio_min" // v2/v1 下限（倍）
)

// vsParams 放量上涨后连续跌停缩量的判定参数。
type vsParams struct {
	Ago         int     // A 日距今日的交易日数
	DropMin     float64 // 跌停段单日跌幅下限（%）
	VolDiffMax  float64 // 段内两日成交量相差上限（%）
	RiseMin     float64 // 上涨段单日涨幅下限（%）
	VolRatioMin float64 // v2/v1 下限（倍）
}

type SignalVolumeShrinkLimitDown struct {
	indicator.BaseSignal
}

// NewSignalVolumeShrinkLimitDown 创建"放量上涨后连续跌停缩量"内置信号。
func NewSignalVolumeShrinkLimitDown() *SignalVolumeShrinkLimitDown {
	return &SignalVolumeShrinkLimitDown{
		BaseSignal: indicator.NewBaseSignal(
			"03",
			"连续跌停缩量",
			"A、A-1 连续两日跌幅超阈值且量能相当（均量 v1），此前 A-2、A-3 两日上涨且量能相当（均量 v2），且 v2/v1 大于阈值",
			indicator.ValSeries,
			[]indicator.OperatorOption{{
				Operator: indicator.OpCustom,
				Label:    "参数设置",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(paramVSAgo, "A日距今", 0, "天前"),
					signalutil.ParamNumber(paramVSDropMin, "跌停段跌幅下限", 9.9, "%"),
					signalutil.ParamNumber(paramVSVolDiffMax, "段内量差上限", 30, "%"),
					signalutil.ParamNumber(paramVSRiseMin, "上涨段涨幅下限", 0, "%"),
					signalutil.ParamNumber(paramVSVolRatioMin, "量比下限 v2/v1", 7, "倍"),
				},
			}},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					paramVSAgo:         float64(0),
					paramVSDropMin:     float64(9.9),
					paramVSVolDiffMax:  float64(30),
					paramVSRiseMin:     float64(0),
					paramVSVolRatioMin: float64(7),
				},
			},
		),
	}
}

// parseVSParams 从信号配置解析判定参数，缺失项使用默认阈值，并对越界值做收敛。
func parseVSParams(config *indicator.SignalConfig) vsParams {
	p := vsParams{
		Ago:         int(config.GetFloat64(paramVSAgo, 0)),
		DropMin:     config.GetFloat64(paramVSDropMin, 9.9),
		VolDiffMax:  config.GetFloat64(paramVSVolDiffMax, 30),
		RiseMin:     config.GetFloat64(paramVSRiseMin, 0),
		VolRatioMin: config.GetFloat64(paramVSVolRatioMin, 7),
	}
	if p.Ago < 0 {
		p.Ago = 0
	}
	if p.DropMin < 0 {
		p.DropMin = 0
	}
	if p.VolDiffMax < 0 {
		p.VolDiffMax = 0
	}
	if p.VolRatioMin < 0 {
		p.VolRatioMin = 0
	}
	return p
}

// Evaluate 放量上涨后连续跌停缩量判定。
//
// klines 倒序（klines[0]=最新）。命中返回 ResultPassed 并在 Message 中给出 A 日、
// 两段均量与量比；未命中返回 ResultRejected 与首个不满足的条件说明。
func (s *SignalVolumeShrinkLimitDown) Evaluate(klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if config == nil {
		config = s.DefaultConfig()
	}
	p := parseVSParams(config)

	// A 日 + A-1 + A-2 + A-3 + A-4（A-3 涨幅的分母）
	need := p.Ago + 5
	if len(klines) < need {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根（A日距今%d天 + A-1~A-4），当前 %d 根", need, p.Ago, len(klines)),
		}
	}
	a, a1, a2, a3, a4 := klines[p.Ago], klines[p.Ago+1], klines[p.Ago+2], klines[p.Ago+3], klines[p.Ago+4]
	if a1.Close <= 0 || a2.Close <= 0 || a3.Close <= 0 || a4.Close <= 0 {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("A日 %d：A-1~A-4 存在收盘价 <= 0，无法计算涨跌幅", a.TradeDate),
		}
	}

	// D1/D2 跌停段跌幅
	dropA := vsChangePct(a.Close, a1.Close)
	if dropA > -p.DropMin {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("A日 (%d) 跌幅 %.2f%% < %.2f%%", a.TradeDate, -dropA, p.DropMin),
		}
	}
	dropA1 := vsChangePct(a1.Close, a2.Close)
	if dropA1 > -p.DropMin {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("A-1日 (%d) 跌幅 %.2f%% < %.2f%%", a1.TradeDate, -dropA1, p.DropMin),
		}
	}

	// D3 跌停段量差与均量 v1
	volDiffDown, ok := vsVolumeDiffPct(a.Volume, a1.Volume)
	if !ok {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("跌停段成交量为 0（A日 %d 量=%d，A-1日 %d 量=%d）", a.TradeDate, a.Volume, a1.TradeDate, a1.Volume),
		}
	}
	if volDiffDown > p.VolDiffMax {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message: fmt.Sprintf("跌停段量差 %.2f%% > %.2f%%（A日 %d 量=%d，A-1日 %d 量=%d）",
				volDiffDown, p.VolDiffMax, a.TradeDate, a.Volume, a1.TradeDate, a1.Volume),
		}
	}
	v1 := (float64(a.Volume) + float64(a1.Volume)) / 2

	// U1/U2 上涨段涨幅
	riseA2 := vsChangePct(a2.Close, a3.Close)
	if riseA2 <= p.RiseMin {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("A-2日 (%d) 涨幅 %.2f%% <= %.2f%%", a2.TradeDate, riseA2, p.RiseMin),
		}
	}
	riseA3 := vsChangePct(a3.Close, a4.Close)
	if riseA3 <= p.RiseMin {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("A-3日 (%d) 涨幅 %.2f%% <= %.2f%%", a3.TradeDate, riseA3, p.RiseMin),
		}
	}

	// U3 上涨段量差与均量 v2
	volDiffUp, ok := vsVolumeDiffPct(a2.Volume, a3.Volume)
	if !ok {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("上涨段成交量为 0（A-2日 %d 量=%d，A-3日 %d 量=%d）", a2.TradeDate, a2.Volume, a3.TradeDate, a3.Volume),
		}
	}
	if volDiffUp > p.VolDiffMax {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message: fmt.Sprintf("上涨段量差 %.2f%% > %.2f%%（A-2日 %d 量=%d，A-3日 %d 量=%d）",
				volDiffUp, p.VolDiffMax, a2.TradeDate, a2.Volume, a3.TradeDate, a3.Volume),
		}
	}
	v2 := (float64(a2.Volume) + float64(a3.Volume)) / 2

	// 量能对比
	if v1 <= 0 {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("跌停段均量 v1=%.0f，无法计算 v2/v1", v1),
		}
	}
	ratio := v2 / v1
	if ratio <= p.VolRatioMin {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message: fmt.Sprintf("量比 v2/v1=%.2f <= %.1f（v1=%.0f，v2=%.0f）",
				ratio, p.VolRatioMin, v1, v2),
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultPassed,
		SignalID: config.SignalID,
		Message: fmt.Sprintf("A日 %d（距今%d个交易日）：跌幅 %.2f%%、A-1 (%d) 跌幅 %.2f%%；"+
			"v1=%.0f（量差 %.2f%%）、v2=%.0f（量差 %.2f%%）、v2/v1=%.2f > %.1f",
			a.TradeDate, p.Ago, -dropA, a1.TradeDate, -dropA1,
			v1, volDiffDown, v2, volDiffUp, ratio, p.VolRatioMin),
	}
}

// vsChangePct 计算 cur 相对 prev 的涨跌幅百分比（跌为负）。
func vsChangePct(cur, prev int) float64 {
	return (float64(cur)/float64(prev) - 1) * 100
}

// vsVolumeDiffPct 计算两日成交量相差百分比，分母取较小者（130 vs 100 → 30%）。
//
// 任一方为 0 时返回 false（比值无意义）。
func vsVolumeDiffPct(a, b int64) (float64, bool) {
	if a <= 0 || b <= 0 {
		return 0, false
	}
	return float64(vsAbsDiff(a, b)) / float64(vsMin(a, b)) * 100, true
}

// vsAbsDiff 返回两数的差的绝对值。
func vsAbsDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

// vsMin 返回较小值。
func vsMin(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
