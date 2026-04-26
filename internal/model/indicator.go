package model

import (
	"fmt"
	"time"
)

// ============================================================================
//  选股指标体系 — 三层架构
//
//  Layer 1: IndicatorCategory (指标大类) → 技术面 / 行情面 / 基本面 / 财务面
//  Layer 2: Indicator          (指标)     → MACD / PE / 换手率 / ...
//  Layer 3: Signal             (信号)     → MACD金叉 / PE<20 / 放量突破 / ...
//
//  策略 (Strategy) = 有序的 Signal 列表，取交集过滤
// ============================================================================

// ---------- Layer 1: 指标大类 ----------

// IndicatorCategory 指标大类枚举
type IndicatorCategory string

const (
	CategoryTechnical  IndicatorCategory = "technical"   // 技术面
	CategoryMarket     IndicatorCategory = "market"      // 行情面
	CategoryFundamental IndicatorCategory = "fundamental" // 基本面
	CategoryFinancial  IndicatorCategory = "financial"    // 财务面
	CategorySentiment  IndicatorCategory = "sentiment"    // 情绪面（预留）
)

// CategoryInfo 大类元信息（用于前端展示和注册表）
var CategoryMeta = map[IndicatorCategory]struct {
	Name        string // 中文名
	Description string // 说明
	Order       int    // 排序
}{
	CategoryTechnical:  {"技术面", "基于价格、成交量计算的技术分析指标", 1},
	CategoryMarket:     {"行情面", "基于当日行情数据的市场表现指标", 2},
	CategoryFundamental: {"基本面", "基于公司属性的基本面特征", 3},
	CategoryFinancial:  {"财务面", "基于财报数据的财务健康度指标", 4},
	CategorySentiment:  {"情绪面", "基于市场情绪的资金流向与热度指标", 5},
}

func (c IndicatorCategory) DisplayName() string {
	if info, ok := CategoryMeta[c]; ok {
		return info.Name
	}
	return string(c)
}

func (c IndicatorCategory) Description() string {
	if info, ok := CategoryMeta[c]; ok {
		return info.Description
	}
	return ""
}

// ---------- Layer 2: 指标 ----------

// ValueType 指标值的数据类型
type ValueType string

const (
	ValueTypeNumber  ValueType = "number"  // 数值型：PE, MA, RSI 等
	ValueTypeBool    ValueType = "bool"    // 布尔型：金叉/死叉, 突破等形态信号
	ValueTypeEnum    ValueType = "enum"    // 枚举型：板块, 行业, 交易所
	ValueTypeSeries  ValueType = "series"  // 序列型：需要多日数据判断的趋势/背离
)

// DataSourceType 指标数据来源（决定从哪张表/哪个字段取数）
type DataSourceType string

const (
	DataSourceKline      DataSourceType = "kline"         // K线数据 (daily_kline 等)
	DataSourceStockPrice DataSourceType = "stock_price"   // stock_prices 预计算字段
	DataSourceStockInfo  DataSourceType = "stock_info"    // stocks_detail 基本信息
	DataSourceFinancial  DataSourceType = "financial"     // performance_reports 财报
	DataSourceShareholder DataSourceType = "shareholder"  // shareholder_counts
	DataSourceRealtime   DataSourceType = "realtime"      // 实时行情
)

// Indicator 定义（注册到指标库中的元数据）
// 注意：这是「指标类」的定义，不是某只股票的具体值
type Indicator struct {
	ID          string           `json:"id"`                     // 唯一标识: "macd", "pe_ttm"
	Category    IndicatorCategory `json:"category"`               // 所属大类
	Name        string           `json:"name"`                   // 中文名: "MACD"
	NameEn      string           `json:"name_en"`                // 英文名: "MACD"
	Description string          `json:"description"`            // 描述: "指数平滑异同移动平均线"
	ValueType   ValueType        `json:"value_type"`              // 值类型
	DataSource  DataSourceType   `json:"data_source"`             // 数据来源
	Fields      []string         `json:"fields"`                  // 依赖的数据库字段: ["macd", "macd_signal", "macd_hist"]
	Unit        string           `json:"unit"`                    // 单位: "%", "倍", "" (空=无单位)
	Version     int              `json:"version"`                 // 版本号（支持指标定义迭代）
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// ---------- Layer 3: 信号 ----------

// CompareOperator 比较操作符
type CompareOperator string

const (
	OpGT       CompareOperator = "gt"        // 大于 >
	OpGTE      CompareOperator = "gte"       // 大于等于 >=
	OpLT       CompareOperator = "lt"        // 小于 <
	OpLTE      CompareOperator = "lte"       // 小于等于 <=
	OpEQ       CompareOperator = "eq"        // 等于 ==
	OpNEQ      CompareOperator = "neq"       // 不等于 !=
	OpBetween  CompareOperator = "between"   // 区间 [low, high]
	OpNotBetween CompareOperator = "not_between" // 区间外
	OpIn       CompareOperator = "in"        // 枚举集合内
	OpNotIn    CompareOperator = "not_in"    // 枚举集合外
	OpContains CompareOperator = "contains"  // 包含（字符串/列表）
	OpCrossUp  CompareOperator = "cross_up"  // 上穿/金叉（序列信号）
	OpCrossDown CompareOperator = "cross_down" // 下穿/死叉（序列信号）
	OpDivergencePos CompareOperator = "divergence_pos" // 底背离（正背离）
	OpDivergenceNeg CompareOperator = "divergence_neg" // 顶背离（负背离）
	OpBreakout  CompareOperator = "breakout"  // 突破
	OpBreakdown CompareOperator = "breakdown" // 跌破
)

// OperatorMeta 操作符元信息
var OperatorMeta = map[CompareOperator]struct {
	Symbol    string // 显示符号: ">", "<=", "金叉"
	Label     string // 中文标签: "大于", "大于等于", "金叉"
	Supported []ValueType // 支持的值类型
}{
	OpGT:       {">",  "大于", []ValueType{ValueTypeNumber}},
	OpGTE:      {">=", "大于等于", []ValueType{ValueTypeNumber}},
	OpLT:       {"<",  "小于", []ValueType{ValueTypeNumber}},
	OpLTE:      {"<=", "小于等于", []ValueType{ValueTypeNumber}},
	OpEQ:       {"=",  "等于", []ValueType{ValueTypeNumber, ValueTypeEnum}},
	OpNEQ:      {"!=", "不等于", []ValueType{ValueTypeNumber, ValueTypeEnum}},
	OpBetween:  {"[]", "区间内", []ValueType{ValueTypeNumber}},
	OpNotBetween: {")(", "区间外", []ValueType{ValueTypeNumber}},
	OpIn:       {"∈", "属于", []ValueType{ValueTypeEnum}},
	OpNotIn:    {"∉", "不属于", []ValueType{ValueTypeEnum}},
	OpContains: {"∋", "包含", []ValueType{ValueTypeEnum, ValueTypeBool}},
	OpCrossUp:   {"↑↑", "金叉/上穿", []ValueType{ValueTypeBool, ValueTypeSeries}},
	OpCrossDown: {"↓↓", "死叉/下穿", []ValueType{ValueTypeBool, ValueTypeSeries}},
	OpDivergencePos: {"↗", "底背离", []ValueType{ValueTypeBool, ValueTypeSeries}},
	OpDivergenceNeg: {"↘", "顶背离", []ValueType{ValueTypeBool, ValueTypeSeries}},
	OpBreakout:  {"⤒", "向上突破", []ValueType{ValueTypeBool, ValueTypeSeries}},
	OpBreakdown: {"⤓", "向下跌破", []ValueType{ValueTypeBool, ValueTypeSeries}},
}

// Signal 信号定义 — 一个具体的选股条件（支持参数化配置）
//
// 设计理念：
//   - 每个 Signal 是一个「参数化模板」，默认值由 PresetSignal 提供
//   - 前端页面 / LLM 可以通过可变字段覆盖默认值，实现动态筛选
//   - 例如："MACD 3天内金叉" vs "MACD 1天内金叉" 是同一个 Signal 模板的不同参数实例
//
// 参数覆盖优先级：显式设置值 > DefaultCfg 工厂默认值
type Signal struct {
	ID          string           `json:"id"`                     // 信号唯一标识
	IndicatorID string           `json:"indicator_id"`            // 关联的指标 ID
	Name        string           `json:"name"`                   // 信号显示名（可被前端/LLM覆写）
	Description string          `json:"description"`            // 信号描述
	Category    IndicatorCategory `json:"category,omitempty"`     // 所属大类（冗余存储，方便前端分组展示）
	Operator    CompareOperator  `json:"operator"`                // 操作符

	// ---- 固定参数（单值比较） ----
	ValueNumber float64 `json:"value_number,omitempty"`  // 单值: PE>20 中的 20
	ValueList   []string `json:"value_list,omitempty"`    // 枚举列表: 板块 in ["chinext","star"]

	// ---- 区间参数（Between/NotBetween 使用） ----
	// 优先使用 MinValue/MaxValue；向后兼容 ValueLow/ValueHigh
	ValueLow  float64 `json:"value_low,omitempty"`  // 区间下界(兼容旧版)
	ValueHigh float64 `json:"value_high,omitempty"` // 区间上界(兼容旧版)
	MinValue  float64 `json:"min_value,omitempty"`  // 区间下界（推荐新用）
	MaxValue  float64 `json:"max_value,omitempty"`  // 区间上界（推荐新用）

	// ---- 时间窗口参数（序列信号专用：Cross/Divergence/Breakout） ----
	// 这组参数解决了 "1天前金叉" vs "1-3天内金叉" 的动态需求：
	//
	//   WithinDays=1, AgoFromDays=0  → 仅今天是否发生
	//   WithinDays=1, AgoFromDays=1  → 昨天是否发生（"1天前"）
	//   WithinDays=3, AgoFromDays=0  → 最近3天内是否发生（"近3天"）
	//   WithinDays=3, AgoFromDays=1  → 1~3天前之间是否发生
	//   WithinDays=0                 → 任意历史时间（仅看 LookbackDays 范围内的形态判断）
	//
	WithinDays int `json:"within_days,omitempty"` // 在N天内发生（含起始日），0=不限制窗口/仅看形态
	AgoFromDays int `json:"ago_from_days,omitempty"` // 从N天前开始看，0=从今天开始

	// ---- 回看深度（形态判断用的历史数据长度） ----
	LookbackDays int `json:"lookback_days,omitempty"` // 回看天数(用于背离等需长序列的判断)，默认因指标而异

	// ---- 自定义阈值 ----
	Threshold float64 `json:"threshold,omitempty"` // 自定义阈值（超买线/超卖线/squeeze比率等），0=用指标默认

	// ---- 元信息 ----
	Weight  float32 `json:"weight,omitempty"` // 权重 0~1 (默认1)
	Order   int     `json:"order"`            // 执行顺序（滤网层序）
	Enabled bool    `json:"enabled"`          // 是否启用

	// ---- 扩展参数字典（特殊场景兜底，LLM 可自由传入键值对） ----
	// 前端不需要渲染的字段、或未来扩展的实验性参数都可以放这里
	Params map[string]interface{} `json:"params,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetEffectiveMin 获取实际生效的区间下值（优先 MinValue，fallback 到 ValueLow）
func (s *Signal) GetEffectiveMin() float64 {
	if s.MinValue != 0 {
		return s.MinValue
	}
	return s.ValueLow
}

// GetEffectiveMax 获取实际生效的区间上值（优先 MaxValue，fallback 到 ValueHigh）
func (s *Signal) GetEffectiveMax() float64 {
	if s.MaxValue != 0 {
		return s.MaxValue
	}
	return s.ValueHigh
}

// GetWithinRange 获取时间窗口 [agoFrom, agoFrom+within) 的起止偏移量（天数）
// 返回 (startOffset, endOffset), 都是从今天(0)往前的偏移
// 用于评估函数切片 historical 数据
func (s *Signal) GetWithinRange() (startOffset, endOffset int) {
	if s.WithinDays <= 0 && s.AgoFromDays <= 0 {
		// 未设置窗口 → 返回全范围，由调用方根据 LookbackDays 自行决定
		return 0, 0
	}
	startOffset = s.AgoFromDays
	endOffset = s.AgoFromDays + s.WithinDays
	if endOffset == 0 {
		endOffset = 1 // 至少看1天
	}
	return
}

// HumanReadableDescription 生成人类可读的信号描述（含动态参数值）
// 例如: "MACD在近3日内出现金叉", "市盈率在25~50倍之间"
func (s *Signal) HumanReadableDescription() string {
	base := s.Name
	switch s.Operator {
	case OpBetween:
		return fmt.Sprintf("%s在%.1f~%.1f%s之间", base, s.GetEffectiveMin(), s.GetEffectiveMax(), s.paramUnitLabel())
	case OpGT:
		return fmt.Sprintf("%s>%.1f%s", base, s.ValueNumber, s.paramUnitLabel())
	case OpGTE:
		return fmt.Sprintf("%s≥%.1f%s", base, s.ValueNumber, s.paramUnitLabel())
	case OpLT:
		return fmt.Sprintf("%s<%.1f%s", base, s.ValueNumber, s.paramUnitLabel())
	case OpLTE:
		return fmt.Sprintf("%s≤%.1f%s", base, s.ValueNumber, s.paramUnitLabel())
	case OpCrossUp:
		if s.WithinDays > 0 {
			return fmt.Sprintf("%s在近%d日内出现金叉", base, s.WithinDays)
		}
		return base + "金叉"
	case OpCrossDown:
		if s.WithinDays > 0 {
			return fmt.Sprintf("%s在近%d日内出现死叉", base, s.WithinDays)
		}
		return base + "死叉"
	default:
		return base
	}
}

// paramUnitLabel 返回参数的单位标签（用于人类可读描述）
// TODO: 可从关联的 Indicator 获取默认单位，或使用 Params["unit"]
func (s *Signal) paramUnitLabel() string { return "" }

// ============================================================================
//  SignalParamDef — 信号参数元信息定义
// ============================================================================
//
// 用途：
//   1. 前端自动渲染表单控件（知道每个信号需要什么参数、什么类型、取值范围）
//   2. LLM 理解可接受哪些参数及其约束（生成合法的 Signal JSON）
//   3. 参数校验（运行时检查用户输入是否合法）
//
// ParamType 可选值：
//   - "number"    : 数值输入框（单值）       → 对应 ValueNumber
//   - "range"     : 双滑块区间选择           → 对应 MinValue / MaxValue
//   - "days"      : 天数输入（时间窗口）       → 对应 WithinDays / AgoFromDays
//   - "select"    : 下拉选择                 → 对应 ValueList（预定义选项之一）
//   - "multi_select": 多选                   → 对应 ValueList
//   - "threshold" : 阈值滑块                 → 对应 Threshold
//   - "boolean"   : 开关                     → 对应 IsRequired 或 Params 中的 bool
// ============================================================================

// ParamType 参数类型枚举
type ParamType string

const (
	ParamTypeNumber      ParamType = "number"       // 单值数值
	ParamTypeRange       ParamType = "range"        // 区间 [min, max]
	ParamTypeDays        ParamType = "days"         // 天数（时间窗口）
	ParamTypeSelect      ParamType = "select"       // 单选下拉
	ParamTypeMultiSelect ParamType = "multi_select" // 多选
	ParamTypeThreshold   ParamType = "threshold"    // 阈值
	ParamTypeBoolean     ParamType = "boolean"      // 开关
)

// SignalParamDef 单个参数的元信息定义
type SignalParamDef struct {
	Key         string        `json:"key"`                    // JSON 字段名: "within_days", "min_value", "value_number"
	Label       string        `json:"label"`                  // 中文显示名: "时间窗口(天)", "区间下界"
	Type        ParamType     `json:"type"`                   // 参数类型

	// ---- 默认值与范围约束 ----
	Default     interface{}   `json:"default"`                // 默认值 (float64 | int | string | bool)
	Min         float64       `json:"min,omitempty"`          // 最小值 (number/range/threshold/days)
	Max         float64       `json:"max,omitempty"`          // 最大值
	Step        float64       `json:"step,omitempty"`         // 步长 (slider 的步进)

	// ---- 选项（select / multi_select 类型使用） ----
	Options     []ParamOption `json:"options,omitempty"`      // 预设选项列表

	// ---- 校验与提示 ----
	Required    bool          `json:"required"`               // 是否必填
	Description string        `json:"description"`            // 参数说明：用户看到的功能描述
	Placeholder string        `json:"placeholder,omitempty"`  // 输入框 placeholder
	Example     string        `json:"example,omitempty"`      // 示例值和说明: "3 表示近3天内出现"
	Unit        string        `json:"unit,omitempty"`         // 单位后缀: "天", "倍", "%", "元"

	// ---- 高级控制 ----
	Hidden      bool          `json:"hidden,omitempty"`       // 是否隐藏（前端不展示，但 LLM 可设置）
	Group       string        `json:"group,omitempty"`       // 分组名: "time_window", "value_range", "threshold"
	DependsOn   string        `json:"depends_on,omitempty"`   // 联动字段: 仅当某操作符时才显示此参数
	ConditionValue string     `json:"condition_value,omitempty"` // 联动条件的值
}

// ParamOption 下拉选项
type ParamOption struct {
	Value string `json:"value"` // 选项值
	Label string `json:"label"` // 显示文本
	Desc  string `json:"desc,omitempty"` // 选项补充说明
}

// EvaluateParam 信号评估时的运行时参数（从 DB 中查出的原始数据）
type EvaluateParam struct {
	StockCode                               string
	Current                                 map[string]float64 // 当日/最新值 {字段名: 值}
	Historical                              []map[string]float64 // 历史序列（按时间倒序，[0]=最新）
	FundamentalLatest                       map[string]float64 // 最新财报值
	StockInfo                               map[string]string  // 基本信息（行业、板块等）
}

// EvaluateResult 单个信号的评估结果
type EvaluateResult struct {
	SignalID    string  `json:"signal_id"`
	Pass        bool    `json:"pass"`                  // 是否通过
	RawValue    float64 `json:"raw_value,omitempty"`    // 实际值（用于展示为何通过/未通过）
	Detail      string  `json:"detail,omitempty"`       // 详情说明: "MACD=0.12 > 0 ✓"
	CostNanos   int64   `json:"cost_nanos"`             // 评估耗时(ns)
	Error       error   `json:"error,omitempty"`        // 错误（数据缺失等）
}

// ---------- 策略（Strategy）— 固化后的选股策略 ----------

// StrategyStatus 策略状态
type StrategyStatus string

const (
	StrategyDraft     StrategyStatus = "draft"     // 草稿
	StrategyActive    StrategyStatus = "active"    // 启用
	StrategyArchived  StrategyStatus = "archived"  // 归档
)

// StrategyLogicalOp 多信号间的逻辑关系
type LogicalOp string

const (
	LogicalAND LogicalOp = "and" // 取交集（所有条件都满足）
	LogicalOR  LogicalOp = "or"  // 取并集（任一条件满足即可）
)

// Strategy 选股策略 — 一组有序信号的组合
type Strategy struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	UserID      uint           `gorm:"index" json:"user_id"`
	Name        string         `gorm:"size:100;not null" json:"name"`                    // 策略名称
	Description string         `gorm:"size:500" json:"description"`                     // 描述
	Status      StrategyStatus `gorm:"size:20;default:draft" json:"status"`             // 状态
	LogicalOp   LogicalOp      `gorm:"size:10;default:and" json:"logical_op"`           // 条件间逻辑关系

	// 信号条件（JSON 存储 Signal 数组）
	Conditions  string         `gorm:"type:text;not null" json:"conditions_raw"`         // JSON: []Signal

	// 回测相关
	BacktestConfig string      `gorm:"type:text" json:"backtest_config,omitempty"`      // 回测参数(JSON)
	BacktestResult string      `gorm:"type:text" json:"backtest_result,omitempty"`      // 回测结果摘要(JSON)

	// 统计
	RunCount    int           `gorm:"default:0" json:"run_count"`                      // 运行次数
	LastRunAt   *time.Time    `json:"last_run_at"`                                    // 最后运行时间
	AvgResultCount int        `gorm:"default:0" json:"avg_result_count"`               // 平均命中数

	// 元数据
	Tags        string         `gorm:"size:200" json:"tags,omitempty"`                 // 标签, 逗号分隔
	IsPublic    bool           `gorm:"default:false" json:"is_public"`                 // 是否公开
	StarCount   int            `gorm:"default:0" json:"star_count"`                    // 收藏数

	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func (Strategy) TableName() string { return "strategies" }

// ParseConditions 解析 Conditions JSON 为 Signal 切片
func (s *Strategy) ParseConditions() ([]Signal, error) {
	// TODO: 实现 JSON 反序列化
	return nil, nil
}

// ---------- 运行时：筛选上下文与结果 ----------

// ScreenContext 筛选执行上下文
type ScreenContext struct {
	TradeDate    string     // 筛选基准日期 YYYY-MM-DD
	Market       string     // 市场: A股/港股/美股
	Limit        int        // 最大返回数量, 0=不限制
	Mode  string `json:"mode,omitempty"` // 策略模式（保留兼容，运行时不再使用）
	Debug bool   `json:"debug,omitempty"` // 是否输出调试信息
}

// ScreenResult 筛选最终结果
type ScreenResult struct {
	StrategyID    uint              `json:"strategy_id,omitempty"`
	StrategyName  string            `json:"strategy_name,omitempty"`
	TradeDate     string            `json:"trade_date"`
	TotalScanned  int               `json:"total_scanned"`   // 全市场扫描数量
	TotalPassed   int               `json:"total_passed"`    // 通过所有条件的数量
	Candidates    []CandidateStock  `json:"candidates"`      // 候选股列表
	DurationMs    int64             `json:"duration_ms"`     // 总耗时
	SignalStats   map[string]int    `json:"signal_stats"`     // 各信号的通过/未通过统计
	Error         error             `json:"error,omitempty"`
}

// CandidateStock 候选股（旧版，保留兼容）
type CandidateStock struct {
	Code            string            `json:"code"`
	Name            string            `json:"name"`
	Exchange        string            `json:"exchange"`
	Industry        string            `json:"industry"`
	Price           float64           `json:"price"`
	ChangePct       float64           `json:"change_pct"`
	MatchedSignals  []string          `json:"matched_signals"`         // 匹配到的信号ID列表
	FailedSignals   []string          `json:"failed_signals,omitempty"` // 未通过的 required 信号
	SignalDetails   map[string]string `json:"signal_details"`          // 各信号的详情 {signal_id: "MACD=0.12>0 ✓"}
	Extra           map[string]any    `json:"extra,omitempty"`          // 扩展信息
}

// ============================================================================
//  StockData — 单只股票的完整数据快照（Engine.Execute 的输入）
//
//  由调用方（service 层）在调用 Execute 前组装完成。
//  screener 包不再关心数据来源，只做纯逻辑评估。
// ============================================================================

type StockData struct {
	Code       string                   `json:"code"`                  // 股票代码 (000001.SZ)
	Name       string                   `json:"name"`                  // 名称
	Exchange   string                   `json:"exchange,omitempty"`    // 交易所
	Industry   string                   `json:"industry,omitempty"`    // 行业

	// 当日/最新行情快照
	Current map[string]float64 `json:"current"` // {close, ma5, macd, pe_ttm, volume...}

	// 历史序列（按时间倒序，[0]=最近一天）
	Historical []map[string]float64 `json:"historical,omitempty"`

	// 最新财报数据
	Financial map[string]float64 `json:"financial,omitempty"` // {roe_w, eps, debt_ratio...}

	// 基本信息（字符串型字段）
	Info map[string]string `json:"info,omitempty"` // {listing_board, list_date, sector...}
}

// ============================================================================
//  EvaluatedStock — 单只股票的评估结果（新版 ScreenResult 的元素）
// ============================================================================

type EvaluatedStock struct {
	Code           string            `json:"code"`
	Name           string            `json:"name"`
	Price          float64           `json:"price"`
	ChangePct      float64           `json:"change_pct"`
	MatchedSignals []string          `json:"matched_signals"`
	FailedSignals  []string          `json:"failed_signals,omitempty"`
	SignalDetails  map[string]string `json:"signal_details"`
	Reason         string            `json:"reason,omitempty"` // 归类原因（可选）
}

// ============================================================================
//  ScreenResult — 筛选最终结果：三态返回
//
//  所有传入的股票被分为三组：
//    Passed:   通过所有 required 条件
//    Rejected: 至少一个 required 未通过
//    Pending:  数据不足/错误导致无法判断
// ============================================================================

type ScreenResultV2 struct {
	TotalScanned int              `json:"total_scanned"` // 输入的股票总数
	Passed       []EvaluatedStock `json:"passed"`        // 通过
	Rejected     []EvaluatedStock `json:"rejected"`      // 驳回
	Pending      []EvaluatedStock `json:"pending"`       // 待定
	DurationMs   int64            `json:"duration_ms"`   // 耗时
	SignalStats  map[string]int   `json:"signal_stats"`  // 各信号通过统计
}
