package technical

import (
	"fmt"
	"math"

	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  CYQ — 筹码分布图指标 (数值型)
//  ID: 01021 = CatCodeTechnical("01") + IndCyqSeq("021")
//  数据源: GetDailyKline()
//
//  设计要点:
//    基于 CYQ 三角形分布 + 换手率衰减模型，计算每日筹码分布。
//    核心算法移植自 /Users/xiangyt/go/src/stock/internal/indicator/cyq.go
//
//  算法原理:
//    1. 确定价格区间，构建 Y 轴价格刻度（默认 150 档）
//    2. 逐根 K 线：
//       a) 旧筹码按换手率衰减: x *= (1 - turnoverRate)
//       b) 新筹码按三角形分布分配（均价为峰顶）
//    3. 计算获利比例、平均成本、90%/70% 筹码集中度
//
//  集中度公式:
//    concentration = (priceUpper - priceLower) / (priceUpper + priceLower)
//    集中度越小，筹码越集中
// ============================================================================

// CYQResult CYQ 筹码分布预计算结果，供该指标下所有信号复用。
//
// 数据顺序: 从旧到新 ([0]=最旧, [len-1]=最新)
// 与 NormalizeLookback 的索引映射一致 ("N天前" → dataLen-1-N)
type CYQResult struct {
	ProfitRatio []float64    // 获利比例 (0~1)，每个交易日一个值
	AvgCost     []float64    // 平均成本 (元)，50% 分位数价格
	Cost90      [][2]float64 // 90% 成本区间 [下限, 上限] (元)
	Conc90      []float64    // 90% 集中度
	Cost70      [][2]float64 // 70% 成本区间 [下限, 上限] (元)
	Conc70      []float64    // 70% 集中度
	ClosePrice  []float64    // 收盘价（元，从旧到新）

	// 以下字段仅在 BuildCYQDetail 模式下填充，用于图表渲染
	XData     []float64 // 筹码堆叠数据（最新日，150 档）
	YData     []float64 // Y 轴价格刻度（150 档）
	MinPrice  float64   // 价格区间下限
	Accuracy  float64   // 每档价格精度
}

// Cyq 默认参数（与东方财富 quotechart2022.js 保持一致）
const (
	CyqAccuracyFactor   = 150 // 纵轴价格刻度数（东财 fator=150）
	CyqDefaultKlineCount = 210 // 默认计算K线根数（东财 data_count=2×kline_count+30=2×90+30=210）
	CyqMinKlines        = 60  // 最少K线根数
)

// Cyq 筹码分布指标结构体
type Cyq struct {
	indicator.BaseIndicator
}

func NewCyq() *Cyq {
	i := &Cyq{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndCyqSeq,
			NameStr:     "筹码分布",
			CategoryVal: indicator.CatTechnical,
			Desc:        "筹码分布图（获利比例/成本集中度等）",
			UnitStr:     "",
		},
	}

	profitOps := signalutil.NumberOpsByUnit("%")
	cost90Ops := signalutil.NumberOpsByUnit("元")
	conc90Ops := signalutil.NumberOpsWith("", 0)
	cost70Ops := signalutil.NumberOpsByUnit("元")
	conc70Ops := signalutil.NumberOpsWith("", 0)

	i.SetBuiltInSignals([]indicator.Signal{
		&signalCyqProfitRatio{indicator.NewBaseSignal(
			"01", "获利比例", "当前价格以下筹码占总筹码的比例（0~100%）",
			indicator.ValNumber, profitOps,
			&indicator.SignalConfig{Operator: indicator.OpGTE, Params: map[string]any{indicator.ParamKeyThreshold: float64(50)}},
		)},
		&signalCyqCost90{indicator.NewBaseSignal(
			"02", "90%成本", "90%筹码分布的价格区间宽度（上限-下限）",
			indicator.ValNumber, cost90Ops,
			&indicator.SignalConfig{Operator: indicator.OpLTE, Params: map[string]any{indicator.ParamKeyThreshold: float64(10)}},
		)},
		&signalCyqConc90{indicator.NewBaseSignal(
			"03", "90%集中度", "90%筹码的集中度，值越小筹码越集中",
			indicator.ValNumber, conc90Ops,
			&indicator.SignalConfig{Operator: indicator.OpLTE, Params: map[string]any{indicator.ParamKeyThreshold: float64(0.15)}},
		)},
		&signalCyqCost70{indicator.NewBaseSignal(
			"04", "70%成本", "70%筹码分布的价格区间宽度（上限-下限）",
			indicator.ValNumber, cost70Ops,
			&indicator.SignalConfig{Operator: indicator.OpLTE, Params: map[string]any{indicator.ParamKeyThreshold: float64(5)}},
		)},
		&signalCyqConc70{indicator.NewBaseSignal(
			"05", "70%集中度", "70%筹码的集中度，值越小筹码越集中",
			indicator.ValNumber, conc70Ops,
			&indicator.SignalConfig{Operator: indicator.OpLTE, Params: map[string]any{indicator.ParamKeyThreshold: float64(0.10)}},
		)},
	})

	i.SetCustomSignals(nil)
	return i
}

// Evaluate CYQ 指标评估入口
//
//  1. 取最近 CyqDefaultKlineCount 根 K 线（与东财 lmt=210 一致）
//  2. 计算筹码分布结果
//  3. 将结果传给各信号分发处理
func (i *Cyq) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: err.Error()}
	}

	if len(klines) < CyqMinKlines {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: configs[0].SignalID,
			Message:  fmt.Sprintf("K线数据不足，需要至少 %d 根，当前 %d 根", CyqMinKlines, len(klines)),
		}
	}

	// 截取最近 CyqDefaultKlineCount 根 K 线（与东财 data_count=210 一致）
	// klines 输入顺序: [0]=最新, [len-1]=最旧
	if len(klines) > CyqDefaultKlineCount {
		klines = klines[:CyqDefaultKlineCount]
	}

	result := BuildCYQ(klines, CyqAccuracyFactor)

	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch v := s.(type) {
			case *signalCyqProfitRatio:
				res = v.Evaluate(result, cfg)
			case *signalCyqCost90:
				res = v.Evaluate(result, cfg)
			case *signalCyqConc90:
				res = v.Evaluate(result, cfg)
			case *signalCyqCost70:
				res = v.Evaluate(result, cfg)
			case *signalCyqConc70:
				res = v.Evaluate(result, cfg)
			default:
				return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID,
					Message: indicator.ErrUnsupportedSignal.Error()}
			}
			if res.Result == indicator.ResultPassed {
				continue
			} else {
				return res
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// ============================================================================
//  buildCYQ — 核心筹码分布计算
//
//  算法步骤:
//    1. 找价格极值，构建 Y 轴等间距价格刻度
//    2. 逐根 K 线，旧筹码衰减 + 新筹码三角形分布
//    3. 逐根 K 线，计算获利比例、平均成本、90%/70% 集中度
//
//  klines 输入: [0]=最新, [len-1]=最旧
//  结果: oldest-first (从旧到新), [0]=最旧, [len-1]=最新
// ============================================================================
// BuildCYQ — 核心筹码分布计算
//
//  算法步骤:
//    1. 找价格极值，构建 Y 轴等间距价格刻度
//    2. 逐根 K 线，旧筹码衰减 + 新筹码三角形分布
//    3. 逐根 K 线，计算获利比例、平均成本、90%/70% 集中度
//
//  klines 输入: [0]=最新, [len-1]=最旧
//  结果: oldest-first (从旧到新), [0]=最旧, [len-1]=最新
func BuildCYQ(klines []*model.DailyKline, factor int) CYQResult {
	n := len(klines)

	// 反转为 oldest-first，同时分→元
	openPrices := make([]float64, n)
	highPrices := make([]float64, n)
	lowPrices := make([]float64, n)
	closePrices := make([]float64, n)
	turnoverRates := make([]float64, n) // 换手率 (小数形式 0~1)
	for i := range klines {
		j := n - 1 - i
		openPrices[j] = float64(klines[i].Open) / 100.0
		highPrices[j] = float64(klines[i].High) / 100.0
		lowPrices[j] = float64(klines[i].Low) / 100.0
		closePrices[j] = float64(klines[i].Close) / 100.0
		turnoverRates[j] = klines[i].TurnoverRate / 100.0 // 百分比 → 小数
	}

	// 找全局价格极值
	maxPrice := 0.0
	minPrice := math.MaxFloat64
	for i := 0; i < n; i++ {
		if highPrices[i] > maxPrice {
			maxPrice = highPrices[i]
		}
		if lowPrices[i] < minPrice {
			minPrice = lowPrices[i]
		}
	}
	if maxPrice <= minPrice {
		maxPrice = minPrice + 1 // 极端情况兜底
	}

	// 精度（最小 0.01）
	accuracy := (maxPrice - minPrice) / float64(factor-1)
	if accuracy < 0.01 {
		accuracy = 0.01
	}

	// 初始化筹码分布数组
	xData := make([]float64, factor)

	// 结果数组
	profitRatio := make([]float64, n)
	avgCost := make([]float64, n)
	cost90 := make([][2]float64, n)
	conc90 := make([]float64, n)
	cost70 := make([][2]float64, n)
	conc70 := make([]float64, n)

	// 逐根 K 线计算
	for idx := 0; idx < n; idx++ {
		high := highPrices[idx]
		low := lowPrices[idx]
		open := openPrices[idx]
		close := closePrices[idx]
		avg := (open + close + high + low) / 4.0
		hsl := turnoverRates[idx]
		if hsl > 1.0 {
			hsl = 1.0
		}

		// (a) 衰减现有筹码
		for k := 0; k < factor; k++ {
			xData[k] *= (1.0 - hsl)
		}

		// (b) 计算价格索引范围
		H := int(math.Floor((high - minPrice) / accuracy))
		L := int(math.Ceil((low - minPrice) / accuracy))
		if H >= factor {
			H = factor - 1
		}
		if L < 0 {
			L = 0
		}

		// (c) 计算 G 点坐标
		var gPoint [2]float64
		if high == low {
			// 一字板：矩形分布
			gPoint[0] = float64(factor - 1)
		} else {
			gPoint[0] = 2.0 / (high - low)
		}
		gPoint[1] = math.Floor((avg - minPrice) / accuracy)

		// (d) 分配新筹码
		if high == low {
			// 一字板：矩形
			gIdx := int(gPoint[1])
			if gIdx >= 0 && gIdx < factor {
				xData[gIdx] += gPoint[0] * hsl / 2.0
			}
		} else {
			// 正常K线：三角形分布
			for j := L; j <= H && j < factor; j++ {
				if j < 0 {
					continue
				}
				curPrice := minPrice + accuracy*float64(j)

				if avg == low {
					// 均价等于最低价，上半部分全部取最大值
					xData[j] += gPoint[0] * hsl
				} else if high == avg {
					// 均价等于最高价，下半部分全部取最大值
					xData[j] += gPoint[0] * hsl
				} else if curPrice <= avg {
					// 上半三角（从低到均值）
					xData[j] += (curPrice - low) / (avg - low) * gPoint[0] * hsl
				} else {
					// 下半三角（从均值到高）
					xData[j] += (high - curPrice) / (high - avg) * gPoint[0] * hsl
				}
			}
		}

		// (e) 精度处理 & 计算总筹码
		totalChips := 0.0
		for k := 0; k < factor; k++ {
			xData[k] = round12(xData[k])
			totalChips += xData[k]
		}

		if totalChips <= 0 {
			continue
		}

		// 计算当前日的各项指标
		currentClose := closePrices[idx]

		// 获利比例
		profitRatio[idx] = cyqGetBenefitPart(currentClose, minPrice, accuracy, factor, xData, totalChips)

		// 平均成本 (50% 分位数)
		avgCost[idx] = cyqGetCostByChip(totalChips*0.5, minPrice, accuracy, factor, xData)

		// 90% 筹码集中度
		pc90 := cyqComputePercentChips(0.9, totalChips, minPrice, accuracy, factor, xData)
		cost90[idx] = pc90.PriceRange
		conc90[idx] = pc90.Concentration

		// 70% 筹码集中度
		pc70 := cyqComputePercentChips(0.7, totalChips, minPrice, accuracy, factor, xData)
		cost70[idx] = pc70.PriceRange
		conc70[idx] = pc70.Concentration
	}

	return CYQResult{
		ProfitRatio: profitRatio,
		AvgCost:     avgCost,
		Cost90:      cost90,
		Conc90:      conc90,
		Cost70:      cost70,
		Conc70:      conc70,
		ClosePrice:  closePrices,
		XData:       xData,
		YData:       buildYRange(minPrice, accuracy, factor),
		MinPrice:    minPrice,
		Accuracy:    accuracy,
	}
}

// buildYRange 构建 Y 轴价格刻度
func buildYRange(minPrice, accuracy float64, factor int) []float64 {
	yRange := make([]float64, factor)
	for i := 0; i < factor; i++ {
		yRange[i] = math.Round((minPrice+accuracy*float64(i))*100) / 100
	}
	return yRange
}

// cyqGetBenefitPart 计算获利比例
// 当前价格以下的所有筹码占总筹码的比例
func cyqGetBenefitPart(price, minPrice, accuracy float64, factor int, xData []float64, totalChips float64) float64 {
	below := 0.0
	for i := 0; i < factor; i++ {
		x := round12(xData[i])
		if price >= minPrice+float64(i)*accuracy {
			below += x
		}
	}
	if totalChips == 0 {
		return 0
	}
	return below / totalChips
}

// cyqGetCostByChip 从低价到高价累加筹码，找到累计筹码首次超过目标值时的价格
func cyqGetCostByChip(chip, minPrice, accuracy float64, factor int, xData []float64) float64 {
	sum := 0.0
	for i := 0; i < factor; i++ {
		x := round12(xData[i])
		if sum+x > chip {
			return minPrice + float64(i)*accuracy
		}
		sum += x
	}
	return 0
}

// percentChip 百分比筹码数据
type percentChip struct {
	PriceRange    [2]float64 // 价格区间 [下限, 上限]
	Concentration float64    // 集中度
}

// cyqComputePercentChips 计算指定百分比的筹码集中度
// percent=0.9 → 取 5%~95% 区间；percent=0.7 → 取 15%~85% 区间
func cyqComputePercentChips(percent, totalChips, minPrice, accuracy float64, factor int, xData []float64) percentChip {
	// 分位数位置
	lowP := (1.0 - percent) / 2.0
	highP := (1.0 + percent) / 2.0

	prLow := cyqGetCostByChip(totalChips*lowP, minPrice, accuracy, factor, xData)
	prHigh := cyqGetCostByChip(totalChips*highP, minPrice, accuracy, factor, xData)

	// 集中度 = 价格区间宽度 / 两端价格之和
	concentration := 0.0
	if prLow+prHigh != 0 {
		concentration = (prHigh - prLow) / (prHigh + prLow)
	}

	return percentChip{
		PriceRange:    [2]float64{prLow, prHigh},
		Concentration: concentration,
	}
}

// round2 保留2位小数
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

// round4 保留4位小数
func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}

// round12 保留12位有效精度（对应 JS 的 toPrecision(12)）
func round12(f float64) float64 {
	return math.Round(f*1e12) / 1e12
}

// ============================================================================
//  Signal 01 — 获利比例
//
//  判定规则:
//    最新交易日的获利比例满足阈值条件
//    获利比例 = 当前价格以下筹码 / 总筹码 (0~100%)
// ============================================================================

type signalCyqProfitRatio struct {
	indicator.BaseSignal
}

func (s *signalCyqProfitRatio) Evaluate(result CYQResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	latestIdx := len(result.ProfitRatio) - 1
	if latestIdx < 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: "无筹码数据"}
	}
	value := result.ProfitRatio[latestIdx] * 100.0 // 转为百分比
	return signalutil.EvalNumberOp(value, "获利比例", "%.2f%%", "%.0f", sId, config)
}

// ============================================================================
//  Signal 02 — 90%成本
//
//  判定规则:
//    90% 筹码成本区间宽度满足阈值条件
//    宽度 = 上限 - 下限
// ============================================================================

type signalCyqCost90 struct {
	indicator.BaseSignal
}

func (s *signalCyqCost90) Evaluate(result CYQResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	latestIdx := len(result.Cost90) - 1
	if latestIdx < 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: "无筹码数据"}
	}
	// 区间宽度 = 上限 - 下限
	width := result.Cost90[latestIdx][1] - result.Cost90[latestIdx][0]
	return signalutil.EvalNumberOp(round2(width), "90%成本区间宽度", "%.2f元", "%.0f", sId, config)
}

// ============================================================================
//  Signal 03 — 90%集中度
//
//  判定规则:
//    90% 筹码集中度满足阈值条件
//    集中度 = (上限 - 下限) / (上限 + 下限)
//    集中度越小，筹码越集中
// ============================================================================

type signalCyqConc90 struct {
	indicator.BaseSignal
}

func (s *signalCyqConc90) Evaluate(result CYQResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	latestIdx := len(result.Conc90) - 1
	if latestIdx < 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: "无筹码数据"}
	}
	value := result.Conc90[latestIdx]
	return signalutil.EvalNumberOp(round4(value), "90%集中度", "%.4f", "%.2f", sId, config)
}

// ============================================================================
//  Signal 04 — 70%成本
//
//  判定规则:
//    70% 筹码成本区间宽度满足阈值条件
//    宽度 = 上限 - 下限
// ============================================================================

type signalCyqCost70 struct {
	indicator.BaseSignal
}

func (s *signalCyqCost70) Evaluate(result CYQResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	latestIdx := len(result.Cost70) - 1
	if latestIdx < 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: "无筹码数据"}
	}
	width := result.Cost70[latestIdx][1] - result.Cost70[latestIdx][0]
	return signalutil.EvalNumberOp(round2(width), "70%成本区间宽度", "%.2f元", "%.0f", sId, config)
}

// ============================================================================
//  Signal 05 — 70%集中度
//
//  判定规则:
//    70% 筹码集中度满足阈值条件
//    集中度 = (上限 - 下限) / (上限 + 下限)
//    集中度越小，筹码越集中
// ============================================================================

type signalCyqConc70 struct {
	indicator.BaseSignal
}

func (s *signalCyqConc70) Evaluate(result CYQResult, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	latestIdx := len(result.Conc70) - 1
	if latestIdx < 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId,
			Message: "无筹码数据"}
	}
	value := result.Conc70[latestIdx]
	return signalutil.EvalNumberOp(round4(value), "70%集中度", "%.4f", "%.2f", sId, config)
}
