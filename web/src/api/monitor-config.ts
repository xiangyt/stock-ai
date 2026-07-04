import { getToken } from '../utils/auth'

const BASE = 'http://localhost:9100/api/v1/monitor-configs'

// ========== 请求封装 ==========

function request(url: string, options?: RequestInit): Promise<any> {
  const token = getToken()
  return fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...((options?.headers as Record<string, string>) || {}),
    },
  }).then(async (res) => {
    const data = await res.json()
    if (!res.ok) throw new Error(data.message || '请求失败')
    return data
  })
}

// ========== 类型定义 ==========

/** 监控范围 */
export type MonitorScope = 'held' | 'custom'

/** 规则类型 */
export type RuleType =
  | 'daily_change'
  | 'rapid_move'
  | 'volume_ratio'
  | 'seal_board'

/** 告警规则（动态类型 + 参数） */
export interface MonitorRule {
  type: RuleType
  params: Record<string, number | boolean> // 类型特定参数
}

/** 当日涨幅参数 */
export interface DailyChangeParams {
  surge_big_enabled: boolean
  surge_big: number
  surge_small_enabled: boolean
  surge_small: number
  limit_up_enabled: boolean
  limit_up: number
  limit_down_enabled: boolean
  limit_down: number
  drop_small_enabled: boolean
  drop_small: number
  drop_big_enabled: boolean
  drop_big: number
}

/** 急拉急跌参数 */
export interface RapidMoveParams {
  minutes: number
  amplitude_pct: number
  up_enabled: boolean
  down_enabled: boolean
}

/** 量比异动参数 */
export interface VolumeRatioParams {
  min_ratio: number
}

/** 封单监控参数 */
export interface SealBoardParams {
  min_lots: number
}

/** 冷却策略 */
export interface MonitorCooldown {
  interval_minutes: number
  daily_max: number
}

/** 机器人信息 */
export interface BotInfo {
  id: number
  name: string
  channel: string
}

/** 监控配置详情 */
export interface MonitorConfigDetail {
  id: number
  uid: number
  name: string
  scope: MonitorScope
  stocks: string[]
  rule: MonitorRule
  cooldown: MonitorCooldown
  template: string
  is_active: boolean
  bots: BotInfo[]
  created_at: string
  updated_at: string
}

/** 创建监控配置请求 */
export interface CreateMonitorConfigReq {
  name: string
  scope?: MonitorScope
  stocks?: string[]
  rule: MonitorRule
  cooldown: MonitorCooldown
  template?: string
  bot_ids?: number[]
}

/** 更新监控配置请求（部分字段） */
export interface UpdateMonitorConfigReq {
  name: string
  scope: MonitorScope
  stocks?: string[]
  rule: MonitorRule
  cooldown: MonitorCooldown
  template: string
}

/** 列表响应 */
export interface MonitorConfigListResp {
  data: MonitorConfigDetail[]
  total: number
  page: number
  page_size: number
}

// ========== 常量映射 ==========

export const ScopeLabels: Record<MonitorScope, string> = {
  held: '我的持仓',
  custom: '自选股票',
}

export const RuleTypeLabels: Record<RuleType, string> = {
  daily_change: '当日涨幅监控',
  rapid_move: '急拉急跌监控',
  volume_ratio: '量比异动监控',
  seal_board: '涨跌停封单监控',
}

/** 当日涨幅子类型 */
export const DailyChangeSubKeys = [
  { key: 'surge_big', label: '大涨阈值(%)' },
  { key: 'surge_small', label: '小涨阈值(%)' },
  { key: 'limit_up', label: '涨停阈值(%)' },
  { key: 'limit_down', label: '跌停阈值(%)' },
  { key: 'drop_small', label: '小跌阈值(%)' },
  { key: 'drop_big', label: '大跌阈值(%)' },
] as const

/** 短期振幅参数 key */
export const VolatilityParamKeys = [
  { key: 'minutes', label: '时间窗口(分钟)' },
  { key: 'amplitude_pct', label: '振幅阈值(%)' },
] as const

/** 封单参数 key */
export const SealBoardParamKeys = [
  { key: 'min_lots', label: '封单小于(手)' },
] as const

export const DefaultCooldown: MonitorCooldown = {
  interval_minutes: 5,
  daily_max: 3,
}

/** 每种规则类型的默认推送文案 */
export const DefaultRuleTemplates: Record<RuleType, string> = {
  daily_change: '[${alert_label}] ${name}(${code}) 报 ${price} (${change_pct}%)',
  rapid_move: '[${alert_label}] ${name}(${code}) ${change_pct}% 报 ${price}',
  volume_ratio: '[${alert_label}] ${name}(${code}) 量比 ${ratio} 成交 ${volume}',
  seal_board: '[${alert_label}] ${name}(${code}) 封单不足 ${min_lots}手',
}

/** 所有可用模板变量及其含义 */
export const AllTemplateVars = {
  name: '股票名称',
  code: '股票代码',
  alert_type: '规则大类 (daily_change / rapid_move / ...)',
  alert_subtype: '告警子类型 (surge_big / rapid_up / ...)',
  alert_label: '告警中文标签 (涨停 / 急拉 / 量比异动 / ...)',
  price: '当前价格(元)',
  change_pct: '涨跌幅 %',
  turnover: '换手率 %',
  volume: '成交量',
  minutes: '时间窗口(分钟) — 仅急拉急跌',
  amplitude: '幅度阈值 — 仅急拉急跌',
  ratio: '计算出的实际量比 — 仅量比异动',
  min_lots: '封单阈值(手) — 仅封单监控',
} as const

// ========== API 函数 ==========

export function listMonitorConfigs(
  page = 1,
  pageSize = 20,
): Promise<MonitorConfigListResp> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  return request(`${BASE}?${params}`)
}

export function getMonitorConfig(id: number): Promise<MonitorConfigDetail> {
  return request(`${BASE}/${id}`).then((d) => d.data)
}

export function createMonitorConfig(data: CreateMonitorConfigReq): Promise<MonitorConfigDetail> {
  return request(BASE, { method: 'POST', body: JSON.stringify(data) }).then((d) => d.data)
}

export function updateMonitorConfig(id: number, data: UpdateMonitorConfigReq): Promise<MonitorConfigDetail> {
  return request(`${BASE}/${id}`, { method: 'PUT', body: JSON.stringify(data) }).then((d) => d.data)
}

export function deleteMonitorConfig(id: number): Promise<void> {
  return request(`${BASE}/${id}`, { method: 'DELETE' })
}

export function setMonitorConfigActive(id: number, active: boolean): Promise<void> {
  return request(`${BASE}/${id}/active`, {
    method: 'PATCH',
    body: JSON.stringify({ is_active: active }),
  })
}

export function updateMonitorConfigBots(id: number, botIDs: number[]): Promise<void> {
  return request(`${BASE}/${id}/bots`, {
    method: 'PUT',
    body: JSON.stringify({ bot_ids: botIDs }),
  })
}
