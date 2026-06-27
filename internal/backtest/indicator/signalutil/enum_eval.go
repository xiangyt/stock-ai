package signalutil

import (
	"fmt"
	indicator "stock-ai/internal/backtest/indicator"
)

// ============================================================================
//  EnumDef — 通用枚举型信号元数据
//
//  用于描述一个枚举型信号的定义，适用于所有枚举值匹配场景。
//  规则:
//    - OpEQ / OpNEQ  → 取 Value 作为匹配值（存入 ParamKeyThreshold）
//    - OpIn / OpNotIn → 取 Values 作为匹配列表（存入 ParamKeyValues）
// ============================================================================

type EnumDef struct {
	Seq      string                    // 2位信号序号，如 "01"
	Desc     string                    // 显示名称，如 "属于主板"
	Alias    string                    // 展示别名（非空时前端优先展示别名，同类型多实例时用于区分）
	Operator indicator.CompareOperator // 操作符
	Options  []indicator.EnumOption   // 枚举可选项（前端渲染用）
	Value    string                    // 单值匹配默认值（OpEQ/OpNEQ 使用）
	Values   []string                 // 多值匹配默认值（OpIn/OpNotIn 使用）
}

// EnumDefConfig 将 EnumDef 转为对应的 SignalConfig。
func EnumDefConfig(def EnumDef) *indicator.SignalConfig {
	switch def.Operator {
	case indicator.OpEQ, indicator.OpNEQ:
		return &indicator.SignalConfig{
			Operator: def.Operator,
			Params:   map[string]any{indicator.ParamKeyThreshold: def.Value},
		}
	case indicator.OpIn, indicator.OpNotIn:
		// 去重
		seen := make(map[string]bool, len(def.Values))
		uniq := make([]string, 0, len(def.Values))
		for _, v := range def.Values {
			if !seen[v] {
				seen[v] = true
				uniq = append(uniq, v)
			}
		}
		return &indicator.SignalConfig{
			Operator: indicator.OpIn,
			Params:   map[string]any{indicator.ParamKeyValues: uniq},
		}
	default:
		return nil
	}
}

// BuildEnumSignals 根据 EnumDef 列表批量生成 BaseSignal，并用 wrapFn 包装为具体信号类型。
func BuildEnumSignals(defs []EnumDef, name string, ops []indicator.OperatorOption, wrapFn func(indicator.BaseSignal) indicator.Signal) []indicator.Signal {
	sigs := make([]indicator.Signal, 0, len(defs))
	for _, def := range defs {
		cfg := EnumDefConfig(def)
		if cfg == nil {
			continue
		}
		bs := indicator.NewBaseSignal(
			def.Seq,
			name,
			def.Desc,
			indicator.ValEnum,
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
//  EvalEnumOp — 通用枚举型操作符评估函数
// ============================================================================

// EvalEnumOp 根据 config 中的操作符对 value（字符串）进行枚举匹配并返回评估结果。
//  value:   当前待评估的枚举值（如行业名称）
//  label:   展示名称（如 "所属行业"）
//  signalID: 信号ID
//  config:  信号配置
func EvalEnumOp(value, label, signalID string, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	pass := func(msg string) *indicator.EvaluatedStock {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: signalID, Message: msg}
	}
	fail := func(msg string) *indicator.EvaluatedStock {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: signalID, Message: msg}
	}

	switch config.Operator {
	case indicator.OpEQ:
		target := config.GetString(indicator.ParamKeyThreshold, "")
		if value == target {
			return pass(fmt.Sprintf("%s = %s ✓", label, target))
		}
		return fail(fmt.Sprintf("%s = %s，实际为 %s", label, target, value))

	case indicator.OpNEQ:
		target := config.GetString(indicator.ParamKeyThreshold, "")
		if value != target {
			return pass(fmt.Sprintf("%s ≠ %s ✓", label, target))
		}
		return fail(fmt.Sprintf("%s = %s", label, target))

	case indicator.OpIn:
		targets := config.Values()
		if len(targets) == 0 {
			return fail(fmt.Sprintf("%s 未设置匹配值", label))
		}
		for _, t := range targets {
			if value == t {
				return pass(fmt.Sprintf("%s 在 [%v] 中 ✓", label, targets))
			}
		}
		return fail(fmt.Sprintf("%s = %s，不在 [%v] 中", label, value, targets))

	case indicator.OpNotIn:
		targets := config.Values()
		if len(targets) == 0 {
			return pass(fmt.Sprintf("%s 未设置排除值 ✓", label))
		}
		for _, t := range targets {
			if value == t {
				return fail(fmt.Sprintf("%s 在排除列表 [%v] 中", label, targets))
			}
		}
		return pass(fmt.Sprintf("%s = %s，不在 [%v] 中 ✓", label, value, targets))

	default:
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: signalID, Message: "不支持的枚举操作符"}
	}
}
