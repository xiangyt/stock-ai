/**
 * 从 Go 后端 screener/registry.go + operators/operators.go 提取的 Mock 数据
 * 
 * 数据结构对应：
 *   model.Indicator → Indicator
 *   screener.PresetSignal → PresetSignal（★ 新增中间层：信号模板）
 *   operators.OperatorOption → OperatorOption
 *   screener.IndicatorWithOperators → IndicatorWithOps
 */

// ========== 枚举类型 ==========

export enum ValueType {
  Number = 'number',
  Bool = 'bool',
  Enum = 'enum',
  Series = 'series',
}

export enum Category {
  Technical = 'technical',     // 技术面
  Market = 'market',           // 行情面
  Fundamental = 'fundamental', // 基本面
  Financial = 'financial',     // 财务面
}

export enum CompareOperator {
  GT = '>', GTE = '>=', LT = '<', LTE = '<=',
  EQ = '=', NEQ = '!=',
  Between = 'between', NotBetween = 'not_between',
  In = 'in', NotIn = 'not_in', Contains = 'contains',
  CrossUp = 'cross_up', CrossDown = 'cross_down',
  DivergencePos = 'divergence_pos', DivergenceNeg = 'divergence_neg',
  Breakout = 'breakout', Breakdown = 'breakdown',
}

// ========== 类型定义 ==========

export interface ParamDef {
  key: string
  label: string
  type: string          // number | range | days | select | multiSelect | threshold
  default: any
  min?: number
  max?: number
  step?: number
  unit?: string
  placeholder?: string
  description?: string
  required: boolean
  group?: string
  conditionValue?: string
}

export interface OperatorOption {
  operator: CompareOperator
  symbol: string
  label: string
  example: string
  params: ParamDef[]
}

export interface Indicator {
  id: string
  category: Category
  name: string
  nameEn: string
  description: string
  valueType: ValueType
  dataSource: string
  fields: string[]
  unit: string
  version: number
}

/** ★ 预设信号模板 — 指标与操作符之间的中间层 */
export interface PresetSignal {
  id: string
  name: string
  category: Category
  indicatorID: string        // 关联指标
  defaultOperator: CompareOperator  // 默认操作符（用户可覆盖）
  defaultParams: Record<string, any> // 默认参数值
  paramDefs: ParamDef[]      // 参数元信息（驱动表单渲染）
  description: string
}

export interface IndicatorWithPresets extends Indicator {
  presets: PresetSignal[]    // 该指标下可用的信号模板列表
  operators: OperatorOption[] // 该指标支持的所有原始操作符（自定义模式用）
}

// ========== 操作符工厂（对应 Go 的 operators 包）==========

const seriesOps: OperatorOption[] = [
  { operator: CompareOperator.CrossUp, symbol: '↑↑', label: '金叉/上穿', example: 'MACD 近3天内出现金叉', params: [
    { key: 'within_days', label: '时间窗口(天)', type: 'days', default: 1, min: 0, max: 60, step: 1, unit: '天', placeholder: '如: 3', description: '在最近N天内出现金叉。0=仅看当前状态', required: false },
    { key: 'ago_from_days', label: '起始偏移(天)', type: 'days', default: 0, min: 0, max: 60, step: 1, unit: '天', placeholder: '如: 0', description: '从N天前开始看。配合时间窗口使用', required: false },
  ]},
  { operator: CompareOperator.CrossDown, symbol: '↓↓', label: '死叉/下穿', example: 'MACD 近3天内出现死叉', params: [
    { key: 'within_days', label: '时间窗口(天)', type: 'days', default: 1, min: 0, max: 60, step: 1, unit: '天', placeholder: '如: 3', description: '在最近N天内出现死叉', required: false },
    { key: 'ago_from_days', label: '起始偏移(天)', type: 'days', default: 0, min: 0, max: 60, step: 1, unit: '天', placeholder: '如: 0', required: false },
  ]},
  { operator: CompareOperator.DivergencePos, symbol: '↗', label: '底背离', example: 'MACD 底背离（价格新低但MACD未新低）', params: [
    { key: 'lookback_days', label: '回看天数', type: 'days', default: 26, min: 10, max: 120, step: 1, unit: '天', placeholder: '如: 26', description: '用于判断背离的历史数据长度', required: false },
  ]},
  { operator: CompareOperator.DivergenceNeg, symbol: '↘', label: '顶背离', example: 'MACD 顶背离', params: [
    { key: 'lookback_days', label: '回看天数', type: 'days', default: 26, min: 10, max: 120, step: 1, unit: '天', placeholder: '如: 26', required: false },
  ]},
  { operator: CompareOperator.Breakout, symbol: '⤒', label: '向上突破', example: '股价突破布林带上轨', params: [
    { key: 'within_days', label: '时间窗口(天)', type: 'days', default: 1, min: 0, max: 20, step: 1, unit: '天', placeholder: '如: 1', required: false },
  ]},
  { operator: CompareOperator.Breakdown, symbol: '⤓', label: '向下跌破', example: '股价跌破布林带下轨', params: [
    { key: 'within_days', label: '时间窗口(天)', type: 'days', default: 1, min: 0, max: 20, step: 1, unit: '天', placeholder: '如: 1', required: false },
  ]},
]

const numberOps: OperatorOption[] = [
  { operator: CompareOperator.GT, symbol: '>', label: '大于', example: 'PE > 20', params: [{ key: 'value_number', label: '阈值', type: 'number', default: 0, min: -1e10, max: 1e10, step: 0.01, placeholder: '如: 20', description: '大于此值的股票通过', required: true }] },
  { operator: CompareOperator.GTE, symbol: '>=', label: '大于等于', example: 'ROE >= 15%', params: [{ key: 'value_number', label: '阈值', type: 'number', default: 0, min: -1e10, max: 1e10, step: 0.01, placeholder: '如: 15', description: '大于或等于此值的股票通过', required: true }] },
  { operator: CompareOperator.LT, symbol: '<', label: '小于', example: 'PE < 30', params: [{ key: 'value_number', label: '阈值', type: 'number', default: 100, min: -1e10, max: 1e10, step: 0.01, placeholder: '如: 30', description: '小于此值的股票通过', required: true }] },
  { operator: CompareOperator.LTE, symbol: '<=', label: '小于等于', example: '负债率 <= 50%', params: [{ key: 'value_number', label: '阈值', type: 'number', default: 100, min: -1e10, max: 1e10, step: 0.01, placeholder: '如: 50', description: '小于或等于此值的股票通过', required: true }] },
  { operator: CompareOperator.Between, symbol: '[~]', label: '区间内', example: 'PE 在 20~50 之间', params: [
    { key: 'min_value', label: '下界', type: 'range', default: 0, min: -1e10, max: 1e10, step: 1, placeholder: '如: 20', description: '区间下限（含）', required: true },
    { key: 'max_value', label: '上界', type: 'range', default: 100, min: -1e10, max: 1e10, step: 1, placeholder: '如: 50', description: '区间上限（含）', required: true },
  ]},
  { operator: CompareOperator.NotBetween, signal: ')~(', label: '区间外', example: 'PE 不在 0~10 或 >100', params: [
    { key: 'min_value', label: '下界', type: 'range', default: 0, min: -1e10, max: 1e10, step: 1, placeholder: '如: 0', description: '排除区间的下限', required: true },
    { key: 'max_value', label: '上界', type: 'range', default: 100, min: -1e10, max: 1e10, step: 1, placeholder: '如: 100', description: '排除区间的上限', required: true },
  ]},
]

const boolOps: OperatorOption[] = [
  { operator: CompareOperator.EQ, symbol: '=', label: '等于', example: '涨停 = 是', params: [] },
  { operator: CompareOperator.NEQ, symbol: '!=', label: '不等于', example: '涨停 = 否', params: [] },
]

const enumOps: OperatorOption[] = [
  { operator: CompareOperator.In, symbol: '∈', label: '属于', example: '板块 ∈ {创业板, 科创板}', params: [{ key: 'value_list', label: '选项', type: 'multiSelect', default: [], placeholder: '选择选项...', description: '属于其中任一即通过', required: true }] },
  { operator: CompareOperator.NotIn, symbol: '∉', label: '不属于', example: '板块 ∉ {ST, *ST}', params: [{ key: 'value_list', label: '排除项', type: 'multiSelect', default: [], placeholder: '选择要排除的选项...', description: '不属于其中任一即通过', required: true }] },
  { operator: CompareOperator.Contains, symbol: '⊃', label: '包含', example: '行业名称包含 "电子"', params: [{ key: 'value_list', label: '关键词', type: 'select', default: '', placeholder: '输入关键词...', description: '名称包含该关键词即通过', required: true }] },
  { operator: CompareOperator.EQ, symbol: '=', label: '等于', example: '板块 = 创业板', params: [{ key: 'value_list', label: '选项', type: 'select', default: '', placeholder: '选择...', description: '完全匹配该值', required: true }] },
]

function defaultOpsForType(valueType: ValueType): OperatorOption[] {
  switch (valueType) {
    case ValueType.Number: return numberOps
    case ValueType.Bool: return boolOps
    case ValueType.Enum: return enumOps
    case ValueType.Series: return seriesOps
    default: return []
  }
}

// ========== 指标定义（43个，从 Go BuiltInIndicators 提取）==========

const rawIndicators: Indicator[] = [
  /* 技术面-趋势 */
  { id: 'ma5', category: Category.Technical, name: 'MA5', nameEn: 'MA5', description: '5日移动平均线，短期趋势参考', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['ma5'], unit: '', version: 1 },
  { id: 'ma10', category: Category.Technical, name: 'MA10', nameEn: 'MA10', description: '10日移动平均线', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['ma10'], unit: '', version: 1 },
  { id: 'ma20', category: Category.Technical, name: 'MA20', nameEn: 'MA20', description: '20日移动平均线，生命线/操作线', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['ma20'], unit: '', version: 1 },
  { id: 'ma60', category: Category.Technical, name: 'MA60', nameEn: 'MA60', description: '60日移动平均线，决策线/季线', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['ma60'], unit: '', version: 1 },
  { id: 'ma_cross', category: Category.Technical, name: '均线金叉/死叉', nameEn: 'MA Cross', description: '短期均线上穿/下穿长期均线', valueType: ValueType.Series, dataSource: 'stock_price', fields: ['ma5','ma10','ma20'], unit: '', version: 1 },

  /* 技术面-MACD */
  { id: 'macd', category: Category.Technical, name: 'MACD(DIF值)', nameEn: 'MACD', description: '指数平滑异同移动平均线DIF值', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['macd'], unit: '', version: 1 },
  { id: 'macd_signal', category: Category.Technical, name: 'MACD信号线(DEA)', nameEn: 'MACD Signal', description: 'MACD的信号线DEA', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['macd_signal'], unit: '', version: 1 },
  { id: 'macd_hist', category: Category.Technical, name: 'MACD柱状图', nameEn: 'MACD Histogram', description: 'DIF-DEA柱状图，正=红柱(多头), 负=绿柱(空头)', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['macd_hist'], unit: '', version: 1 },
  { id: 'macd_cross', category: Category.Technical, name: 'MACD交叉', nameEn: 'MACD Cross', description: 'MACD DIF线上穿/下穿 DEA 线', valueType: ValueType.Series, dataSource: 'stock_price', fields: ['macd','macd_signal','macd_hist'], unit: '', version: 1 },
  { id: 'macd_divergence', category: Category.Technical, name: 'MACD背离', nameEn: 'MACD Divergence', description: '价格与MACD的背离关系', valueType: ValueType.Series, dataSource: 'kline', fields: ['close','macd','macd_hist'], unit: '', version: 1 },

  /* 技术面-KDJ */
  { id: 'kdj_k', category: Category.Technical, name: 'KDJ-K值', nameEn: 'KDJ-K', description: '随机指标K值, >80超买 <20超卖', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['kdj_k'], unit: '', version: 1 },
  { id: 'kdj_d', category: Category.Technical, name: 'KDJ-D值', nameEn: 'KDJ-D', description: '随机指标D值', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['kdj_d'], unit: '', version: 1 },
  { id: 'kdj_j', category: Category.Technical, name: 'KDJ-J值', nameEn: 'KDJ-J', description: 'J值更敏感', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['kdj_j'], unit: '', version: 1 },
  { id: 'kdj_cross', category: Category.Technical, name: 'KDJ交叉', nameEn: 'KDJ Cross', description: 'K线上穿/下穿D线', valueType: ValueType.Series, dataSource: 'stock_price', fields: ['kdj_k','kdj_d','kdj_j'], unit: '', version: 1 },

  /* 技术面-RSI/BOLL */
  { id: 'rsi6', category: Category.Technical, name: 'RSI6', nameEn: 'RSI6', description: '相对强弱指数(6日), >70超买 <30超卖', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['rsi6'], unit: '', version: 1 },
  { id: 'rsi12', category: Category.Technical, name: 'RSI12', nameEn: 'RSI12', description: '相对强弱指数(12日)', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['rsi12'], unit: '', version: 1 },
  { id: 'boll', category: Category.Technical, name: 'BOLL布林带', nameEn: 'Bollinger Bands', description: '布林带轨道(上轨/中轨/下轨)', valueType: ValueType.Series, dataSource: 'stock_price', fields: ['boll_upper','boll_mid','boll_lower','close'], unit: '', version: 1 },
  { id: 'boll_squeeze', category: Category.Technical, name: 'BOLL收口/开口', nameEn: 'BOLL Squeeze', description: '收口预示变盘, 开口预示趋势加速', valueType: ValueType.Series, dataSource: 'stock_price', fields: ['boll_upper','boll_lower'], unit: '', version: 1 },

  /* 行情面 */
  { id: 'price', category: Category.Market, name: '收盘价', nameEn: 'Close Price', description: '最新收盘价', valueType: ValueType.Number, dataSource: 'kline', fields: ['close'], unit: '元', version: 1 },
  { id: 'change_pct', category: Category.Market, name: '涨跌幅', nameEn: 'Change %', description: '当日涨跌幅(%)', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['change_pct'], unit: '%', version: 1 },
  { id: 'amplitude', category: Category.Market, name: '振幅', nameEn: 'Amplitude', description: '当日振幅(%)', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['amplitude'], unit: '%', version: 1 },
  { id: 'turnover_rate', category: Category.Market, name: '换手率', nameEn: 'Turnover Rate', description: '当日换手率(%)', valueType: ValueType.Number, dataSource: 'kline', fields: ['turnover_rate'], unit: '%', version: 1 },
  { id: 'volume', category: Category.Market, name: '成交量', nameEn: 'Volume', description: '当日成交量(手)', valueType: ValueType.Number, dataSource: 'kline', fields: ['volume'], unit: '手', version: 1 },
  { id: 'volume_ratio', category: Category.Market, name: '量比', nameEn: 'Volume Ratio', description: '成交量/5日均量, >1.5放量', valueType: ValueType.Number, dataSource: 'kline', fields: ['volume'], unit: '', version: 1 },
  { id: 'limit_up', category: Category.Market, name: '涨停', nameEn: 'Limit Up', description: '当日是否涨停', valueType: ValueType.Bool, dataSource: 'stock_price', fields: ['change_pct'], unit: '', version: 1 },
  { id: 'total_market_cap', category: Category.Market, name: '总市值', nameEn: 'Total MCap', description: '总市值(亿元)', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['total_market_cap'], unit: '亿', version: 1 },
  { id: 'circulate_market_cap', category: Category.Market, name: '流通市值', nameEn: 'Circulating MCap', description: '流通市值(亿元)', valueType: ValueType.Number, dataSource: 'stock_price', fields: ['circulate_market_cap'], unit: '亿', version: 1 },

  /* 基本面 */
  { id: 'listing_board', category: Category.Fundamental, name: '上市板块', nameEn: 'Listing Board', description: '主板/创业板/科创板/北交所', valueType: ValueType.Enum, dataSource: 'stock_info', fields: ['listing_board'], unit: '', version: 1 },
  { id: 'industry', category: Category.Fundamental, name: '所属行业', nameEn: 'Industry', description: '申万一级行业分类', valueType: ValueType.Enum, dataSource: 'stock_info', fields: ['industry'], unit: '', version: 1 },
  { id: 'list_date', category: Category.Fundamental, name: '上市时间', nameEn: 'List Date', description: '上市日期', valueType: ValueType.Number, dataSource: 'stock_info', fields: ['list_date'], unit: '天', version: 1 },

  /* 财务面-盈利 */
  { id: 'pe_ttm', category: Category.Financial, name: '市盈率(TTM)', nameEn: 'PE TTM', description: '滚动市盈率, <0亏损', valueType: ValueType.Number, dataSource: 'realtime', fields: [], unit: '倍', version: 1 },
  { id: 'roe_w', category: Category.Financial, name: 'ROE(加权)', nameEn: 'ROE', description: '净资产收益率(%)', valueType: ValueType.Number, dataSource: 'financial', fields: ['roe_w'], unit: '%', version: 1 },
  { id: 'roa', category: Category.Financial, name: '总资产收益率', nameEn: 'ROA', description: '总资产收益率(%)', valueType: ValueType.Number, dataSource: 'financial', fields: ['roa'], unit: '%', version: 1 },
  { id: 'gross_margin', category: Category.Financial, name: '毛利率', nameEn: 'Gross Margin', description: '销售毛利率(%)', valueType: ValueType.Number, dataSource: 'financial', fields: ['gross_margin'], unit: '%', version: 1 },
  { id: 'net_margin', category: Category.Financial, name: '净利率', nameEn: 'Net Margin', description: '销售净利率(%)', valueType: ValueType.Number, dataSource: 'financial', fields: ['net_margin'], unit: '%', version: 1 },

  /* 财务面-成长 */
  { id: 'revenue_yoy', category: Category.Financial, name: '营收同比', nameEn: 'Revenue YoY', description: '营业总收入同比增长(%)', valueType: ValueType.Number, dataSource: 'financial', fields: ['revenue_yoy'], unit: '%', version: 1 },
  { id: 'profit_yoy', category: Category.Financial, name: '净利润同比', nameEn: 'Profit YoY', description: '归母净利润同比增长(%)', valueType: ValueType.Number, dataSource: 'financial', fields: ['parent_net_profit_yoy'], unit: '%', version: 1 },

  /* 财务面-偿债 */
  { id: 'debt_ratio', category: Category.Financial, name: '资产负债率', nameEn: 'Debt Ratio', description: '资产负债率(%), <60%较安全', valueType: ValueType.Number, dataSource: 'financial', fields: ['debt_ratio'], unit: '%', version: 1 },
  { id: 'current_ratio', category: Category.Financial, name: '流动比率', nameEn: 'Current Ratio', description: '流动比率(倍), >1.5较安全', valueType: ValueType.Number, dataSource: 'financial', fields: ['current_ratio'], unit: '倍', version: 1 },
  { id: 'quick_ratio', category: Category.Financial, name: '速动比率', nameEn: 'Quick Ratio', description: '速动比率(倍)', valueType: ValueType.Number, dataSource: 'financial', fields: ['quick_ratio'], unit: '倍', version: 1 },

  /* 财务面-每股 */
  { id: 'eps', category: Category.Financial, name: '每股收益(EPS)', nameEn: 'EPS', description: '基本每股收益(元), >0盈利', valueType: ValueType.Number, dataSource: 'financial', fields: ['basic_eps'], unit: '元', version: 1 },
  { id: 'bvps', category: Category.Financial, name: '每股净资产(BPS)', nameEn: 'BVPS', description: '每股净资产(元)', valueType: ValueType.Number, dataSource: 'financial', fields: ['bvps'], unit: '元', version: 1 },
  { id: 'ocfps', category: Category.Financial, name: '每股经营现金流', nameEn: 'OCFPS', description: '每股经营现金流(元)', valueType: ValueType.Number, dataSource: 'financial', fields: ['ocfps'], unit: '元', version: 1 },
]

// ★★★ 预设信号模板（PresetSignals）— 从 Go registry.go 提取 ★★★
// 这是"三级选择"中的关键中间层：指标 → 信号模板 → 操作符

const rawPresets: PresetSignal[] = [
  // ===== MACD =====
  { id: 'macd_golden_cross', name: 'MACD金叉', category: Category.Technical, indicatorID: 'macd_cross',
    defaultOperator: CompareOperator.CrossUp, defaultParams: { within_days: 1, ago_from_days: 0 },
    paramDefs: [
      { key: 'within_days', label: '时间窗口(天)', type: 'days', default: 1, min: 0, max: 60, step: 1, unit: '天', placeholder: '1', description: '在最近N天内出现金叉。0=不限制窗口', required: false, group: 'time_window' },
      { key: 'ago_from_days', label: '起始偏移(天)', type: 'days', default: 0, min: 0, max: 60, step: 1, unit: '天', placeholder: '0', description: '从N天前开始看', required: false, group: 'time_window' },
    ], description: 'DIF线上穿DEA线，看多信号' },
  { id: 'macd_death_cross', name: 'MACD死叉', category: Category.Technical, indicatorID: 'macd_cross',
    defaultOperator: CompareOperator.CrossDown, defaultParams: { within_days: 1, ago_from_days: 0 },
    paramDefs: [
      { key: 'within_days', label: '时间窗口(天)', type: 'days', default: 1, min: 0, max: 60, step: 1, unit: '天', placeholder: '1', description: '在最近N天内出现死叉', required: false, group: 'time_window' },
      { key: 'ago_from_days', label: '起始偏移(天)', type: 'days', default: 0, min: 0, max: 60, step: 1, unit: '天', placeholder: '0', required: false, group: 'time_window' },
    ], description: 'DIF线下穿DEA线，看空信号' },
  { id: 'macd_bull_divergence', name: 'MACD底背离', category: Category.Technical, indicatorID: 'macd_divergence',
    defaultOperator: CompareOperator.DivergencePos, defaultParams: { lookback_days: 26 },
    paramDefs: [
      { key: 'lookback_days', label: '回看天数', type: 'days', default: 26, min: 10, max: 120, step: 1, unit: '天', placeholder: '26', description: '判断背离的历史数据长度', required: false, group: 'depth' },
    ], description: '价格创新低但MACD未创新低，潜在反转' },
  { id: 'macd_hist_positive', name: 'MACD红柱', category: Category.Technical, indicatorID: 'macd_hist',
    defaultOperator: CompareOperator.GT, defaultParams: { value_number: 0 },
    paramDefs: [
      { key: 'value_number', label: '阈值', type: 'number', default: 0, min: -1, max: 1, step: 0.01, placeholder: '0', description: 'MACD柱状图>此值为红柱', required: false, group: 'value_range' },
    ], description: '柱状图由负转正(红柱)，多头动能增强' },

  // ===== KDJ =====
  { id: 'kdj_golden_cross', name: 'KDJ金叉', category: Category.Technical, indicatorID: 'kdj_cross',
    defaultOperator: CompareOperator.CrossUp, defaultParams: { within_days: 1 },
    paramDefs: [
      { key: 'within_days', label: '时间窗口(天)', type: 'days', default: 1, min: 0, max: 30, step: 1, unit: '天', placeholder: '1', description: '近N天内KDJ金叉', required: false },
    ], description: 'K线上穿D线' },
  { id: 'kdj_oversold', name: 'KDJ超卖(J<20)', category: Category.Technical, indicatorID: 'kdj_j',
    defaultOperator: CompareOperator.LT, defaultParams: { value_number: 20 },
    paramDefs: [
      { key: 'value_number', label: '超卖线', type: 'threshold', default: 20, min: 0, max: 50, step: 1, placeholder: '20', description: 'J值低于此视为超卖。经典: 20', required: false, group: 'threshold' },
    ], description: 'KDJ J值进入超卖区域(<20)' },

  // ===== RSI =====
  { id: 'rsi_oversold', name: 'RSI6超卖(<30)', category: Category.Technical, indicatorID: 'rsi6',
    defaultOperator: CompareOperator.LT, defaultParams: { value_number: 30 },
    paramDefs: [
      { key: 'value_number', label: '超卖线', type: 'threshold', default: 30, min: 0, max: 50, step: 1, placeholder: '30', description: 'RSI低于此为超卖。经典: 30', required: false, group: 'threshold' },
    ], description: 'RSI6进入超卖区域(<30)' },
  { id: 'rsi_overbought', name: 'RSI6超买(>70)', category: Category.Technical, indicatorID: 'rsi6',
    defaultOperator: CompareOperator.GT, defaultParams: { value_number: 70 },
    paramDefs: [
      { key: 'value_number', label: '超买线', type: 'threshold', default: 70, min: 50, max: 100, step: 1, placeholder: '70', description: 'RSI高于此为超买。经典: 70', required: false, group: 'threshold' },
    ], description: 'RSI6进入超买区域(>70)' },

  // ===== 均线 =====
  { id: 'ma_bullish_arrange', name: '均线多头排列', category: Category.Technical, indicatorID: 'ma_cross',
    defaultOperator: CompareOperator.CrossUp, defaultParams: { within_days: 3 },
    paramDefs: [
      { key: 'within_days', label: '持续天数', type: 'days', default: 3, min: 1, max: 20, step: 1, unit: '天', placeholder: '3', description: '连续N天保持多头排列(MA5>MA10>MA20)', required: false },
    ], description: '短中长期均线依次向上排列，强势格局' },
  { id: 'price_above_ma20', name: '股价站上MA20', category: Category.Technical, indicatorID: 'ma20',
    defaultOperator: CompareOperator.GT, defaultParams: { value_number: 0 },
    paramDefs: [], description: '收盘价站在20日均线上方，中期趋势向好' },

  // ===== BOLL =====
  { id: 'boll_breakout_upper', name: 'BOLL突破上轨', category: Category.Technical, indicatorID: 'boll',
    defaultOperator: CompareOperator.Breakout, defaultParams: { within_days: 1 },
    paramDefs: [
      { key: 'within_days', label: '时间窗口(天)', type: 'days', default: 1, min: 0, max: 20, step: 1, unit: '天', placeholder: '1', required: false },
    ], description: '收盘价突破布林带上轨，强势特征' },

  // ===== 行情面 =====
  { id: 'market_limit_up', name: '涨停', category: Category.Market, indicatorID: 'limit_up',
    defaultOperator: CompareOperator.GTE, defaultParams: { value_number: 9.5 },
    paramDefs: [
      { key: 'value_number', label: '涨幅阈值(%)', type: 'number', default: 9.5, min: 5, max: 20, step: 0.1, placeholder: '9.5', description: '涨跌幅≥此值触发。9.5≈涨停', required: false, group: 'value_range' },
    ], description: '当日涨停或接近涨停(≥9.5%)' },
  { id: 'market_high_volume', name: '放量', category: Category.Market, indicatorID: 'volume_ratio',
    defaultOperator: CompareOperator.GT, defaultParams: { value_number: 1.5 },
    paramDefs: [
      { key: 'value_number', label: '量比阈值', type: 'threshold', default: 1.5, min: 0.8, max: 5, step: 0.1, placeholder: '1.5', description: '量比>1.5明显放量，>3巨量', required: false, group: 'threshold' },
    ], description: '成交量超过5日均值1.5倍以上' },
  { id: 'market_high_turnover', name: '高换手', category: Category.Market, indicatorID: 'turnover_rate',
    defaultOperator: CompareOperator.GT, defaultParams: { value_number: 5 },
    paramDefs: [
      { key: 'value_number', label: '换手率(%)', type: 'number', default: 5, min: 1, max: 50, step: 0.5, placeholder: '5', required: false, group: 'value_range' },
    ], description: '换手率超过5%，交投活跃' },
  { id: 'market_low_price', name: '低价股', category: Category.Market, indicatorID: 'price',
    defaultOperator: CompareOperator.LTE, defaultParams: { value_number: 10 },
    paramDefs: [
      { key: 'value_number', label: '价格上限(元)', type: 'number', default: 10, min: 1, max: 100, step: 0.5, placeholder: '10', required: false, group: 'value_range' },
    ], description: '收盘价不超过指定金额' },
  { id: 'market_small_cap', name: '小市值', category: Category.Market, indicatorID: 'circulate_market_cap',
    defaultOperator: CompareOperator.LT, defaultParams: { value_number: 50 },
    paramDefs: [
      { key: 'value_number', label: '市值上限(亿)', type: 'number', default: 50, min: 5, max: 500, step: 5, placeholder: '50', required: false, group: 'value_range' },
    ], description: '流通市值小于指定值，弹性较好' },

  // ===== 基本面 =====
  { id: 'fund_chinext_or_star', name: '创业板/科创板', category: Category.Fundamental, indicatorID: 'listing_board',
    defaultOperator: CompareOperator.In, defaultParams: { value_list: ['chinext', 'star'] },
    paramDefs: [
      { key: 'value_list', label: '板块', type: 'multiSelect', default: [], placeholder: '选择板块...', required: true },
    ], description: '上市板块为创业板或科创板' },

  // ===== 财务面 =====
  { id: 'fin_pe_reasonable', name: 'PE区间', category: Category.Financial, indicatorID: 'pe_ttm',
    defaultOperator: CompareOperator.Between, defaultParams: { min_value: 0, max_value: 30 },
    paramDefs: [
      { key: 'min_value', label: 'PE下界(倍)', type: 'range', default: 0, min: -50, max: 200, step: 1, placeholder: '0', description: '负数表示亏损股', required: false, group: 'value_range' },
      { key: 'max_value', label: 'PE上界(倍)', type: 'range', default: 30, min: 0, max: 500, step: 1, placeholder: '30', required: false, group: 'value_range' },
    ], description: 'TTM市盈率在指定范围内' },
  { id: 'fin_high_roe', name: '高ROE', category: Category.Financial, indicatorID: 'roe_w',
    defaultOperator: CompareOperator.GTE, defaultParams: { value_number: 15 },
    paramDefs: [
      { key: 'value_number', label: 'ROE下界(%)', type: 'threshold', default: 15, min: 0, max: 50, step: 1, placeholder: '15', description: '巴菲特偏好>15%', required: false, group: 'threshold' },
    ], description: '净资产收益率≥15%，优质公司特征' },
  { id: 'fin_profitable', name: '盈利公司(EPS>0)', category: Category.Financial, indicatorID: 'eps',
    defaultOperator: CompareOperator.GT, defaultParams: { value_number: 0 },
    paramDefs: [
      { key: 'value_number', label: 'EPS下界(元)', type: 'number', default: 0, min: -5, max: 10, step: 0.01, placeholder: '0', required: false, group: 'value_range' },
    ], description: '基本每股收益大于0，排除亏损企业' },
  { id: 'fin_high_growth', name: '高成长', category: Category.Financial, indicatorID: 'revenue_yoy',
    defaultOperator: CompareOperator.GT, defaultParams: { value_number: 20 },
    paramDefs: [
      { key: 'value_number', label: '营收增速(%)', type: 'number', default: 20, min: -50, max: 200, step: 1, placeholder: '20', required: false, group: 'value_range' },
    ], description: '营收同比增长超过20%' },
  { id: 'fin_low_debt', name: '低负债', category: Category.Financial, indicatorID: 'debt_ratio',
    defaultOperator: CompareOperator.LT, defaultParams: { value_number: 50 },
    paramDefs: [
      { key: 'value_number', label: '负债率上限(%)', type: 'threshold', default: 50, min: 10, max: 100, step: 1, placeholder: '50', required: false, group: 'threshold' },
    ], description: '资产负债率低于50%，财务稳健' },
  { id: 'fin_positive_ocf', name: '正经营现金流', category: Category.Financial, indicatorID: 'ocfps',
    defaultOperator: CompareOperator.GT, defaultParams: { value_number: 0 },
    paramDefs: [], description: '每股经营现金流转正，真金白银赚钱' },
]

// ========== 操作符注册表（Go 各指标文件 init() 注册）==========
const opRegistry: Record<string, OperatorOption[]> = {
  'macd_cross': seriesOps, 'macd_hist': numberOps, 'macd_divergence': seriesOps,
  'ma_cross': seriesOps, 'kdj_cross': seriesOps,
  'rsi6': numberOps, 'rsi12': numberOps,
  'boll': seriesOps, 'boll_squeeze': numberOps,
  'volume': numberOps, 'volume_ratio': numberOps,
}

// ========== API 函数 ==========

/** 获取所有指标 + 其预设信号模板 + 操作符 */
export function getAllIndicators(): Record<Category, IndicatorWithPresets[]> {
  const result: Record<string, IndicatorWithPresets[]> = {}

  for (const ind of rawIndicators) {
    const ops = opRegistry[ind.id] || defaultOpsForType(ind.valueType)
    const presets = rawPresets.filter(p => p.indicatorID === ind.id)
    const wrapped: IndicatorWithPresets = { ...ind, operators: ops, presets }
    if (!result[ind.category]) result[ind.category] = []
    result[ind.category].push(wrapped)
  }
  return result as Record<Category, IndicatorWithPresets[]>
}

/** 获取单个指标的详情（含模板和操作符） */
export function getIndicatorByID(id: string): IndicatorWithPresets | null {
  const ind = rawIndicators.find(i => i.id === id)
  if (!ind) return null
  const ops = opRegistry[id] || defaultOpsForType(ind.valueType)
  const presets = rawPresets.filter(p => p.indicatorID === id)
  return { ...ind, operators: ops, presets }
}

/** 根据 ID 获取单个预设信号模板 */
export function getPresetByID(id: string): PresetSignal | undefined {
  return rawPresets.find(p => p.id === id)
}

/** 获取所有预设信号模板 */
export function getAllPresets(): PresetSignal[] {
  return [...rawPresets]
}

/** 分类标签映射 */
export const categoryLabels: Record<Category, string> = {
  [Category.Technical]: '📈 技术面',
  [Category.Market]: '📊 行情面',
  [Category.Fundamental]: '🏢 基本面',
  [Category.Financial]: '💰 财务面',
}

export const valueTypeLabels: Record<ValueType, string> = {
  [ValueType.Number]: '数值', [ValueType.Bool]: '布尔',
  [ValueType.Enum]: '枚举', [ValueType.Series]: '序列',
}

/** 枚举选项 */
export const enumOptions: Record<string, string[]> = {
  listing_board: ['main', 'chinext', 'star', 'neeq'],
  listing_board_labels: ['主板', '创业板', '科创板', '北交所'],
  industry: ['农业', '采掘', '化工', '钢铁', '有色金属', '电子', '汽车', '机械设备', '计算机', '传媒', '电气设备', '国防军工', '医药生物', '家用电器', '食品饮料', '纺织服装', '轻工制造', '商业贸易', '休闲服务', '建筑材料', '建筑装饰', '房地产', '银行', '非银金融', '交通运输', '公用事业', '综合'],
}
