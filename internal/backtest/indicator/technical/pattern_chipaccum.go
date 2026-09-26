package technical

import (
	"fmt"
	"math"
	"sort"

	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  SignalChipAccumulation — 筹码收集后放量启动
//  Pattern 内置信号，序号 02 → 完整 ID 01006002
//
//  klines[0]=最新，klines[末尾]=最早（倒序约定与 pattern.go 一致）。
//
//  ★ 判定模型（两段式，A 日为分界点）：
//
//	收集期 [A-collect_days, A-1]（默认 20 个交易日）：
//	  成交量序列去掉 trim_count 个最大值与 trim_count 个最小值（默认各 3 个），
//	  剩余样本按时间顺序最小二乘拟合 y = a·x + b（x = 原始时间序号 0..N-1）：
//	    S1 接近水平：|a| / 平台均量 × 100 <= max_flat_slope（默认 1.5 %/日）
//	    S2 离散度低：残差 RMSE / 平台均量 <= max_dispersion（默认 0.30）
//	  → 视为"收集筹码"：量能平稳、无趋势、无异常脉冲
//
//	A 日触发：
//	    V(A) >= break_vol_mult × 平台均量（默认 2 倍）
//	    Close(A)/Close(A-1) - 1 >= break_rise（默认 5%）
//
//	放量段 [A, A+up_days-1]（默认 7 个交易日，含 A 日）：
//	  每日 MA10(V) = 当日 + 前 9 日成交量均值（前缀和 O(1) 取用），得 up_days 个点拟合：
//	    S3 量能持续抬升：斜率 >= min_slope_ratio × 平台均量（默认 0.5）
//	    S4 绝对放量：max(V) >= peak_mult × 平台均量（默认 7 倍）
//	    S5 价格确认：Close(A+up_days-1)/Close(A-1) - 1 >= confirm_rise（默认 15%）
//	    强势直通：S5 的涨幅 >= fast_rise（默认 50%）时直接命中，跳过 S1~S4 与收集期校验
//
//  ★ 口径说明：
//    1. 收集期用"去极值拟合"代替 MA5(V)/MA15(V) 之类的比值：单日脉冲（偶发放量、停牌、
//       长假）会被 trim 掉，不会污染平台量级判定。
//    2. S1~S4 的尺度基准统一为"平台均量" = 去极值后剩余样本（默认 20-2×3=14 个交易日）
//       的成交量均值，不用拟合截距 b：b 是外推值，会随斜率与 x 原点选取漂移（x=0 在收
//       集期首日时 b = 均值 - a·(N-1)/2），而均值是窗口量能水平的稳健估计、恒为正。
//    3. S3 过滤"一日游"：只有 A 日单独放天量时 MA10 在随后 7 天几乎不抬升（斜率≈0），
//       必须连续放量才能让 MA10 斜率达标。
//    4. 收集期水平度用的是"相对斜率"（%/日 = 斜率/均值×100），跨股票可比。
//    5. 强势直通（fast_rise）：A+up_days-1 相对 A-1 涨幅达到阈值时直接命中，不再校验
//       收集期平整度/离散度与放量段量能。理由：连续涨停/主升行情的"收集期"通常并不安静
//       （震荡大、量能不平），这些条件恰恰会误杀最强的票；直通发生在 P1/P2（单日大涨且
//       A 日量能已显著高于平台）已通过之后，风险可控。置 0 可关闭直通、回到完整校验。
//    6. A 日放量以"平台均量"为分母，不用 V(A-1)：单日量本身噪声大（A-1 可能是地量或天
//       量），且连续涨停首日的放大倍数往往不高，用单日量做分母会误杀真正的起涨点。
//
//  ★ 判定顺序按"成本从低到高、过滤力从强到弱"排列（P1~P7），绝大多数候选在 O(1)
//    判断阶段即被淘汰，排序与拟合只在极少数候选上执行。
// ============================================================================

// 筹码收集后放量启动的参数键
const (
	paramChipMinAgo       = "a_min_ago"       // A 日距今日最小交易日数
	paramChipCollectDays  = "collect_days"    // 收集期天数
	paramChipTrim         = "trim_count"      // 收集期去掉的最大/最小样本个数
	paramChipMaxFlatSlope = "max_flat_slope"  // 收集期相对斜率上限（%/日）
	paramChipMaxDisp      = "max_dispersion"  // 收集期相对离散度上限（RMSE/均值）
	paramChipUpDays       = "up_days"         // 放量段天数（含 A 日）
	paramChipBreakVolMult = "break_vol_mult"  // A 日成交量 / 平台均量 下限
	paramChipBreakRise    = "break_rise"      // A 日涨幅下限（%，相对 A-1 收盘）
	paramChipFastRise     = "fast_rise"       // 强势直通涨幅下限（%，末端相对 A-1；达标直接命中，0=不启用）
	paramChipMinSlope     = "min_slope_ratio" // 放量段 MA10 斜率下限（× 平台均量）
	paramChipPeakMult     = "peak_mult"       // 放量段峰值 / 平台均量 下限
	paramChipConfirmRise  = "confirm_rise"    // 放量段末端相对 A-1 收盘的最小涨幅（%，0=不启用）
)

// 参数边界
const (
	chipMinCollectDays = 10 // 收集期最小天数
	chipMinKeep        = 3  // 去极值后至少保留的样本数
	chipMinUpDays      = 3  // 放量段最小天数（拟合斜率至少 3 点）
	chipStageHit       = 7  // 全部检查项通过时的 stage
)

type SignalChipAccumulation struct {
	indicator.BaseSignal
}

// NewSignalChipAccumulation 创建"筹码收集后放量启动"内置信号。
func NewSignalChipAccumulation() *SignalChipAccumulation {
	return &SignalChipAccumulation{
		BaseSignal: indicator.NewBaseSignal(
			"02",
			"筹码收集后放量启动",
			"近N个交易日内存在一天A：A之前N日成交量去极值后拟合近水平且离散度低（收集筹码），A日放量上涨，A起N日成交量MA10持续抬升、峰值达平台量级N倍且价格显著上行",
			indicator.ValSeries,
			[]indicator.OperatorOption{{
				Operator: indicator.OpCustom,
				Label:    "参数设置",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyDays, "启动日距今上限", 40, "天"),
					signalutil.ParamNumber(paramChipMinAgo, "启动日距今下限", 7, "天"),
					signalutil.ParamNumber(paramChipCollectDays, "收集期天数", 20, "天"),
					signalutil.ParamNumber(paramChipTrim, "去极值个数", 3, "个"),
					signalutil.ParamNumber(paramChipMaxFlatSlope, "收集期斜率上限", 1.5, "%/日"),
					signalutil.ParamNumber(paramChipMaxDisp, "收集期离散度上限", 0.30, "×平台均量"),
					signalutil.ParamNumber(paramChipUpDays, "放量段天数", 7, "天"),
					signalutil.ParamNumber(paramChipBreakVolMult, "A日放量倍数", 2.0, "×平台均量"),
					signalutil.ParamNumber(paramChipBreakRise, "A日涨幅下限", 5, "%"),
					signalutil.ParamNumber(paramChipFastRise, "强势直通涨幅", 50, "%"),
					signalutil.ParamNumber(paramChipMinSlope, "MA10斜率下限", 0.5, "×平台均量"),
					signalutil.ParamNumber(paramChipPeakMult, "峰值倍数下限", 7, "×平台均量"),
					signalutil.ParamNumber(paramChipConfirmRise, "末端涨幅下限", 15, "%"),
				},
			}},
			&indicator.SignalConfig{
				Operator: indicator.OpCustom,
				Params: map[string]any{
					indicator.ParamKeyDays: float64(40),
					paramChipMinAgo:        float64(7),
					paramChipCollectDays:   float64(20),
					paramChipTrim:          float64(3),
					paramChipMaxFlatSlope:  float64(1.5),
					paramChipMaxDisp:       float64(0.30),
					paramChipUpDays:        float64(7),
					paramChipBreakVolMult:  float64(2.0),
					paramChipBreakRise:     float64(5),
					paramChipFastRise:      float64(50),
					paramChipMinSlope:      float64(0.5),
					paramChipPeakMult:      float64(7),
					paramChipConfirmRise:   float64(15),
				},
			},
		),
	}
}

// chipParams 筹码收集后放量启动的判定参数。
type chipParams struct {
	Lookback      int     // A 日距今日上限（交易日）
	MinAgo        int     // A 日距今日下限（交易日）
	CollectDays   int     // 收集期天数 [A-N, A-1]
	TrimCount     int     // 去极值的最大/最小个数
	MaxFlatSlope  float64 // 收集期相对斜率上限（%/日）
	MaxDisp       float64 // 收集期相对离散度上限（残差 RMSE / 平台均量）
	UpDays        int     // 放量段天数（含 A 日）
	BreakVolMult  float64 // A 日成交量 / 平台均量 下限
	BreakRise     float64 // A 日涨幅下限（%，相对 A-1 收盘）
	FastRise      float64 // 强势直通涨幅下限（%，放量段末端相对 A-1；达标即命中，0=不启用）
	MinSlopeRatio float64 // 放量段 MA10 斜率下限（× 平台均量）
	PeakMult      float64 // 放量段峰值 / 平台均量 下限
	ConfirmRise   float64 // 放量段末端相对 A-1 收盘的最小涨幅（%，0=不启用）
}

// parseChipParams 从信号配置解析判定参数，缺失项使用默认阈值，并对越界值做收敛。
func parseChipParams(config *indicator.SignalConfig) chipParams {
	p := chipParams{
		Lookback:      int(config.GetFloat64(indicator.ParamKeyDays, 40)),
		MinAgo:        int(config.GetFloat64(paramChipMinAgo, 7)),
		CollectDays:   int(config.GetFloat64(paramChipCollectDays, 20)),
		TrimCount:     int(config.GetFloat64(paramChipTrim, 3)),
		MaxFlatSlope:  config.GetFloat64(paramChipMaxFlatSlope, 1.5),
		MaxDisp:       config.GetFloat64(paramChipMaxDisp, 0.30),
		UpDays:        int(config.GetFloat64(paramChipUpDays, 7)),
		BreakVolMult:  config.GetFloat64(paramChipBreakVolMult, 2.0),
		BreakRise:     config.GetFloat64(paramChipBreakRise, 5),
		FastRise:      config.GetFloat64(paramChipFastRise, 50),
		MinSlopeRatio: config.GetFloat64(paramChipMinSlope, 0.5),
		PeakMult:      config.GetFloat64(paramChipPeakMult, 7),
		ConfirmRise:   config.GetFloat64(paramChipConfirmRise, 15),
	}
	if p.Lookback < 1 {
		p.Lookback = 1
	}
	if p.CollectDays < chipMinCollectDays {
		p.CollectDays = chipMinCollectDays
	}
	if maxKeep := (p.CollectDays - chipMinKeep) / 2; p.TrimCount > maxKeep { // 去极值后至少保留 chipMinKeep 个
		p.TrimCount = maxKeep
	}
	if p.TrimCount < 0 {
		p.TrimCount = 0
	}
	if p.UpDays < chipMinUpDays {
		p.UpDays = chipMinUpDays
	}
	if p.Lookback < p.CollectDays+1 { // 回看窗口至少要装得下收集期
		p.Lookback = p.CollectDays + 1
	}
	if p.BreakVolMult < 0 {
		p.BreakVolMult = 0
	}
	if p.MinSlopeRatio < 0 {
		p.MinSlopeRatio = 0
	}
	return p
}

// Evaluate 筹码收集后放量启动判定。
//
// klines 倒序（klines[0]=最新）。命中返回 ResultPassed 并在 Message 中给出 A 日与两段拟合指标；
// 未命中返回 ResultRejected 与窗口内"最接近命中"候选的失败原因。
func (s *SignalChipAccumulation) Evaluate(klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if config == nil {
		config = s.DefaultConfig()
	}
	p := parseChipParams(config)

	minLen := p.CollectDays + p.UpDays + 1
	if len(klines) < minLen {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message: fmt.Sprintf("K线数据不足，需要至少 %d 根（收集期%d+放量段%d+1），当前 %d 根",
				minLen, p.CollectDays, p.UpDays, len(klines)),
		}
	}

	idx, reason := findChipAccum(klines, p)
	if idx < 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: reason}
	}

	prefix := chipVolumePrefix(klines)
	ma10, _ := chipUpMASeriesPrefix(prefix, idx, p.UpDays)
	slope2, _ := linearFitFloat(chipIndexSeries(len(ma10)), ma10)
	collect, _ := chipCollectVolumes(klines, idx, p.CollectDays)
	_, ky := chipTrimExtremes(collect, p.TrimCount)
	base := chipMean(ky) // 平台均量（去极值后剩余样本均值）

	peak := float64(0)
	for t := 0; t < p.UpDays; t++ {
		if v := float64(klines[idx-t].Volume); v > peak {
			peak = v
		}
	}
	prev, end := klines[idx+1], klines[idx-p.UpDays+1]
	maFirst, maLast := float64(0), float64(0)
	if len(ma10) > 0 {
		maFirst, maLast = ma10[0], ma10[len(ma10)-1]
	}
	endRise := (float64(end.Close)/float64(prev.Close) - 1) * 100
	fastNote := ""
	if p.FastRise > 0 && endRise >= p.FastRise {
		fastNote = fmt.Sprintf("；强势直通（末端涨幅 %.2f%% >= %.0f%%）：未校验收集期平整度与量能持续性",
			endRise, p.FastRise)
	}
	return &indicator.EvaluatedStock{
		Result:   indicator.ResultPassed,
		SignalID: config.SignalID,
		Message: fmt.Sprintf("A日 %d（距今%d个交易日）：收集期 %d~%d 平台均量=%.0f；"+
			"A日量 %.1f×平台均量、涨幅 %.2f%%；放量段 %d~%d MA10(V) %.0f→%.0f（斜率 %.0f/日 = %.2f×平台均量）、"+
			"峰值 %.1f×平台均量、末端涨幅 %.2f%%%s",
			klines[idx].TradeDate, idx,
			klines[idx+p.CollectDays].TradeDate, prev.TradeDate, base,
			float64(klines[idx].Volume)/chipMax(base, 1),
			(float64(klines[idx].Close)/float64(prev.Close)-1)*100,
			klines[idx].TradeDate, end.TradeDate, maFirst, maLast, slope2, slope2/chipMax(base, 1),
			peak/chipMax(base, 1), endRise, fastNote),
	}
}

// findChipAccum 在 A 距今 [MinAgo, Lookback] 个交易日的窗口内搜索 A，返回其索引（klines 倒序）。
//
// 扫描方向：从窗口最早端开始沿时间轴向今日推进（idx 由大到小），命中即返回。
// 未命中返回 -1，reason 取窗口内通过检查项最多的候选的失败原因。
func findChipAccum(klines []*model.DailyKline, p chipParams) (idx int, reason string) {
	maxIdx := p.Lookback - 1
	if limit := len(klines) - p.CollectDays - 1; maxIdx > limit {
		maxIdx = limit
	}
	// A 之后必须有 UpDays-1 天放量段，且 A 距今不少于 MinAgo 天
	minIdx := max(p.MinAgo, p.UpDays-1)
	reason = fmt.Sprintf("近%d个交易日内未出现符合条件的筹码收集后放量启动"+
		"（启动日距今%d~%d个交易日、收集期%d日、放量段%d日）",
		p.Lookback, p.MinAgo, p.Lookback, p.CollectDays, p.UpDays)

	prefix := chipVolumePrefix(klines) // 一次 O(n)，全窗口复用
	bestStage := -1
	for i := maxIdx; i >= minIdx; i-- {
		ok, stage, msg := checkChipAccumAt(klines, prefix, i, p)
		if ok {
			return i, ""
		}
		if stage >= bestStage { // 由远及近扫描，同分时保留更接近今日的候选
			bestStage, reason = stage, msg
		}
	}
	return -1, reason
}

// checkChipAccumAt 判定 klines[idx] 是否为"筹码收集后放量启动"的 A 日。
//
// 返回是否命中、已通过的检查项数（stage，越大越接近命中）、失败原因。
// 检查按成本递增排列（见 P1~P7 注释），以便尽早淘汰候选。
func checkChipAccumAt(klines []*model.DailyKline, prefix []float64, idx int, p chipParams) (bool, int, string) {
	stage := 0
	aDay, prev := klines[idx], klines[idx+1]

	// P1 A 日涨幅（O(1)，最强过滤）
	if prev.Close <= 0 {
		return false, stage, fmt.Sprintf("距今日%d天：A-1 收盘价异常", idx)
	}
	breakRise := (float64(aDay.Close)/float64(prev.Close) - 1) * 100
	if breakRise < p.BreakRise {
		return false, stage, fmt.Sprintf("距今日%d天 (%d)：A日涨幅 %.2f%% < %.2f%%",
			idx, aDay.TradeDate, breakRise, p.BreakRise)
	}
	stage++

	// P2 收集期去极值 → 平台均量，并据此判定 A 日放量
	//
	// 基准用"平台均量"而非 A-1 单日量：A-1 本身可能是地量或天量，比值噪声大；而连续涨停
	// 行情的首日往往只是温和放大（相对 A-1 不到 2.5 倍），却已显著高于平台量级，用 A-1 做
	// 分母会把这类真正的起涨点误杀。
	collect, ok := chipCollectVolumes(klines, idx, p.CollectDays)
	if !ok {
		return false, stage, fmt.Sprintf("距今日%d天：收集期数据不足", idx)
	}
	keptX, keptY := chipTrimExtremes(collect, p.TrimCount)
	if len(keptY) < chipMinKeep {
		return false, stage, fmt.Sprintf("距今日%d天：去极值后样本不足（%d个）", idx, len(keptY))
	}
	mean := chipMean(keptY) // 平台均量：去极值后剩余样本（默认 14 日）均值，S1~S4 统一以此为尺度
	if mean <= 0 {
		return false, stage, fmt.Sprintf("距今日%d天：收集期成交量为 0", idx)
	}
	if float64(aDay.Volume) < mean*p.BreakVolMult {
		return false, stage, fmt.Sprintf("距今日%d天 (%d)：A日量 %d < 平台均量 %.0f×%.2f=%.0f",
			idx, aDay.TradeDate, aDay.Volume, mean, p.BreakVolMult, mean*p.BreakVolMult)
	}
	stage++

	// P3 放量段末端涨幅（O(1)，复用 A-1 收盘）
	endIdx := idx - p.UpDays + 1
	if endIdx < 0 {
		return false, stage, fmt.Sprintf("距今日%d天：放量段数据不足", idx)
	}
	rise := (float64(klines[endIdx].Close)/float64(prev.Close) - 1) * 100
	// 强势直通：A+up_days-1 相对 A-1 涨幅达到 fast_rise 时，认为趋势已自证，直接命中，
	// 跳过收集期平整度/离散度与放量段量能校验——强趋势票的收集期往往并不"安静"，
	// 这些条件在真正的连续涨停/主升行情下反而会误杀。
	if p.FastRise > 0 && rise >= p.FastRise {
		return true, chipStageHit, ""
	}
	if p.ConfirmRise > 0 {
		if rise < p.ConfirmRise {
			return false, stage, fmt.Sprintf("距今日%d天 (%d)：A+%d日收盘 %d 相对 A-1 收盘 %d 涨幅 %.2f%% < %.2f%%",
				idx, aDay.TradeDate, p.UpDays-1, klines[endIdx].Close, prev.Close, rise, p.ConfirmRise)
		}
		stage++
	}

	// P4/P5 收集期拟合（keptX/keptY/平台均量已在 P2 算好，此处只补拟合）
	slope, intercept := linearFitFloat(keptX, keptY)
	flatPct := math.Abs(slope) / mean * 100
	if flatPct > p.MaxFlatSlope {
		return false, stage, fmt.Sprintf("距今日%d天 (%d)：收集期斜率 %.2f%%/日 > %.2f%%/日（非水平）",
			idx, aDay.TradeDate, flatPct, p.MaxFlatSlope)
	}
	stage++
	disp := chipDispersion(keptX, keptY, slope, intercept) / mean
	if disp > p.MaxDisp {
		return false, stage, fmt.Sprintf("距今日%d天 (%d)：收集期离散度 %.2f > %.2f",
			idx, aDay.TradeDate, disp, p.MaxDisp)
	}
	stage++

	// P6 绝对放量：峰值 >= peak_mult × 平台均量（O(UpDays)，比 MA10 便宜）
	peak := float64(0)
	for t := 0; t < p.UpDays; t++ {
		if v := float64(klines[idx-t].Volume); v > peak {
			peak = v
		}
	}
	if peak < mean*p.PeakMult {
		return false, stage, fmt.Sprintf("距今日%d天 (%d)：放量段峰值 %.0f < 平台均量*%.1f=%.0f（平台均量=%.0f）",
			idx, aDay.TradeDate, peak, p.PeakMult, mean*p.PeakMult, mean)
	}
	stage++

	// P7 量能持续抬升：MA10 斜率 >= min_slope_ratio × 平台均量（最贵，放最后）
	ma10, ok := chipUpMASeriesPrefix(prefix, idx, p.UpDays)
	if !ok {
		return false, stage, fmt.Sprintf("距今日%d天：放量段 MA10 数据不足", idx)
	}
	if chipMean(ma10) <= 0 {
		return false, stage, fmt.Sprintf("距今日%d天：放量段成交量为 0", idx)
	}
	upSlope, _ := linearFitFloat(chipIndexSeries(len(ma10)), ma10)
	need := p.MinSlopeRatio * mean
	if upSlope < need {
		return false, stage, fmt.Sprintf("距今日%d天 (%d)：放量段MA10斜率 %.0f/日 < %.2f×平台均量=%.0f（平台均量=%.0f）",
			idx, aDay.TradeDate, upSlope, p.MinSlopeRatio, need, mean)
	}
	return true, chipStageHit, ""
}

// chipCollectVolumes 收集期 [A-days, A-1] 成交量，按时间升序（[0]=最早）。
func chipCollectVolumes(klines []*model.DailyKline, idx, days int) ([]float64, bool) {
	if days <= 0 || idx+days >= len(klines) {
		return nil, false
	}
	out := make([]float64, days)
	for j := 0; j < days; j++ {
		out[j] = float64(klines[idx+days-j].Volume) // j=0 → A-days（最早）
	}
	return out, true
}

// chipVolumePrefix 成交量前缀和（倒序下标）：prefix[i] = sum(Volume[0..i-1])。
func chipVolumePrefix(klines []*model.DailyKline) []float64 {
	prefix := make([]float64, len(klines)+1)
	for i, k := range klines {
		prefix[i+1] = prefix[i] + float64(k.Volume)
	}
	return prefix
}

// chipUpMASeriesPrefix 放量段 [A, A+days-1] 每日 MA10(V)，按时间升序（[0]=A 日），O(days)。
func chipUpMASeriesPrefix(prefix []float64, idx, days int) ([]float64, bool) {
	if idx < 0 || idx-days+1 < 0 || idx+10 > len(prefix)-1 {
		return nil, false
	}
	out := make([]float64, days)
	for t := 0; t < days; t++ {
		out[t] = (prefix[idx-t+10] - prefix[idx-t]) / 10 // 当日 + 前 9 日
	}
	return out, true
}

// chipTrimExtremes 去掉 n 个最大值与 n 个最小值，返回保留样本的 x（原始时间序号）与 y。
func chipTrimExtremes(ys []float64, n int) (xs, kept []float64) {
	if n <= 0 {
		return chipIndexSeries(len(ys)), ys
	}
	type pair struct{ x, y float64 }
	ps := make([]pair, len(ys))
	for i, y := range ys {
		ps[i] = pair{float64(i), y}
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].y < ps[j].y })
	if 2*n >= len(ps) {
		return nil, nil
	}
	ps = ps[n : len(ps)-n]
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].x < ps[j].x })
	xs, kept = make([]float64, len(ps)), make([]float64, len(ps))
	for i, p := range ps {
		xs[i], kept[i] = p.x, p.y
	}
	return xs, kept
}

// linearFitFloat 最小二乘拟合 y = a·x + b，返回 a, b。
func linearFitFloat(xs, ys []float64) (a, b float64) {
	n := float64(len(xs))
	if len(xs) == 0 {
		return 0, 0
	}
	var sx, sy, sxx, sxy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
		sxx += xs[i] * xs[i]
		sxy += xs[i] * ys[i]
	}
	den := n*sxx - sx*sx
	if math.Abs(den) < 1e-9 { // x 无变化，退化为均值
		return 0, sy / n
	}
	a = (n*sxy - sx*sy) / den
	return a, (sy - a*sx) / n
}

// chipDispersion 残差 RMSE。
func chipDispersion(xs, ys []float64, a, b float64) float64 {
	if len(ys) == 0 {
		return 0
	}
	var sum float64
	for i := range ys {
		r := ys[i] - (a*xs[i] + b)
		sum += r * r
	}
	return math.Sqrt(sum / float64(len(ys)))
}

// chipIndexSeries 生成 0..n-1 的 x 序列。
func chipIndexSeries(n int) []float64 {
	xs := make([]float64, n)
	for i := range xs {
		xs[i] = float64(i)
	}
	return xs
}

// chipMean 均值。
func chipMean(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vs {
		sum += v
	}
	return sum / float64(len(vs))
}

// chipMax 返回较大值（避免除零时用作分母下限）。
func chipMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
