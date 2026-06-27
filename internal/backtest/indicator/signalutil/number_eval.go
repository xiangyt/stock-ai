package signalutil

import (
	"fmt"
	indicator "stock-ai/internal/backtest/indicator"
)

// ============================================================================
//  RangeDef — 通用数值范围信号元数据
//
//  用于描述一个内置信号的范围定义，适用于所有数值型指标。
//  规则:
//    - OpLT / OpLTE  → 取 MaxThreshold 作为阈值（存入 ParamKeyThreshold）
//    - OpGT / OpGTE  → 取 MinThreshold 作为阈值（存入 ParamKeyThreshold）
//    - OpBetween / OpNotBetween → 取 [MinThreshold, MaxThreshold] 作为范围
// ============================================================================

type RangeDef struct {
	Seq          string                    // 2位信号序号，如 "01"
	Desc         string                    // 显示名称，如 "小于0"
	Alias        string                    // 展示别名（非空时前端优先展示别名，同类型多实例时用于区分）
	Operator     indicator.CompareOperator // 操作符
	MinThreshold float64                   // 区间下限 / GT 系列阈值
	MaxThreshold float64                   // 区间上限 / LT 系列阈值
}

// RangeDefConfig 将 RangeDef 转为对应的 SignalConfig。
func RangeDefConfig(def RangeDef) *indicator.SignalConfig {
	switch def.Operator {
	case indicator.OpLT, indicator.OpLTE:
		return &indicator.SignalConfig{
			Operator: def.Operator,
			Params:   map[string]any{indicator.ParamKeyThreshold: def.MaxThreshold},
		}
	case indicator.OpBetween, indicator.OpNotBetween:
		return &indicator.SignalConfig{
			Operator: indicator.OpBetween,
			Params:   map[string]any{indicator.ParamKeyMin: def.MinThreshold, indicator.ParamKeyMax: def.MaxThreshold},
		}
	case indicator.OpGT, indicator.OpGTE:
		return &indicator.SignalConfig{
			Operator: def.Operator,
			Params:   map[string]any{indicator.ParamKeyThreshold: def.MinThreshold},
		}
	default:
		return nil
	}
}

// BuildRangeSignals 根据 RangeDef 列表批量生成 BaseSignal，并用 wrapFn 包装为具体信号类型。
func BuildRangeSignals(defs []RangeDef, name string, ops []indicator.OperatorOption, wrapFn func(indicator.BaseSignal) indicator.Signal) []indicator.Signal {
	sigs := make([]indicator.Signal, 0, len(defs))
	for _, def := range defs {
		cfg := RangeDefConfig(def)
		if cfg == nil {
			continue
		}
		bs := indicator.NewBaseSignal(
			def.Seq,
			name,
			def.Desc,
			indicator.ValNumber,
			ops,
			cfg,
		)
		if def.Alias != "" {
			bs.SetAlias(def.Alias)
		}
		sigs = append(sigs, wrapFn(bs))
	}
	return sigs
}

// ============================================================================
//  NumberOps — 标准数值型操作符列表构造器
// ============================================================================

// NumberOpsWith 返回完整的数值型操作符列表，支持自定义单位和参数最小值约束。
func NumberOpsWith(unit string, minVal float64) []indicator.OperatorOption {
	thresh := indicator.ParamDef{Key: indicator.ParamKeyThreshold, Label: "阈值", Type: "number", Unit: unit, Min: minVal}
	minP := indicator.ParamDef{Key: indicator.ParamKeyMin, Label: "下限", Type: "number", Unit: unit, Min: minVal}
	maxP := indicator.ParamDef{Key: indicator.ParamKeyMax, Label: "上限", Type: "number", Unit: unit, Min: minVal}
	return []indicator.OperatorOption{
		{Operator: indicator.OpLT, Label: "小于", Params: []indicator.ParamDef{thresh}},
		{Operator: indicator.OpLTE, Label: "小于等于", Params: []indicator.ParamDef{thresh}},
		{Operator: indicator.OpGT, Label: "大于", Params: []indicator.ParamDef{thresh}},
		{Operator: indicator.OpGTE, Label: "大于等于", Params: []indicator.ParamDef{thresh}},
		{Operator: indicator.OpBetween, Label: "区间内", Params: []indicator.ParamDef{minP, maxP}},
		{Operator: indicator.OpNotBetween, Label: "区间外", Params: []indicator.ParamDef{minP, maxP}},
	}
}

// NumberOpsByUnit 数值型操作符列表，不限制参数最小值。
func NumberOpsByUnit(unit string) []indicator.OperatorOption {
	return NumberOpsWith(unit, 0)
}

// NumberOpsByUnitMin 数值型操作符列表，为参数设置最小值约束。
func NumberOpsByUnitMin(unit string, minVal float64) []indicator.OperatorOption {
	return NumberOpsWith(unit, minVal)
}

// ============================================================================
//  EvalNumberOp — 通用数值型操作符评估函数
// ============================================================================

// EvalNumberOp 根据 config 中的操作符对 value 进行比较并返回评估结果。
func EvalNumberOp(value float64, label, valueFmt, threshFmt string, signalID string, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	pass := func(msg string) *indicator.EvaluatedStock {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: signalID, Message: msg}
	}
	fail := func(msg string) *indicator.EvaluatedStock {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: signalID, Message: msg}
	}

	vStr := fmt.Sprintf(valueFmt, value)

	switch config.Operator {
	case indicator.OpLT:
		t := config.Threshold()
		tStr := fmt.Sprintf(threshFmt, t)
		if value < t {
			return pass(fmt.Sprintf("%s %s < %s ✓", label, vStr, tStr))
		}
		return fail(fmt.Sprintf("%s %s 不小于 %s", label, vStr, tStr))

	case indicator.OpLTE:
		t := config.Threshold()
		tStr := fmt.Sprintf(threshFmt, t)
		if value <= t {
			return pass(fmt.Sprintf("%s %s ≤ %s ✓", label, vStr, tStr))
		}
		return fail(fmt.Sprintf("%s %s 大于 %s", label, vStr, tStr))

	case indicator.OpGT:
		t := config.Threshold()
		tStr := fmt.Sprintf(threshFmt, t)
		if value > t {
			return pass(fmt.Sprintf("%s %s > %s ✓", label, vStr, tStr))
		}
		return fail(fmt.Sprintf("%s %s 不大于 %s", label, vStr, tStr))

	case indicator.OpGTE:
		t := config.Threshold()
		tStr := fmt.Sprintf(threshFmt, t)
		if value >= t {
			return pass(fmt.Sprintf("%s %s ≥ %s ✓", label, vStr, tStr))
		}
		return fail(fmt.Sprintf("%s %s 小于 %s", label, vStr, tStr))

	case indicator.OpBetween:
		minV, maxV := config.RangeMin(), config.RangeMax()
		minStr := fmt.Sprintf(threshFmt, minV)
		maxStr := fmt.Sprintf(threshFmt, maxV)
		if value >= minV && value <= maxV {
			return pass(fmt.Sprintf("%s %s 在 [%s~%s] ✓", label, vStr, minStr, maxStr))
		}
		return fail(fmt.Sprintf("%s %s 不在 [%s~%s]", label, vStr, minStr, maxStr))

	case indicator.OpNotBetween:
		minV, maxV := config.RangeMin(), config.RangeMax()
		minStr := fmt.Sprintf(threshFmt, minV)
		maxStr := fmt.Sprintf(threshFmt, maxV)
		if value < minV || value > maxV {
			return pass(fmt.Sprintf("%s %s 不在 [%s~%s] ✓", label, vStr, minStr, maxStr))
		}
		return fail(fmt.Sprintf("%s %s 在 [%s~%s] 内", label, vStr, minStr, maxStr))

	default:
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: signalID, Message: "不支持的操作符"}
	}
}
