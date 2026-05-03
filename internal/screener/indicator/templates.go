package indicator

import "fmt"

// ============================================================================
//  参数工厂函数
// ============================================================================

// ParamNumber 创建数值型参数定义
func ParamNumber(key, label string, defaultVal float64, unit string) ParamDef {
	return ParamDef{
		Key:      key,
		Label:    label,
		Type:     "number",
		Required: true,
		Default:  defaultVal,
		Unit:     unit,
	}
}

// ParamRange 创建范围型参数定义 (带 min/max 和占位提示)
func ParamRange(key, label string, minVal, maxVal, defaultVal float64, unit string) ParamDef {
	return ParamDef{
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
func ParamDays(defaultVal int) ParamDef {
	return ParamDef{
		Key:      "days",
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
//
//  这些预定义集合供模板工厂和指标实现直接引用，
//  避免在每个信号创建时重复构建操作符列表。
// ============================================================================

// NumberOps 数值型比较操作符 (gt/gte/lt/lte/between)
var NumberOps = []OperatorOption{
	{Operator: OpGT, Label: "大于", Params: []ParamDef{
		ParamNumber("threshold", "阈值", 0, ""),
	}},
	{Operator: OpGTE, Label: "大于等于", Params: []ParamDef{
		ParamNumber("threshold", "阈值", 0, ""),
	}},
	{Operator: OpLT, Label: "小于", Params: []ParamDef{
		ParamNumber("threshold", "阈值", 0, ""),
	}},
	{Operator: OpLTE, Label: "小于等于", Params: []ParamDef{
		ParamNumber("threshold", "阈值", 0, ""),
	}},
	{Operator: OpBetween, Label: "区间内", Params: []ParamDef{
		ParamNumber("min", "下限", 0, ""),
		ParamNumber("max", "上限", 100, ""),
	}},
}

// EnumOps 构建枚举型匹配操作符 (in/not_in)，根据传入选项动态生成参数定义
func EnumOps(options []EnumOption) []OperatorOption {
	enumParam := ParamDef{
		Key: "values", Label: "选择值", Type: "select_multi",
		Required: true, Options: options,
	}
	return []OperatorOption{
		{Operator: OpIn, Label: "属于", Params: []ParamDef{enumParam}},
		{Operator: OpNotIn, Label: "不属于", Params: []ParamDef{enumParam}},
	}
}

// BoolOps 布尔型判断操作符 (无需额外参数)
var BoolOps = []OperatorOption{
	{Operator: OpEQ, Label: "是"},
	{Operator: OpNEQ, Label: "不是"},
}

// CrossOps 序列交叉操作符 (金叉/死叉，可配置快慢线周期)
var CrossOps = []OperatorOption{
	{Operator: OpCrossAbove, Label: "上穿(金叉)", Params: []ParamDef{
		ParamDays(12), ParamDays(26),
	}},
	{Operator: OpCrossBelow, Label: "下穿(死叉)", Params: []ParamDef{
		ParamDays(12), ParamDays(26),
	}},
}

// ============================================================================
//  数值型信号的通用模板工厂
//
//  返回 Signal 接口 (底层 *BaseSignal, 只含2位序号, 完整ID由指标动态生成)
//
//  这些模板可复用到任意数值型指标:
//    PE指标挂载 → FullID("04001") + "0" + SigSeqNumberGT = "04001001"
//    ROE指标挂载 → FullID("04002") + "0" + SigSeqNumberGT = "04002001"
// ============================================================================

const (
	SigSeqNumberGT      = "01" // 大于
	SigSeqNumberLT      = "02" // 小于
	SigSeqNumberBetween = "03" // 区间内
)

// NumberSignalGT 创建"大于"信号模板 (序号=01)
// 适用场景: PE>30、ROE>15% 等数值下限筛选
func NumberSignalGT(unit string) Signal {
	sig := NewBaseSignal(
		SigSeqNumberGT,
		"大于",
		fmt.Sprintf("指标值大于阈值 (%s)", unit),
		ValNumber,
		[]OperatorOption{
			{Operator: OpGT, Label: "大于", Params: []ParamDef{
				ParamNumber("threshold", "阈值", 0, unit),
			}},
			{Operator: OpGTE, Label: "大于等于", Params: []ParamDef{
				ParamNumber("threshold", "阈值", 0, unit),
			}},
		},
		&SignalConfig{Operator: OpGT, Params: map[string]any{"threshold": 0}},
	)
	return &sig
}

// NumberSignalLT 创建"小于"信号模板 (序号=02)
// 适用场景: PB<3、市值<100亿 等数值上限筛选
func NumberSignalLT(unit string) Signal {
	sig := NewBaseSignal(
		SigSeqNumberLT,
		"小于",
		fmt.Sprintf("指标值小于阈值 (%s)", unit),
		ValNumber,
		[]OperatorOption{
			{Operator: OpLT, Label: "小于", Params: []ParamDef{
				ParamNumber("threshold", "阈值", 0, unit),
			}},
			{Operator: OpLTE, Label: "小于等于", Params: []ParamDef{
				ParamNumber("threshold", "阈值", 0, unit),
			}},
		},
		&SignalConfig{Operator: OpLT, Params: map[string]any{"threshold": 0}},
	)
	return &sig
}

// NumberSignalBetween 创建"区间内"信号模板 (序号=03)
// 适用场景: PE在10~30之间、ROE在8%~20%等区间筛选
func NumberSignalBetween(unit string) Signal {
	sig := NewBaseSignal(
		SigSeqNumberBetween,
		"区间内",
		fmt.Sprintf("指标值在指定区间内 (%s)", unit),
		ValNumber,
		[]OperatorOption{
			{Operator: OpBetween, Label: "区间内", Params: []ParamDef{
				ParamNumber("min", "下限", 0, unit),
				ParamNumber("max", "上限", 100, unit),
			}},
			{Operator: OpNotBetween, Label: "区间外", Params: []ParamDef{
				ParamNumber("min", "下限", 0, unit),
				ParamNumber("max", "上限", 100, unit),
			}},
		},
		&SignalConfig{Operator: OpBetween, Params: map[string]any{"min": 0, "max": 100}},
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
// 适用场景: 涨停、跌停、ST、新股等布尔属性判断
func BoolSignalIs(name, desc string) Signal {
	sig := NewBaseSignal(
		SigSeqBoolIs, name, desc,
		ValBool, BoolOps,
		&SignalConfig{Operator: OpEQ},
	)
	return &sig
}

// BoolSignalNot 创建"不是"布尔信号 (序号=02)
// 适用场景: 排除涨停股、排除ST股等反向过滤
func BoolSignalNot(name, desc string) Signal {
	sig := NewBaseSignal(
		SigSeqBoolNot, name, desc,
		ValBool, BoolOps,
		&SignalConfig{Operator: OpNEQ},
	)
	return &sig
}

// DefaultBoolSignals 返回默认布尔信号对 (是/否)
// 便捷方法：一次性创建正反两个布尔信号
func DefaultBoolSignals(isName, notName, desc string) []Signal {
	return []Signal{
		BoolSignalIs(isName, desc),
		BoolSignalNot(notName, desc),
	}
}

// DefaultBoolSignal 单数别名 (便捷调用)
func DefaultBoolSignal(isName, notName, desc string) []Signal {
	return DefaultBoolSignals(isName, notName, desc)
}

// ============================================================================
//  枚举型信号模板
// ============================================================================

const SigSeqEnumMatch = "01"

// EnumSignalMatch 创建枚举匹配信号 (序号=01)
// 适用场景: 板块归属、行业分类、概念标签等多选一/多选多筛选
func EnumSignalMatch(name, desc string, options []EnumOption) Signal {
	sig := NewBaseSignal(
		SigSeqEnumMatch, name, desc,
		ValEnum, EnumOps(options),
		&SignalConfig{Operator: OpIn, Params: map[string]any{"values": []string{}}},
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
// 适用场景: MACD金叉、均线上穿、KDJ金叉等技术信号
// 参数 fastName/slowName 用于动态生成显示名 (如 "MACD上DIF" / "MA5上MA20")
func SeriesSignalCrossAbove(fastName, slowName string) Signal {
	sig := NewBaseSignal(
		SigSeqSeriesCrossAbove,
		fmt.Sprintf("%s上%s", fastName, slowName),
		fmt.Sprintf("%s从下方穿越%s（金叉）", fastName, slowName),
		ValSeries, CrossOps,
		&SignalConfig{Operator: OpCrossAbove, Params: map[string]any{"fast_days": 12, "slow_days": 26}},
	)
	return &sig
}

// SeriesSignalCrossBelow 创建下穿(死叉)序列信号 (序号=02)
// 适用场景: MACD死叉、均线下穿、KDJ死叉等技术信号
func SeriesSignalCrossBelow(fastName, slowName string) Signal {
	sig := NewBaseSignal(
		SigSeqSeriesCrossBelow,
		fmt.Sprintf("%s下%s", fastName, slowName),
		fmt.Sprintf("%s从上方穿越%s（死叉）", fastName, slowName),
		ValSeries, CrossOps,
		&SignalConfig{Operator: OpCrossBelow, Params: map[string]any{"fast_days": 12, "slow_days": 26}},
	)
	return &sig
}
