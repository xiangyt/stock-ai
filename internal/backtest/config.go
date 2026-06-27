package backtest

// ============================================================================
//  SignalConfig / ParamDef — 信号配置与参数定义
// ============================================================================

// SignalConfig 单条信号的运行时配置（前后端交互载体）。
type SignalConfig struct {
	SignalID string         `json:"signal_id"` // 完整 8 位编码
	Operator string         `json:"operator,omitempty"` // 操作符
	Params   map[string]any `json:"params"`    // 信号参数 key-value
}

// ParamDef 信号参数定义（供前端渲染和 API 元数据）。
type ParamDef struct {
	Key      string       `json:"key"`               // 参数 key
	Label    string       `json:"label"`             // 前端显示标签
	Type     string       `json:"type"`              // 控件类型 "number" | "select" | "boolean"
	Required bool         `json:"required"`          // 是否必填
	Default  float64      `json:"default"`           // 默认值
	Min      float64      `json:"min,omitempty"`     // 最小允许值
	Max      float64      `json:"max,omitempty"`     // 最大允许值
	Step     float64      `json:"step,omitempty"`    // 步长
	Unit     string       `json:"unit,omitempty"`    // 单位（如 "%"、"天"、"亿"）
	Options  []EnumOption `json:"options,omitempty"`  // 枚举选项（Type="select" 时）
}

// EnumOption 枚举选项。
type EnumOption struct {
	Value string `json:"value"` // 选项值
	Label string `json:"label"` // 选项标签
}
