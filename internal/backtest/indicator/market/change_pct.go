package market

import (
	"fmt"

	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  ChangePct — 涨跌幅 (数值型)
//  ID: 02004 = CatCodeMarket("02") + IndChangePctSeq("004")
//  数据源: GetDailyKline() 近2条 Close (int, 单位: 分)
//  K 线顺序: 最新在前（DESC）
//  涨跌幅 = (当日收盘价 - 前日收盘价) / 前日收盘价 × 100
//
//  时间窗口:
//    lookback_start=0, lookback_end=0 表示只看今天
//    lookback_start=2, lookback_end=0 表示近3天(0~2天前)都要满足
// ============================================================================

type ChangePct struct {
	indicator.BaseIndicator
}

// changePctSignal 涨跌幅信号，支持时间窗口。
type changePctSignal struct {
	indicator.BaseSignal
}

var changePctDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于-5%", Alias: "大跌", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: -5},
	{Seq: "02", Desc: "-5%~0", Alias: "小跌", Operator: indicator.OpBetween, MinThreshold: -5, MaxThreshold: 0},
	{Seq: "03", Desc: "0~5%", Alias: "小涨", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 5},
	{Seq: "04", Desc: "大于5%", Alias: "大涨", Operator: indicator.OpGT, MinThreshold: 5, MaxThreshold: 0},
}

// newChangePctSignal 创建涨跌幅信号（含时间窗口参数）。
//
// 参考 SignalLimitUpCount 的实现模式：在 OperatorOption.Params 中声明所有可配置参数，
// 前端根据此渲染输入表单。
func newChangePctSignal(def signalutil.RangeDef, name string) *changePctSignal {
	// 基础操作符 + 阈值参数
	var baseOps []indicator.OperatorOption
	switch def.Operator {
	case indicator.OpGT, indicator.OpGTE:
		baseOps = []indicator.OperatorOption{
			{
				Operator: def.Operator,
				Label:    "大于",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", def.MinThreshold, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
		}
	case indicator.OpLT, indicator.OpLTE:
		baseOps = []indicator.OperatorOption{
			{
				Operator: def.Operator,
				Label:    "小于",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", def.MaxThreshold, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
		}
	case indicator.OpBetween, indicator.OpNotBetween:
		baseOps = []indicator.OperatorOption{
			{
				Operator: def.Operator,
				Label:    "区间内",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyMin, "下限", def.MinThreshold, "%"),
					signalutil.ParamNumber(indicator.ParamKeyMax, "上限", def.MaxThreshold, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
		}
	default:
		return nil
	}

	// 默认配置包含阈值和时间窗口默认值 0-0
	cfg := &indicator.SignalConfig{
		Operator: def.Operator,
		Params: map[string]any{
			indicator.ParamKeyLookbackStart: float64(0),
			indicator.ParamKeyLookbackEnd:   float64(0),
		},
	}
	switch def.Operator {
	case indicator.OpGT, indicator.OpGTE:
		cfg.Params[indicator.ParamKeyThreshold] = def.MinThreshold
	case indicator.OpLT, indicator.OpLTE:
		cfg.Params[indicator.ParamKeyThreshold] = def.MaxThreshold
	case indicator.OpBetween, indicator.OpNotBetween:
		cfg.Params[indicator.ParamKeyMin] = def.MinThreshold
		cfg.Params[indicator.ParamKeyMax] = def.MaxThreshold
	}

	bs := indicator.NewBaseSignal(def.Seq, name, def.Desc, indicator.ValNumber, baseOps, cfg)
	if def.Alias != "" {
		bs.SetAlias(def.Alias)
	}
	return &changePctSignal{BaseSignal: bs}
}

// NewChangePct 创建涨跌幅指标实例。
func NewChangePct() *ChangePct {
	v := &ChangePct{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndChangePctSeq,
			NameStr:     "涨跌幅",
			CategoryVal: indicator.CatMarket,
			Desc:        "当日收盘价相对前日收盘价的涨跌幅",
			UnitStr:     "%",
		},
	}

	indicatorName := v.NameStr

	// 内置信号：每个都通过 newChangePctSignal 构建，自带时间窗口参数
	builtInSigs := make([]indicator.Signal, 0, len(changePctDefs))
	for _, def := range changePctDefs {
		sig := newChangePctSignal(def, indicatorName)
		if sig != nil {
			builtInSigs = append(builtInSigs, sig)
		}
	}
	v.SetBuiltInSignals(builtInSigs)

	// 自定义信号：支持阈值 + 时间窗口
	cs1 := indicator.NewBaseSignal(
		"01",
		indicatorName,
		"自定义涨跌幅筛选条件",
		indicator.ValNumber,
		[]indicator.OperatorOption{
			{
				Operator: indicator.OpGT,
				Label:    "大于",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", 3, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
			{
				Operator: indicator.OpGTE,
				Label:    "大于等于",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", 3, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
			{
				Operator: indicator.OpLT,
				Label:    "小于",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", 3, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
			{
				Operator: indicator.OpLTE,
				Label:    "小于等于",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyThreshold, "阈值", 3, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
			{
				Operator: indicator.OpBetween,
				Label:    "区间内",
				Params: []indicator.ParamDef{
					signalutil.ParamNumber(indicator.ParamKeyMin, "下限", 0, "%"),
					signalutil.ParamNumber(indicator.ParamKeyMax, "上限", 10, "%"),
					signalutil.ParamLookbackStart(0, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
				},
			},
		},
		&indicator.SignalConfig{
			Operator: indicator.OpGT,
			Params: map[string]any{
				indicator.ParamKeyThreshold:    float64(3),
				indicator.ParamKeyLookbackStart: float64(0),
				indicator.ParamKeyLookbackEnd:   float64(0),
			},
		},
	)
	v.SetCustomSignals([]indicator.Signal{&changePctSignal{BaseSignal: cs1}})
	return v
}

// Evaluate 评估股票是否满足涨跌幅条件。
//
// 支持时间窗口：如果配置了多天窗口（如2-0），则窗口内每天都要满足条件。
func (v *ChangePct) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil || len(klines) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("daily_kline").Error()}
	}

	for _, cfg := range configs {
		if s, ok := v.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*changePctSignal); ok {
				if res := ss.EvaluateWithWindow(klines, cfg); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// evaluateSingleDay 评估单日涨跌幅是否满足条件。
func (s *changePctSignal) evaluateSingleDay(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "涨跌幅", "%.2f%%", "%.1f%%", sId, config)
}

// EvaluateWithWindow 支持时间窗口的涨跌幅评估。
//
// K 线数据为 latest-first（klines[0]=今天），时间窗口参数：
//   - lookback_start=0, lookback_end=0: 只看今天
//   - lookback_start=2, lookback_end=0: 近3天(0~2天前)都要满足
//
// 窗口内每天都要满足条件才算通过。
func (s *changePctSignal) EvaluateWithWindow(klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))

	// 确保 start >= end
	if start < end {
		start, end = end, start
	}

	// K 线为 latest-first: klines[0]=今天(0天前), klines[N]=N天前
	windowSize := start - end + 1

	// 检查数据是否足够：需要 klines[start] 存在，且能计算涨跌幅需要 klines[start+1]
	if start+1 >= len(klines) {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("K线数据不足：需要至少 %d 条，实际 %d 条", start+2, len(klines)),
		}
	}

	// 窗口内每天都要满足条件：从 end 天前到 start 天前
	for dayOffset := end; dayOffset <= start; dayOffset++ {
		value := calcChangePctAtIndex(klines, dayOffset)
		res := s.evaluateSingleDay(value, config)
		if res.Result != indicator.ResultPassed {
			return &indicator.EvaluatedStock{
				Result:   indicator.ResultRejected,
				SignalID: config.SignalID,
				Message:  fmt.Sprintf("第%d天前涨跌幅%.2f%%不满足条件（窗口[%d-%d]天前）", dayOffset, value, end, start),
			}
		}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultPassed,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("近%d天（%d-%d天前）均满足涨跌幅条件", windowSize, end, start),
	}
}

// calcChangePctAtIndex 计算指定索引位置的涨跌幅（独立函数便于测试）。
func calcChangePctAtIndex(klines []*model.DailyKline, idx int) float64 {
	if idx+1 >= len(klines) {
		return 0
	}

	todayClose := float64(klines[idx].Close)
	prevClose := float64(klines[idx+1].Close)
	if prevClose <= 0 {
		return 0
	}

	return (todayClose - prevClose) / prevClose * 100
}
