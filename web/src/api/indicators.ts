/**
 * 指标元数据 API — 对齐后端 screener/indicator.ToAPIMeta()
 *
 * 后端数据源:
 *   indicator/types.go   → APIMeta / IndicatorMeta / SignalDef / OperatorMeta / ParamDef
 *   indicator/registry.go → Registry.ToAPIMeta() 输出
 *
 * 后端端口: 9100 (Go Gin)
 * 前端端口: 5173 (Vite dev server)
 *
 * [v2] 数字ID编码体系:
 *   SignalID = 8位数字 "CCIIISSTT"
 *     CC  = 分类码 (01技术 02行情 03基本 04财务)
 *     III = 指标序号 (001~999)
 *     S  = 来源标志 (0=内置, 1=自定义)
 *    TT  = 信号序号 (01~99)
 */

import { getToken } from '../utils/auth'

const BASE = 'http://localhost:9100/api/v1/indicators'

// ============================================================================
//  类型定义（与后端 ToAPIMeta 输出对齐）
// ============================================================================

/** 指标分类标识 */
export type Category = 'technical' | 'market' | 'fundamental' | 'financial'

/** 分类码（SignalID 前两位） */
export type CategoryCode = '01' | '02' | '03' | '04'

/** 值类型 */
export type ValueType = 'number' | 'series' | 'bool' | 'enum' | 'string'

/** 比较操作符 */
export type CompareOperator =
  | 'gt' | 'gte' | 'lt' | 'lte'
  | 'eq' | 'neq'
  | 'between' | 'not_between'
  | 'in' | 'not_in' | 'contains'
  | 'cross_above' | 'cross_below'
  | 'divergence_pos' | 'divergence_neg'
  | 'rising' | 'falling'
  | 'custom'

/** PresetMode 内置信号执行模式 */
export type PresetMode = 'simple' | 'combo' | 'custom'

// ---------- 分类元信息 ----------

export interface CategoryMeta {
  id: Category
  name: string
  desc: string
}

// ---------- 枚举选项 ----------

export interface EnumOption {
  value: string
  label: string
  desc?: string
}

// ---------- 参数定义 ----------

export type ParamType = 'number' | 'range' | 'days' | 'select' | 'multi_select' | 'threshold' | 'boolean' | 'string' | 'select_multi'

export interface ParamDef {
  key: string
  label: string
  type: ParamType

  default?: any
  min?: number
  max?: number
  step?: number

  options?: EnumOption[]

  required?: boolean
  description?: string
  placeholder?: string
  unit?: string

  hidden?: boolean
  group?: string
  dependsOn?: string
  conditionValue?: string
}

// ---------- 操作符选项 ----------

export interface SignalOperatorOption {
  operator: CompareOperator
  label: string
  params: ParamDef[]
}

// ---------- 信号定义 ----------

export interface SignalDef {
  /** 8位数字唯一标识, 如 "03001001" */
  signal_id: string
  name: string
  /** 展示别名（非空时前端优先展示，hover 展示 description） */
  alias?: string
  description?: string
  operators: SignalOperatorOption[]
  default_config?: SignalConfig
  /** 预设配置列表（内置信号的快捷入口） */
  presets?: SignalPreset[]
}

// ---------- 预设配置 ----------

export interface SignalPreset {
  label: string
  mode: PresetMode
  /** simple 模式：单个预填配置 */
  config?: SignalConfig
  /** combo 模式：多个子配置 */
  configs?: SignalConfig[]
  /** combo 逻辑: "and" | "or" */
  combo_logic?: 'and' | 'or'
  // custom 模式的 EvalFunc 不传前端
}

// ---------- 运行时信号配置 ----------

/**
 * 策略保存时的精简格式
 * signal_id 是 8 位数字唯一标识，自带分组信息（前5位=指标ID）
 */
export interface SignalConfig {
  /** 8位数字唯一标识, 如 "03001001" */
  signal_id: string
  operator: CompareOperator
  params: Record<string, any>
}

// ---------- 操作符元信息 ----------

export interface OperatorMeta {
  operator: CompareOperator
  label: string
  value_type: ValueType
  desc: string
}

// ---------- 指标元数据 ----------

export interface IndicatorMeta {
  /** 5位指标标识符 (CCIII), 如 "03001" */
  id: string
  name: string
  category: Category
  description: string
  unit: string
  signals: SignalDef[]
}

// ---------- API 响应结构 ----------

export interface IndicatorsResponse {
  categories: CategoryMeta[]
  indicators: IndicatorMeta[]
  enum_options: Record<string, EnumOption[]>
}

// ============================================================================
//  兼容性别名
// ============================================================================

/** @deprecated 使用 IndicatorMeta 替代 */
export type Indicator = IndicatorMeta

/** @deprecated 使用 IndicatorsResponse 替代 */
export type IndicatorsData = IndicatorsResponse

/** @deprecated 使用 SignalDef 替代 */
export type SignalTemplate = SignalDef

// ============================================================================
//  SignalID 工具函数
// ============================================================================

/** 从 8 位 SignalID 提取前5位作为指标标识符 (CCIII = 分类码2位 + 指标序号3位) */
export function getIndicatorID(signalId: string): string {
  return signalId.slice(0, 5)
}

/** 判断是否为自定义信号 (第6位 S == '1', 索引为5) */
export function isCustomSignal(signalId: string): boolean {
  return signalId.length >= 8 && signalId[5] === '1'
}

/** 分类码映射 */
export const categoryCodeMap: Record<CategoryCode, Category> = {
  '01': 'technical',
  '02': 'market',
  '03': 'fundamental',
  '04': 'financial',
}

export const categoryNameMap: Record<CategoryCode, string> = {
  '01': '技术面',
  '02': '行情面',
  '03': '基本面',
  '04': '财务面',
}

// ============================================================================
//  API 接口
// ============================================================================

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const token = getToken()
  const res = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options?.headers,
    },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  const json = await res.json()
  return json.data ?? json
}

/** 获取全部指标元数据 */
export async function fetchIndicators(): Promise<IndicatorsResponse> {
  return request<IndicatorsResponse>(BASE)
}

/** 获取单个指标详情 */
export async function fetchIndicatorById(id: string): Promise<IndicatorMeta> {
  return request<IndicatorMeta>(`${BASE}/${id}`)
}

// ---------- 选股执行 ----------

/** 选股执行请求 */
export interface ExecuteRequest {
  configs: SignalConfig[]
  max_concurrency?: number
  /** 筛选日期，格式 YYYY-MM-DD，默认为今天 */
  date?: string
}

/** 单股评估结果 */
export interface EvaluatedStock {
  code: string
  name: string
  price: number
  result: number // 0=passed, 1=rejected, 2=pending
  signal_id?: string
  message?: string
}

/** 选股执行响应 */
export interface ExecuteResponse {
  total: number
  passed: EvaluatedStock[]
  rejected: EvaluatedStock[]
}

/** 执行选股筛选 */
export async function executeScreen(req: ExecuteRequest): Promise<ExecuteResponse> {
  return request<ExecuteResponse>(`${BASE}/execute`, {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

// ============================================================================
//  工具函数
// ============================================================================

export function findSignalOperator(signal: SignalDef, op: CompareOperator): SignalOperatorOption | undefined {
  return signal.operators.find(o => o.operator === op)
}

/** 查找信号参数定义中的 unit 字段 */
function findParamUnit(ind: IndicatorMeta, paramKey: string): string | undefined {
  for (const sig of ind.signals) {
    for (const op of sig.operators) {
      const p = op.params.find(p => p.key === paramKey)
      if (p) return p.unit
    }
  }
  return undefined
}

/** 查找信号参数定义中的 label 和 unit */
function findParamMeta(ind: IndicatorMeta, paramKey: string): { label: string; unit: string } | undefined {
  for (const sig of ind.signals) {
    for (const op of sig.operators) {
      const p = op.params.find(p => p.key === paramKey)
      if (p) return { label: p.label, unit: p.unit }
    }
  }
  return undefined
}

/** 将 SignalConfig 转为可读描述（用于 chip 显示） */
export function formatSignalConfig(
  _signalId: string,
  config: SignalConfig,
  indicator: IndicatorMeta,
): string {
  switch (config.operator) {
    case 'gt':
      return `${(config.params.threshold ?? '')}${indicator.unit}`
    case 'gte':
      return `≥${(config.params.threshold ?? '')}${indicator.unit}`
    case 'lt':
      return `<${(config.params.threshold ?? '')}${indicator.unit}`
    case 'lte':
      return `≤${(config.params.threshold ?? '')}${indicator.unit}`
    case 'between': case 'not_between':
      return `[${config.params.min ?? ''}~${config.params.max ?? ''}]${indicator.unit}`
    case 'in': case 'not_in':
      return `{${(config.params.values as string[])?.join(',') || ''}}`
    case 'custom': {
      const start = config.params.lookback_start
      const end = config.params.lookback_end
      const startMeta = findParamMeta(ind, 'lookback_start')
      const endMeta = findParamMeta(ind, 'lookback_end')
      const sLabel = startMeta?.label ?? '起始天数'
      const eLabel = endMeta?.label ?? '结束天数'
      const sUnit = startMeta?.unit || '天前'
      const eUnit = endMeta?.unit || '天前'
      return `${sLabel}${start ?? 0}${sUnit}, ${eLabel}${end ?? 0}${eUnit}`
    }
    default:
      return Object.entries(config.params).map(([k, v]) => `${k}=${v}`).join(', ')
  }
}

/** 操作符 → 显示符号映射 */
export const operatorSymbols: Record<CompareOperator, string> = {
  gt: '>', gte: '≥', lt: '<', lte: '≤',
  eq: '', neq: '≠',
  between: '[]', not_between: ')(',
  in: '∈', not_in: '∉', contains: '∋',
  cross_above: '↑↑', cross_below: '↓↓',
  divergence_pos: '↘', divergence_neg: '↗',
  rising: '⤒', falling: '⤓',
  custom: '',
}

/** 分类 → 中文标签映射 */
export const categoryLabels: Record<Category, string> = {
  technical: '技术面',
  market: '行情面',
  fundamental: '基本面',
  financial: '财务面',
}

/** 分类 → 颜色映射 */
export const categoryColors: Record<Category, string> = {
  technical: '#08979c',
  market: '#0958d9',
  fundamental: '#d46b08',
  financial: '#52c41a',
}
