package signalutil

import (
	"fmt"
	indicator "stock-ai/internal/indicator"
)

// ============================================================================
//  参数工厂函数
// ============================================================================

// ParamNumber 创建数值型参数定义
func ParamNumber(key, label string, defaultVal float64, unit string) indicator.ParamDef {
	return indicator.ParamDef{
		Key:      key,
		Label:    label,
		Type:     "number",
		Required: true,
		Default:  defaultVal,
		Unit:     unit,
	}
}

// ParamRange 创建范围型参数定义 (带 min/max 和占位提示)
func ParamRange(key, label string, minVal, maxVal, defaultVal float64, unit string) indicator.ParamDef {
	return indicator.ParamDef{
		Key:         key,
		Label:       label,
		Type:        "range",
		Required:    true,
		Default:     defaultVal,
		Min:         minVal,
		Max:         maxVal,
		Unit:        unit,
		Placeholder: fmt.Sprintf("%.1f ~ %.1f", minVal, maxVal),
	}
}

// ParamDays 创建回看天数参数定义 (默认范围 5~120 天)
func ParamDays(defaultVal int) indicator.ParamDef {
	return indicator.ParamDef{
		Key:      indicator.ParamKeyDays,
		Label:    "回看天数",
		Type:     "days",
		Required: false,
		Default:  float64(defaultVal),
		Min:      5,
		Max:      120,
		Unit:     "天",
	}
}

// ============================================================================
//  操作符集合 — 按值类型分组的常用操作符选项
// ============================================================================

// NumberOps 数值型比较操作符 (gt/gte/lt/lte/between)，不含单位。
// 需要携带单位或 min 约束时请改用 NumberOpsWith / NumberOpsByUnit / NumberOpsByUnitMin。
var NumberOps = []indicator.OperatorOption{
	{Operator: indicator.OpGT, Label: "大于", Params: []indicator.ParamDef{
		ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, ""),
	}},
	{Operator: indicator.OpGTE, Label: "大于等于", Params: []indicator.ParamDef{
		ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, ""),
	}},
	{Operator: indicator.OpLT, Label: "小于", Params: []indicator.ParamDef{
		ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, ""),
	}},
	{Operator: indicator.OpLTE, Label: "小于等于", Params: []indicator.ParamDef{
		ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, ""),
	}},
	{Operator: indicator.OpBetween, Label: "区间内", Params: []indicator.ParamDef{
		ParamNumber(indicator.ParamKeyMin, "下限", 0, ""),
		ParamNumber(indicator.ParamKeyMax, "上限", 100, ""),
	}},
}

// EnumOps 构建枚举型操作符列表，根据传入选项同时支持单选(eq/neq)和多选(in/not_in)。
func EnumOps(options []indicator.EnumOption) []indicator.OperatorOption {
	selectParam := indicator.ParamDef{
		Key:      indicator.ParamKeyThreshold,
		Label:    "匹配值",
		Type:     "select",
		Required: true,
		Options:  options,
	}
	multiParam := indicator.ParamDef{
		Key:      indicator.ParamKeyValues,
		Label:    "匹配值",
		Type:     "select_multi",
		Required: true,
		Options:  options,
	}
	return []indicator.OperatorOption{
		{Operator: indicator.OpEQ, Label: "等于", Params: []indicator.ParamDef{selectParam}},
		{Operator: indicator.OpNEQ, Label: "不等于", Params: []indicator.ParamDef{selectParam}},
		{Operator: indicator.OpIn, Label: "属于", Params: []indicator.ParamDef{multiParam}},
		{Operator: indicator.OpNotIn, Label: "不属于", Params: []indicator.ParamDef{multiParam}},
	}
}

// BoolOps 布尔型判断操作符 (无需额外参数)
var BoolOps = []indicator.OperatorOption{
	{Operator: indicator.OpEQ, Label: "是"},
	{Operator: indicator.OpNEQ, Label: "不是"},
}

// CrossOps 序列交叉操作符 (金叉/死叉，可配置快慢线周期)
var CrossOps = []indicator.OperatorOption{
	{Operator: indicator.OpCrossAbove, Label: "上穿(金叉)", Params: []indicator.ParamDef{
		ParamDays(12), ParamDays(26),
	}},
	{Operator: indicator.OpCrossBelow, Label: "下穿(死叉)", Params: []indicator.ParamDef{
		ParamDays(12), ParamDays(26),
	}},
}

// ============================================================================
//  数值型信号的通用模板工厂
// ============================================================================

const (
	SigSeqNumberGT      = "01" // 大于
	SigSeqNumberLT      = "02" // 小于
	SigSeqNumberBetween = "03" // 区间内
)

// NumberSignalGT 创建"大于"信号模板 (序号=01)
func NumberSignalGT(unit string) indicator.Signal {
	sig := indicator.NewBaseSignal(
		SigSeqNumberGT,
		"大于",
		fmt.Sprintf("指标值大于阈值 (%s)", unit),
		indicator.ValNumber,
		[]indicator.OperatorOption{
			{Operator: indicator.OpGT, Label: "大于", Params: []indicator.ParamDef{
				ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, unit),
			}},
			{Operator: indicator.OpGTE, Label: "大于等于", Params: []indicator.ParamDef{
				ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, unit),
			}},
		},
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 0}},
	)
	return &sig
}

// NumberSignalLT 创建"小于"信号模板 (序号=02)
func NumberSignalLT(unit string) indicator.Signal {
	sig := indicator.NewBaseSignal(
		SigSeqNumberLT,
		"小于",
		fmt.Sprintf("指标值小于阈值 (%s)", unit),
		indicator.ValNumber,
		[]indicator.OperatorOption{
			{Operator: indicator.OpLT, Label: "小于", Params: []indicator.ParamDef{
				ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, unit),
			}},
			{Operator: indicator.OpLTE, Label: "小于等于", Params: []indicator.ParamDef{
				ParamNumber(indicator.ParamKeyThreshold, "阈值", 0, unit),
			}},
		},
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{indicator.ParamKeyThreshold: 0}},
	)
	return &sig
}

// NumberSignalBetween 创建"区间内"信号模板 (序号=03)
func NumberSignalBetween(unit string) indicator.Signal {
	sig := indicator.NewBaseSignal(
		SigSeqNumberBetween,
		"区间内",
		fmt.Sprintf("指标值在指定区间内 (%s)", unit),
		indicator.ValNumber,
		[]indicator.OperatorOption{
			{Operator: indicator.OpBetween, Label: "区间内", Params: []indicator.ParamDef{
				ParamNumber(indicator.ParamKeyMin, "下限", 0, unit),
				ParamNumber(indicator.ParamKeyMax, "上限", 100, unit),
			}},
			{Operator: indicator.OpNotBetween, Label: "区间外", Params: []indicator.ParamDef{
				ParamNumber(indicator.ParamKeyMin, "下限", 0, unit),
				ParamNumber(indicator.ParamKeyMax, "上限", 100, unit),
			}},
		},
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 0, indicator.ParamKeyMax: 100}},
	)
	return &sig
}

// ============================================================================
//  布尔型信号模板
// ============================================================================

const (
	SigSeqBoolIs  = "01" // 是
	SigSeqBoolNot = "02" // 不是
)

// BoolSignalIs 创建"是"布尔信号 (序号=01)
func BoolSignalIs(name, desc string) indicator.Signal {
	sig := indicator.NewBaseSignal(
		SigSeqBoolIs, name, desc,
		indicator.ValBool, BoolOps,
		&indicator.SignalConfig{Operator: indicator.OpEQ},
	)
	return &sig
}

// BoolSignalNot 创建"不是"布尔信号 (序号=02)
func BoolSignalNot(name, desc string) indicator.Signal {
	sig := indicator.NewBaseSignal(
		SigSeqBoolNot, name, desc,
		indicator.ValBool, BoolOps,
		&indicator.SignalConfig{Operator: indicator.OpNEQ},
	)
	return &sig
}

// DefaultBoolSignals 返回默认布尔信号对 (是/否)
func DefaultBoolSignals(isName, notName, desc string) []indicator.Signal {
	return []indicator.Signal{
		BoolSignalIs(isName, desc),
		BoolSignalNot(notName, desc),
	}
}

// DefaultBoolSignal 单数别名 (便捷调用)
func DefaultBoolSignal(isName, notName, desc string) []indicator.Signal {
	return DefaultBoolSignals(isName, notName, desc)
}

// ============================================================================
//  枚举型信号模板
// ============================================================================

const SigSeqEnumMatch = "01"

// EnumSignalMatch 创建枚举匹配信号 (序号=01)
func EnumSignalMatch(name, desc string, options []indicator.EnumOption) indicator.Signal {
	sig := indicator.NewBaseSignal(
		SigSeqEnumMatch, name, desc,
		indicator.ValEnum, EnumOps(options),
		&indicator.SignalConfig{Operator: indicator.OpIn, Params: map[string]any{indicator.ParamKeyValues: []string{}}},
	)
	return &sig
}

// ============================================================================
//  序列型信号模板 (MACD金叉/死叉、均线交叉等需扫描历史数据的信号)
// ============================================================================

const (
	SigSeqSeriesCrossAbove = "01" // 上穿/金叉
	SigSeqSeriesCrossBelow = "02" // 下穿/死叉
)

// SeriesSignalCrossAbove 创建上穿(金叉)序列信号 (序号=01)
func SeriesSignalCrossAbove(fastName, slowName string) indicator.Signal {
	sig := indicator.NewBaseSignal(
		SigSeqSeriesCrossAbove,
		fmt.Sprintf("%s上%s", fastName, slowName),
		fmt.Sprintf("%s从下方穿越%s（金叉）", fastName, slowName),
		indicator.ValSeries, CrossOps,
		&indicator.SignalConfig{Operator: indicator.OpCrossAbove, Params: map[string]any{indicator.ParamKeyFastDays: 12, indicator.ParamKeySlowDays: 26}},
	)
	return &sig
}

// SeriesSignalCrossBelow 创建下穿(死叉)序列信号 (序号=02)
func SeriesSignalCrossBelow(fastName, slowName string) indicator.Signal {
	sig := indicator.NewBaseSignal(
		SigSeqSeriesCrossBelow,
		fmt.Sprintf("%s下%s", fastName, slowName),
		fmt.Sprintf("%s从上方穿越%s（死叉）", fastName, slowName),
		indicator.ValSeries, CrossOps,
		&indicator.SignalConfig{Operator: indicator.OpCrossBelow, Params: map[string]any{indicator.ParamKeyFastDays: 12, indicator.ParamKeySlowDays: 26}},
	)
	return &sig
}
