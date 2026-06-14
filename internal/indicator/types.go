package indicator

import (
	"fmt"
	"strconv"

	"stock-ai/internal/model"
)

// ============================================================================
//  SignalID 编码体系 — 8位数字自描述唯一标识 (动态生成)
//
//  格式: CCIIISSTT
//    CC  = 分类码 (2位): 01技术 02行情 03基本 04财务
//    III = 指标序号 (3位): 同分类下指标排序, 001~999
//    S  = 来源标志 (1位):   0=内置信号 1=自定义信号
//   TT  = 信号序号 (2位): 同一基础信号在不同指标下复用, 01~99
//
//  核心设计:
//    - Signal 只持有 2 位序号(TT)，完整 ID 动态拼接: indicatorID + S + TT
//    - 同一信号模板(如"大于")挂到不同指标 → 自动获得不同完整ID
//    - 例: PE的"大于"=04001001, ROE的"大于"=04002001 (TT同为"01")
// ============================================================================

// CategoryCode 分类码常量（SignalID 前两位 = 指标ID 前两位）
type CategoryCode string

const (
	CatCodeTechnical   CategoryCode = "01" // 技术面
	CatCodeMarket      CategoryCode = "02" // 行情面
	CatCodeFundamental CategoryCode = "03" // 基本面
	CatCodeFinancial   CategoryCode = "04" // 财务面
)

// CategoryToCode 将 Category 枚举映射为 2 位分类码
func CategoryToCode(cat Category) CategoryCode {
	switch cat {
	case CatTechnical:
		return CatCodeTechnical
	case CatMarket:
		return CatCodeMarket
	case CatFundamental:
		return CatCodeFundamental
	case CatFinancial:
		return CatCodeFinancial
	default:
		return "00"
	}
}

// ParseResult SignalID 解析结果
type ParseResult struct {
	Category  CategoryCode // 分类码
	IndSeq    string       // 指标序号 (3位)
	IsCustom  bool         // true=自定义, false=内置
	SignalSeq string       // 信号序号 (2位)
}

// ParseSignalID 解析8位数字SignalID
func ParseSignalID(id string) ParseResult {
	if len(id) < 8 {
		return ParseResult{}
	}
	return ParseResult{
		Category:  CategoryCode(id[0:2]),
		IndSeq:    id[2:5],
		IsCustom:  id[5] == '1',
		SignalSeq: id[6:8],
	}
}

// IndicatorID 从 SignalID 提取前5位作为指标唯一标识符 (CCIII = 分类码2位 + 指标序号3位)
func IndicatorID(signalID string) string {
	if len(signalID) >= 8 {
		return signalID[0:5]
	}
	return signalID
}

// SignalSeqFromFullID 从完整位ID提取2位信号序号
func SignalSeqFromFullID(id string) string {
	if len(id) >= 8 {
		return id[6:8]
	}
	return id
}

// BuildSignalID 动态构建完整8位SignalID
func BuildSignalID(indicatorID string, isCustom bool, seq string) string {
	source := "0"
	if isCustom {
		source = "1"
	}
	return indicatorID + source + seq
}

// ============================================================================
//  Category — 向后兼容的分类枚举
// ============================================================================

type Category string

const (
	CatTechnical   Category = "technical"
	CatMarket      Category = "market"
	CatFundamental Category = "fundamental"
	CatFinancial   Category = "financial"
)

// ============================================================================
//  ValueType — ★ 值类型是信号的属性，不是指标的属性
//
//  不同信号对同一指标数据可能有不同解释:
//  - MACD指标: 金叉信号是 series(需扫描), 柱状图值可能是 number
//  - 价格指标: 绝对值比较是 number, 涨停判断是 bool
// ============================================================================

type ValueType string

const (
	ValNumber ValueType = "number" // 数值型
	ValSeries ValueType = "series" // 序列型 (需扫描历史)
	ValBool   ValueType = "bool"   // 布尔型
	ValEnum   ValueType = "enum"   // 枚举型
	ValString ValueType = "string" // 字符串型
)

// ============================================================================
//  CompareOperator 比较操作符常量
// ============================================================================

type CompareOperator string

const (
	OpGT            CompareOperator = "gt"
	OpGTE           CompareOperator = "gte"
	OpLT            CompareOperator = "lt"
	OpLTE           CompareOperator = "lte"
	OpEQ            CompareOperator = "eq"
	OpNEQ           CompareOperator = "neq"
	OpBetween       CompareOperator = "between"
	OpNotBetween    CompareOperator = "not_between"
	OpIn            CompareOperator = "in"
	OpNotIn         CompareOperator = "not_in"
	OpContains      CompareOperator = "contains"
	OpCrossAbove    CompareOperator = "cross_above"
	OpCrossBelow    CompareOperator = "cross_below"
	OpDivergencePos CompareOperator = "divergence_pos"
	OpDivergenceNeg CompareOperator = "divergence_neg"
	OpRising        CompareOperator = "rising"
	OpFalling       CompareOperator = "falling"
	OpCustom        CompareOperator = "custom" // 自定义参数（如 lookback 窗口）
)

// OperatorMeta 操作符元数据
type OperatorMeta struct {
	Operator CompareOperator `json:"operator"`
	Label    string          `json:"label"`
	Desc     string          `json:"desc"`
}

// ParamDef 参数定义
type ParamDef struct {
	Key         string       `json:"key"`
	Label       string       `json:"label"`
	Type        string       `json:"type"` // number/range/days/select
	Required    bool         `json:"required"`
	Default     float64      `json:"default"`
	Min         float64      `json:"min,omitempty"`
	Max         float64      `json:"max,omitempty"`
	Step        float64      `json:"step,omitempty"`
	Unit        string       `json:"unit,omitempty"`
	Options     []EnumOption `json:"options,omitempty"`
	Placeholder string       `json:"placeholder,omitempty"`
}

// EnumOption 枚举选项
type EnumOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// OperatorOption 操作符选项
type OperatorOption struct {
	Operator CompareOperator `json:"operator"`
	Label    string          `json:"label"`
	Params   []ParamDef      `json:"params,omitempty"`
}

// ============================================================================
//
//	Signal — 信号一等公民 (纯元数据 + 操作符定义)
//
//	信号是选股系统的核心原子单位。每个信号拥有:
//	  - 独立身份: 仅 2 位序号, 完整 8 位 ID 由所属指标动态生成
//	  - 自有值类型: 不同信号对同一指标数据可有不同解释
//	  - 元数据 + 操作符定义: 供前端渲染
//	  - 默认配置: 预填参数模板
//
//	★ 评估逻辑由 Indicator 负责，Signal 不携带 EvalFunc。
//	   指标通过 BatchEvaluate/Evaluate 统一调度，
//	   根据配置中的 SignalSeq 分发给对应的评估逻辑。
//
//	复用模型:
//	  同一 Signal 定义(如 Seq="01" Name="大于")可挂载到不同指标,
//	  通过 FullID(indicatorID) 自动获得不同的完整 ID。
//
// ============================================================================
type Signal interface {
	Seq() string
	SignalType() string // 信号类型名称（固定，同类型信号共享；用于自定义信号去重）
	Name() string
	Alias() string      // 展示别名（非空时前端优先展示别名，hover 展示 Description）
	Description() string
	ValueType() ValueType
	Operators() []OperatorOption
	DefaultConfig() *SignalConfig
}

func NewBaseSignal(seq, name, desc string, valueType ValueType, operators []OperatorOption, defaultConfig *SignalConfig) BaseSignal {
	return BaseSignal{
		seq:           seq,
		NameStr:       name,
		Desc:          desc,
		valueType:     valueType,
		operators:     operators,
		defaultConfig: defaultConfig,
	}
}

type BaseSignal struct {
	seq           string           // 2位序号 "01"~"99"
	signalType    string           // 信号类型名称（固定，用于自定义信号去重；空则 fallback NameStr）
	NameStr       string           // 显示名
	Desc          string           // 描述
	AliasStr      string           // 展示别名（非空时前端展示别名，Name 仍作 fallback）
	valueType     ValueType        // ★ 值类型
	operators     []OperatorOption // 该信号支持的操作符
	defaultConfig *SignalConfig    // 默认配置(SignalID运行时填充)
}

func (s *BaseSignal) Seq() string {
	return s.seq
}

// SignalType 返回信号类型名称。如果未显式设置（空字符串），则 fallback 到 NameStr。
// 用于自定义信号去重：同一指标下，同 SignalType 的自定义信号至多一个。
func (s *BaseSignal) SignalType() string {
	if s.signalType != "" {
		return s.signalType
	}
	return s.NameStr
}

func (s *BaseSignal) Name() string {
	return s.NameStr
}

func (s *BaseSignal) Alias() string {
	return s.AliasStr
}

func (s *BaseSignal) Description() string {
	return s.Desc
}

func (s *BaseSignal) ValueType() ValueType {
	return s.valueType
}

func (s *BaseSignal) Operators() []OperatorOption {
	return s.operators
}

func (s *BaseSignal) DefaultConfig() *SignalConfig {
	return s.defaultConfig
}

// SetSignalType 设置信号类型名称并返回自身，用于链式调用。
// 仅当信号类型名与 NameStr 不同时才需显式调用。
func (s *BaseSignal) SetSignalType(t string) *BaseSignal {
	s.signalType = t
	return s
}

// SetAlias 设置展示别名并返回自身，用于链式调用。
func (s *BaseSignal) SetAlias(alias string) *BaseSignal {
	s.AliasStr = alias
	return s
}

// ============================================================================
//  参数键常量 — 所有 SignalConfig.Params 中使用的 key 必须引用这里的常量，
//  禁止在业务代码中出现裸字符串 key。
//
//  分组说明:
//    - 数值比较类 (gt/gte/lt/lte/eq/neq): ParamKeyThreshold
//    - 区间类 (between/not_between):       ParamKeyMin + ParamKeyMax
//    - 枚举多选 (in/not_in):               ParamKeyValues
//    - 序列交叉 (cross_above/cross_below): ParamKeyFastDays + ParamKeySlowDays
//    - 回看天数 (rising/falling 等):       ParamKeyDays
// ============================================================================

const (
	ParamKeyThreshold = "threshold" // 单阈值：gt/gte/lt/lte/eq/neq
	ParamKeyMin       = "min"       // 区间下限：between/not_between
	ParamKeyMax       = "max"       // 区间上限：between/not_between
	ParamKeyValues    = "values"    // 枚举值列表：in/not_in
	ParamKeyFastDays  = "fast_days" // 快线周期：cross_above/cross_below
	ParamKeySlowDays  = "slow_days" // 慢线周期：cross_above/cross_below
	ParamKeyDays      = "days"      // 通用回看天数：rising/falling 等

	ParamKeyLookbackStart = "lookback_start" // 窗口起点（更早的N天前），配合 lookback_end 使用
	ParamKeyLookbackEnd   = "lookback_end"   // 窗口终点（更近的N天前）
)

// ============================================================================
//  SignalConfig 运行时信号配置 — 前后端交互核心载体
// ============================================================================

type SignalConfig struct {
	SignalID string          `json:"signal_id"` // 完整8位 (仅运行时存在)
	Operator CompareOperator `json:"operator"`
	Params   map[string]any  `json:"params"`
}

// GetIndicatorID 返回所属指标的5位标识符 (SignalID[:5])
func (c *SignalConfig) GetIndicatorID() string { return IndicatorID(c.SignalID) }

// GetSignalSeq 返回2位信号序号 (SignalID[6:8])
func (c *SignalConfig) GetSignalSeq() string { return SignalSeqFromFullID(c.SignalID) }

// IsCustom 是否为自定义信号 (第6位 == '1')
func (c *SignalConfig) IsCustom() bool { return len(c.SignalID) >= 8 && c.SignalID[5] == '1' }

// --- 通用参数访问器 ---

func (c *SignalConfig) GetParam(key string, defaultVal any) any {
	if c.Params == nil {
		return defaultVal
	}
	if v, ok := c.Params[key]; ok {
		return v
	}
	return defaultVal
}

func (c *SignalConfig) GetFloat64(key string, defaultVal float64) float64 {
	v := c.GetParam(key, defaultVal)
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return defaultVal
	default:
		return defaultVal
	}
}

func (c *SignalConfig) GetInt(key string, defaultVal int) int {
	v := c.GetParam(key, defaultVal)
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case int64:
		return int(val)
	default:
		return defaultVal
	}
}

func (c *SignalConfig) GetString(key string, defaultVal string) string {
	v := c.GetParam(key, defaultVal)
	if s, ok := v.(string); ok {
		return s
	}
	return defaultVal
}

// ============================================================================
//  操作符专用取值方法
//
//  每种 CompareOperator 对应固定的参数键集合，通过语义化方法名消除业务代码中的
//  裸字符串 key（如 GetFloat64("threshold", 0) → Threshold()）。
//
//  分组:
//    数值比较类 (OpGT/OpGTE/OpLT/OpLTE/OpEQ/OpNEQ): Threshold()
//    区间类     (OpBetween/OpNotBetween):             RangeMin() + RangeMax()
//    枚举多选   (OpIn/OpNotIn):                       Values()
//    序列交叉   (OpCrossAbove/OpCrossBelow):           FastDays() + SlowDays()
//    通用回看   (OpRising/OpFalling 等):               Days()
// ============================================================================

// Threshold 返回单阈值参数（适用于 OpGT/OpGTE/OpLT/OpLTE/OpEQ/OpNEQ）。
func (c *SignalConfig) Threshold() float64 {
	return c.GetFloat64(ParamKeyThreshold, 0)
}

// RangeMin 返回区间下限（适用于 OpBetween/OpNotBetween）。
func (c *SignalConfig) RangeMin() float64 {
	return c.GetFloat64(ParamKeyMin, 0)
}

// RangeMax 返回区间上限（适用于 OpBetween/OpNotBetween）。
func (c *SignalConfig) RangeMax() float64 {
	return c.GetFloat64(ParamKeyMax, 0)
}

// Values 返回枚举值列表（适用于 OpIn/OpNotIn）。
// 若参数不存在或类型不匹配则返回空切片。
func (c *SignalConfig) Values() []string {
	v := c.GetParam(ParamKeyValues, nil)
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// FastDays 返回快线周期天数（适用于 OpCrossAbove/OpCrossBelow），默认 12。
func (c *SignalConfig) FastDays() int {
	return c.GetInt(ParamKeyFastDays, 12)
}

// SlowDays 返回慢线周期天数（适用于 OpCrossAbove/OpCrossBelow），默认 26。
func (c *SignalConfig) SlowDays() int {
	return c.GetInt(ParamKeySlowDays, 26)
}

// Days 返回通用回看天数（适用于 OpRising/OpFalling 等），默认 5。
func (c *SignalConfig) Days() int {
	return c.GetInt(ParamKeyDays, 5)
}

// ============================================================================
//  Indicator 接口 — 所有指标必须实现
//
//  指标的核心职责:
//    1. 提供数据 (从 StockSource 提取/计算指标特有数据)
//    2. 管理信号 (内置 + 自定义两组)
//    3. 调度评估 (Evaluate 将公共计算结果分发给各信号)
// ============================================================================

type Indicator interface {
	// 基本信息
	ID() string
	Name() string
	Category() Category
	Description() string
	Unit() string

	// ★ 信号管理 (分离为内置和自定义)
	BuiltInSignals() []Signal // 内置信号 (由指标开发者定义)
	CustomSignals() []Signal  // 自定义信号 (用户/LLM 运行时添加)
	AllSignals() []Signal     // 合并视图 = BuiltIn + Custom

	// 核心评估方法 — 指标负责所有信号的评估调度
	Evaluate(stock StockSource, configs []*SignalConfig) *EvaluatedStock
}

// ============================================================================
//  BaseIndicator — 嵌入结构体，减少样板代码
// ============================================================================

type BaseIndicator struct {
	Seq         string // 3位指标序号 (如 "001")
	NameStr     string
	CategoryVal Category
	Desc        string
	UnitStr     string
	builtInSigs []Signal
	customSigs  []Signal
	Signal      map[string]Signal
}

// --- 基本信息 ---

// ID 动态生成6位指标ID = CategoryCode(2位) + Seq(3位)
func (b *BaseIndicator) ID() string          { return string(CategoryToCode(b.CategoryVal)) + b.Seq }
func (b *BaseIndicator) Name() string        { return b.NameStr }
func (b *BaseIndicator) Category() Category  { return b.CategoryVal }
func (b *BaseIndicator) Description() string { return b.Desc }
func (b *BaseIndicator) Unit() string        { return b.UnitStr }

// --- 信号管理 ---

func (b *BaseIndicator) BuiltInSignals() []Signal { return b.builtInSigs }
func (b *BaseIndicator) CustomSignals() []Signal  { return b.customSigs }
func (b *BaseIndicator) AllSignals() []Signal {
	return append(b.builtInSigs, b.customSigs...)
}

// --- Setter ---

// registerSignals 将信号列表注册到 Signal 映射表，并赋值给目标切片字段。
// 内部公共方法，供 SetBuiltInSignals / SetCustomSignals 复用，
// 消除两处完全相同的 for+map 赋值逻辑。
func (b *BaseIndicator) registerSignals(sigs []Signal, target *[]Signal, sourceCode string) {
	if b.Signal == nil {
		b.Signal = make(map[string]Signal, len(sigs))
	}

	for _, sig := range sigs {
		b.Signal[b.ID()+sourceCode+sig.Seq()] = sig
	}
	*target = sigs
}

// SetBuiltInSignals 注册内置信号到映射表
func (b *BaseIndicator) SetBuiltInSignals(sigs []Signal) {
	b.registerSignals(sigs, &b.builtInSigs, "0")
}

// SetCustomSignals 注册自定义信号到映射表。
// 约束：同一 SignalType 的自定义信号至多一个实例（内置信号不做此约束）。
func (b *BaseIndicator) SetCustomSignals(sigs []Signal) {
	// 去重检查：同一 SignalType 至多出现一次
	seen := make(map[string]bool, len(sigs))
	for _, sig := range sigs {
		typ := sig.SignalType()
		if seen[typ] {
			panic(fmt.Sprintf("自定义信号类型 %q 在指标 %q 中重复", typ, b.ID()))
		}
		seen[typ] = true
	}
	b.registerSignals(sigs, &b.customSigs, "1")
}

// BatchEvaluate 默认实现:
//
//	逐个调用 Indicator.Evaluate，由子类根据 config 中的信号序号分发到具体评估逻辑。
//	需要优化的指标（如 NumberFieldIndicator）应覆写此方法以实现批量计算。
func (b *BaseIndicator) Evaluate(stock StockSource, configs []*SignalConfig) *EvaluatedStock {
	return &EvaluatedStock{
		Code:    stock.GetCode(),
		Name:    stock.GetName(),
		Result:  ResultRejected,
		Message: fmt.Sprintf("%s 未实现 Evaluate 方法", b.Name()),
	}
}

// ============================================================================
//  分类子接口 — 按指标分类拆分数据源
//
//  每个子接口只包含对应分类所需的方法。
//  实现类可以选择性实现，不必实现所有方法。
// ============================================================================

// TechnicalSource 技术面数据源（01xxxx）
// 提供 K 线序列数据，用于 MACD、均线等技术指标计算。
type TechnicalSource interface {
	GetDailyKline() ([]*model.DailyKline, error)
	GetWeeklyKline() ([]*model.WeeklyKline, error)
	GetMonthlyKline() ([]*model.MonthlyKline, error)
	GetYearlyKline() ([]*model.YearlyKline, error)
}

// MarketSource 行情面数据源（02xxxx）
// 提供实时行情数据，用于价格、涨跌幅、市值等指标。
type MarketSource interface {
	GetDailyKline() ([]*model.DailyKline, error)
	GetDailySnapshot() (*model.StockDailySnapshot, error)
}

// FundamentalSource 基本面数据源（03xxxx）
// 提供公司基本信息，用于上市板块、行业分类等枚举型指标。
type FundamentalSource interface {
	GetDetail() (*model.Stock, error)
}

// FinancialSource 财务面数据源（04xxxx）
// 提供财报、股本等财务数据，用于 PE、PB、ROE 等财务指标。
type FinancialSource interface {
	GetDailySnapshot() (*model.StockDailySnapshot, error)
	GetPerformanceReport() ([]*model.PerformanceReport, error)
	GetShareholderCount() (*model.ShareholderCount, error)
}

// ============================================================================
//
//	StockSource — 总接口（组合 4 个子接口）
//
//	各指标 Evaluate 只依赖自身分类的子接口，但引擎层需要统一处理
//	（取 Code/Name/Price 等），因此需要总接口作为 Engine.Execute 的入参类型。
//
// ============================================================================
type StockSource interface {
	// 公共方法 — 所有指标和引擎都可能访问
	GetCode() string
	GetName() string

	TechnicalSource
	MarketSource
	FundamentalSource
	FinancialSource
}

// ============================================================================
//  API 元数据输出
// ============================================================================

type APIMeta struct {
	Categories  []CategoryMeta          `json:"categories"`
	Indicators  []IndicatorMeta         `json:"indicators"`
	EnumOptions map[string][]EnumOption `json:"enum_options"`
}

type CategoryMeta struct {
	ID   Category `json:"id"`
	Name string   `json:"name"`
	Desc string   `json:"desc"`
}

// IndicatorMeta 单个指标的 API 元数据
type IndicatorMeta struct {
	ID          string       `json:"id"` // 6位指标ID
	Name        string       `json:"name"`
	Category    Category     `json:"category"`
	Description string       `json:"description"`
	Unit        string       `json:"unit"`    // 数据单位 (指标级)
	Signals     []SignalMeta `json:"signals"` // 信号列表 (每个含完整ID、ValueType、操作符)
}

// SignalMeta 信号的 API 元数据 (完整8位ID 在此生成)
type SignalMeta struct {
	SignalID      string           `json:"signal_id"` // 完整8位
	Name          string           `json:"name"`
	Alias         string           `json:"alias,omitempty"`       // 展示别名（非空时前端优先展示，hover 展示 description）
	Description   string           `json:"description,omitempty"`
	ValueType     ValueType        `json:"value_type"` // ★ 信号的值类型
	Operators     []OperatorOption `json:"operators"`
	DefaultConfig *SignalConfig    `json:"default_config,omitempty"`
}

// buildSignalMetas 将一组 Signal 转换为 SignalMeta 列表（动态拼接完整 8 位 ID）。
// 内部公共方法，供 ToAPIMeta 复用，消除 BuiltIn/Custom 两段完全相同的循环。
//
// 参数:
//   - indicatorID: 6 位指标 ID (如 "040001")
//   - sourceCode: 来源标志 "0"=内置 / "1"=自定义
//   - signals: 待转换的 Signal 切片
func buildSignalMetas(indicatorID string, sourceCode string, signals []Signal) []SignalMeta {
	sigMetas := make([]SignalMeta, 0, len(signals))
	for _, sig := range signals {
		fullID := indicatorID + sourceCode + sig.Seq()

		// 克隆 DefaultConfig 并填入完整 SignalID
		var cfgCopy *SignalConfig
		if sig.DefaultConfig() != nil {
			cp := *sig.DefaultConfig()
			cp.SignalID = fullID
			cfgCopy = &cp
		}

		sigMetas = append(sigMetas, SignalMeta{
			SignalID:      fullID,
			Name:          sig.Name(),
			Alias:         sig.Alias(),
			Description:   sig.Description(),
			ValueType:     sig.ValueType(),
			Operators:     sig.Operators(),
			DefaultConfig: cfgCopy,
		})
	}
	return sigMetas
}

// ToAPIMeta 将 Indicator 转换为 API 输出元数据
func ToAPIMeta(ind Indicator) IndicatorMeta {
	indID := ind.ID()

	// 合并内置 + 自定义信号的元数据 (动态生成完整8位ID)
	sigMetas := make([]SignalMeta, 0)
	sigMetas = append(sigMetas, buildSignalMetas(indID, "0", ind.BuiltInSignals())...)
	sigMetas = append(sigMetas, buildSignalMetas(indID, "1", ind.CustomSignals())...)

	return IndicatorMeta{
		ID:          indID,
		Name:        ind.Name(),
		Category:    ind.Category(),
		Description: ind.Description(),
		Unit:        ind.Unit(),
		Signals:     sigMetas,
	}
}

// ============================================================================
//  操作符显示映射表
// ============================================================================

var operatorLabels = map[CompareOperator]string{
	OpGT:            "大于",
	OpGTE:           "大于等于",
	OpLT:            "小于",
	OpLTE:           "小于等于",
	OpEQ:            "等于",
	OpNEQ:           "不等于",
	OpBetween:       "区间内",
	OpNotBetween:    "区间外",
	OpIn:            "属于",
	OpNotIn:         "不属于",
	OpContains:      "包含",
	OpCrossAbove:    "上穿",
	OpCrossBelow:    "下穿",
	OpDivergencePos: "顶背离",
	OpDivergenceNeg: "底背离",
	OpRising:        "多头排列",
	OpFalling:       "空头排列",
	OpCustom:        "自定义参数",
}

var operatorDescs = map[CompareOperator]string{
	OpGT:            "实际值 > 目标值",
	OpGTE:           "实际值 >= 目标值",
	OpLT:            "实际值 < 目标值",
	OpLTE:           "实际值 <= 目标值",
	OpEQ:            "实际值 == 目标值",
	OpNEQ:           "实际值 != 目标值",
	OpBetween:       "目标最小值 <= 实际值 <= 目标最大值",
	OpNotBetween:    "实际值 < 目标最小值 或 实际值 > 目标最大值",
	OpIn:            "实际值在指定枚举列表中",
	OpNotIn:         "实际值不在指定枚举列表中",
	OpContains:      "目标字符串包含在实际值中",
	OpCrossAbove:    "快线从下方穿越慢线（金叉）",
	OpCrossBelow:    "快线从上方穿越慢线（死叉）",
	OpDivergencePos: "价格创新高但指标未创新高（顶背离）",
	OpDivergenceNeg: "价格创新低但指标未创新低（底背离）",
	OpRising:        "短期均线 > 中期均线 > 长期均线",
	OpFalling:       "短期均线 < 中期均线 < 长期均线",
	OpCustom:        "按自定义参数条件判断（如时间窗口）",
}
