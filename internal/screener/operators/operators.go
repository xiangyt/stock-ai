// Package operators 指标操作符选项定义
//
// 本包提供「自由组合模式」的核心构件：
//   - OperatorParamDef / OperatorOption 类型定义（纯数据结构，描述前端需要渲染什么控件）
//   - 四类工厂函数：NumberOps() / BoolOps() / EnumOps() / SeriesOps()
//
// 使用方式：
//
//	各指标文件（如 macd.go）在 init() 中从本包选取合适的操作符组合，
//	通过 indicators.RegisterOperators() 注册到全局注册表。
//
// 示例：
//
//	import "stock-ai/internal/screener/operators"
//
//	func init() {
//	    indicators.RegisterOperators("macd_cross", operators.SeriesOps())
//	    indicators.RegisterOperators("pe_ttm", operators.NumberOps())
//	}
package operators

import "stock-ai/internal/model"

// ============================================================================
//  类型定义 — 描述前端需要渲染的控件
// ============================================================================

// OperatorParamDef 操作符对应的参数定义（告诉前端需要什么输入控件）
type OperatorParamDef struct {
	Key         string        `json:"key"`                   // 参数字段名: "value_number", "min_value"
	Label       string        `json:"label"`                 // 显示名: "阈值", "下界"
	Type        model.ParamType `json:"type"`                // 控件类型: number, range, days
	Default     interface{}   `json:"default"`              // 默认值
	Min         float64       `json:"min,omitempty"`         // 最小值
	Max         float64       `json:"max,omitempty"`         // 最大值
	Step        float64       `json:"step,omitempty"`        // 步长
	Unit        string        `json:"unit,omitempty"`        // 单位
	Placeholder string        `json:"placeholder,omitempty"` // placeholder
	Description string        `json:"description,omitempty"` // 说明文字
	Required    bool          `json:"required"`             // 是否必填
}

// OperatorOption 单个操作符选项的完整描述
// 前端根据此结构自动渲染：操作符下拉 + 参数输入表单
type OperatorOption struct {
	Operator model.CompareOperator `json:"operator"`        // 操作符值
	Symbol   string                `json:"symbol"`          // 显示符号: ">", "[]", "金叉"
	Label    string                `json:"label"`           // 中文标签: "大于", "区间内", "金叉/上穿"
	Example  string                `json:"example"`         // 使用示例: "PE > 20", "PE 在 20~50 之间", "近3天内金叉"
	Params   []OperatorParamDef    `json:"params"`          // 该操作符需要的参数列表
}

// ============================================================================
//  工厂函数 — 四类标准操作符集合
//
//  各指标文件根据自身特性选取合适的子集或全集。
//  例如：MACD交叉用 SeriesOps()，PE用 NumberOps()，涨停用 BoolOps()
// ============================================================================

// NumberOps 数值型操作符 (number): PE, MA, ROE, 价格等单值/区间比较
func NumberOps() []OperatorOption {
	return []OperatorOption{
		{
			Operator: model.OpGT,
			Symbol:   ">",
			Label:    "大于",
			Example:  "PE > 20",
			Params: []OperatorParamDef{{
				Key: "value_number", Label: "阈值", Type: model.ParamTypeNumber,
				Default: float64(0), Min: -1e10, Max: 1e10, Step: 0.01,
				Placeholder: "如: 20", Description: "大于此值的股票通过",
				Required: true,
			}},
		},
		{
			Operator: model.OpGTE,
			Symbol:   ">=",
			Label:    "大于等于",
			Example:  "ROE >= 15%",
			Params: []OperatorParamDef{{
				Key: "value_number", Label: "阈值", Type: model.ParamTypeNumber,
				Default: float64(0), Min: -1e10, Max: 1e10, Step: 0.01,
				Placeholder: "如: 15", Description: "大于或等于此值的股票通过",
				Required: true,
			}},
		},
		{
			Operator: model.OpLT,
			Symbol:   "<",
			Label:    "小于",
			Example:  "PE < 30",
			Params: []OperatorParamDef{{
				Key: "value_number", Label: "阈值", Type: model.ParamTypeNumber,
				Default: float64(100), Min: -1e10, Max: 1e10, Step: 0.01,
				Placeholder: "如: 30", Description: "小于此值的股票通过",
				Required: true,
			}},
		},
		{
			Operator: model.OpLTE,
			Symbol:   "<=",
			Label:    "小于等于",
			Example:  "负债率 <= 50%",
			Params: []OperatorParamDef{{
				Key: "value_number", Label: "阈值", Type: model.ParamTypeNumber,
				Default: float64(100), Min: -1e10, Max: 1e10, Step: 0.01,
				Placeholder: "如: 50", Description: "小于或等于此值的股票通过",
				Required: true,
			}},
		},
		{
			Operator: model.OpBetween,
			Symbol:   "[~]",
			Label:    "区间内",
			Example:  "PE 在 20~50 之间",
			Params: []OperatorParamDef{
				{
					Key: "min_value", Label: "下界", Type: model.ParamTypeRange,
					Default: float64(0), Min: -1e10, Max: 1e10, Step: 1,
					Placeholder: "如: 20", Description: "区间下限（含）",
					Required: true,
				},
				{
					Key: "max_value", Label: "上界", Type: model.ParamTypeRange,
					Default: float64(100), Min: -1e10, Max: 1e10, Step: 1,
					Placeholder: "如: 50", Description: "区间上限（含）",
					Required: true,
				},
			},
		},
		{
			Operator: model.OpNotBetween,
			Symbol:   ")~(",
			Label:    "区间外",
			Example:  "PE 不在 0~10 或 >100 (排除亏损股和异常高估)",
			Params: []OperatorParamDef{
				{
					Key: "min_value", Label: "下界", Type: model.ParamTypeRange,
					Default: float64(0), Min: -1e10, Max: 1e10, Step: 1,
					Placeholder: "如: 0", Description: "排除区间的下限",
					Required: true,
				},
				{
					Key: "max_value", Label: "上界", Type: model.ParamTypeRange,
					Default: float64(100), Min: -1e10, Max: 1e10, Step: 1,
					Placeholder: "如: 100", Description: "排除区间的上限",
					Required: true,
				},
			},
		},
	}
}

// BoolOps 布尔型操作符 (bool): 涨停等是/否判断
func BoolOps() []OperatorOption {
	return []OperatorOption{
		{
			Operator: model.OpEQ,
			Symbol:   "=",
			Label:    "等于",
			Example:  "涨停 = 是",
			Params:   []OperatorParamDef{},
		},
		{
			Operator: model.OpNEQ,
			Symbol:   "!=",
			Label:    "不等于",
			Example:  "涨停 = 否",
			Params:   []OperatorParamDef{},
		},
	}
}

// EnumOps 枚举型操作符 (enum): 板块, 行业等分类匹配
func EnumOps() []OperatorOption {
	return []OperatorOption{
		{
			Operator: model.OpIn,
			Symbol:   "∈",
			Label:    "属于",
			Example:  "板块 ∈ {创业板, 科创板}",
			Params: []OperatorParamDef{{
				Key: "value_list", Label: "选项", Type: model.ParamTypeMultiSelect,
				Default: []string{}, Placeholder: "选择选项...",
				Description: "属于其中任一即通过",
				Required: true,
			}},
		},
		{
			Operator: model.OpNotIn,
			Symbol:   "∉",
			Label:    "不属于",
			Example:  "板块 ∉ {ST, *ST}",
			Params: []OperatorParamDef{{
				Key: "value_list", Label: "排除项", Type: model.ParamTypeMultiSelect,
				Default: []string{}, Placeholder: "选择要排除的选项...",
				Description: "不属于其中任一即通过",
				Required: true,
			}},
		},
		{
			Operator: model.OpContains,
			Symbol:   "⊃",
			Label:    "包含",
			Example:  "行业名称包含 \"电子\"",
			Params: []OperatorParamDef{{
				Key: "value_list", Label: "关键词", Type: model.ParamTypeSelect,
				Default: "", Placeholder: "输入关键词...",
				Description: "名称包含该关键词即通过",
				Required: true,
			}},
		},
		{
			Operator: model.OpEQ,
			Symbol:   "=",
			Label:    "等于",
			Example:  "板块 = 创业板",
			Params: []OperatorParamDef{{
				Key: "value_list", Label: "选项", Type: model.ParamTypeSelect,
				Default: "", Placeholder: "选择...",
				Description: "完全匹配该值",
				Required: true,
			}},
		},
	}
}

// SeriesOps 序列型操作符 (series): 金叉/死叉, 背离, 突破等需历史数据的形态判断
func SeriesOps() []OperatorOption {
	return []OperatorOption{
		{
			Operator: model.OpCrossUp,
			Symbol:   "↑↑",
			Label:    "金叉/上穿",
			Example:  "MACD 近3天内出现金叉",
			Params: []OperatorParamDef{
				{
					Key: "within_days", Label: "时间窗口(天)", Type: model.ParamTypeDays,
					Default: 1, Min: 0, Max: 60, Step: 1, Unit: "天",
					Placeholder: "如: 3", Description: "在最近N天内出现金叉。0=仅看当前状态",
					Required: false,
				},
				{
					Key: "ago_from_days", Label: "起始偏移(天)", Type: model.ParamTypeDays,
					Default: 0, Min: 0, Max: 60, Step: 1, Unit: "天",
					Placeholder: "如: 0", Description: "从N天前开始看。配合时间窗口使用",
					Required: false,
				},
			},
		},
		{
			Operator: model.OpCrossDown,
			Symbol:   "↓↓",
			Label:    "死叉/下穿",
			Example:  "MACD 近3天内出现死叉",
			Params: []OperatorParamDef{
				{
					Key: "within_days", Label: "时间窗口(天)", Type: model.ParamTypeDays,
					Default: 1, Min: 0, Max: 60, Step: 1, Unit: "天",
					Placeholder: "如: 3", Description: "在最近N天内出现死叉",
					Required: false,
				},
				{
					Key: "ago_from_days", Label: "起始偏移(天)", Type: model.ParamTypeDays,
					Default: 0, Min: 0, Max: 60, Step: 1, Unit: "天",
					Placeholder: "如: 0",
					Required: false,
				},
			},
		},
		{
			Operator: model.OpDivergencePos,
			Symbol:   "↗",
			Label:    "底背离",
			Example:  "MACD 底背离（价格新低但MACD未新低）",
			Params: []OperatorParamDef{{
				Key: "lookback_days", Label: "回看天数", Type: model.ParamTypeDays,
				Default: 26, Min: 10, Max: 120, Step: 1, Unit: "天",
				Placeholder: "如: 26", Description: "用于判断背离的历史数据长度",
				Required: false,
			}},
		},
		{
			Operator: model.OpDivergenceNeg,
			Symbol:   "↘",
			Label:    "顶背离",
			Example:  "MACD 顶背离",
			Params: []OperatorParamDef{{
				Key: "lookback_days", Label: "回看天数", Type: model.ParamTypeDays,
				Default: 26, Min: 10, Max: 120, Step: 1, Unit: "天",
				Placeholder: "如: 26", Description: "用于判断背离的历史数据长度",
				Required: false,
			}},
		},
		{
			Operator: model.OpBreakout,
			Symbol:   "⤒",
			Label:    "向上突破",
			Example:  "股价突破布林带上轨",
			Params: []OperatorParamDef{{
				Key: "within_days", Label: "时间窗口(天)", Type: model.ParamTypeDays,
				Default: 1, Min: 0, Max: 20, Step: 1, Unit: "天",
				Placeholder: "如: 1",
				Required: false,
			}},
		},
		{
			Operator: model.OpBreakdown,
			Symbol:   "⤓",
			Label:    "向下跌破",
			Example:  "股价跌破布林带下轨",
			Params: []OperatorParamDef{{
				Key: "within_days", Label: "时间窗口(天)", Type: model.ParamTypeDays,
				Default: 1, Min: 0, Max: 20, Step: 1, Unit: "天",
				Placeholder: "如: 1",
				Required: false,
			}},
		},
	}
}
