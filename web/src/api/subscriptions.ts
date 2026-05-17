import { getToken } from '../utils/auth'

const BASE = 'http://localhost:9100/api/v1/subscriptions'

// ========== 请求封装 ==========

function request(url: string, options?: RequestInit): Promise<any> {
  const token = getToken()
  return fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options?.headers as Record<string, string> || {}),
    },
  }).then(async (res) => {
    const data = await res.json()
    if (!res.ok) throw new Error(data.message || '请求失败')
    return data
  })
}

// ========== 类型定义 ==========

/** 监控范围 */
export type MonitorScope = 'all' | 'held' | 'custom'

/** 预设频率类型 */
export type PresetType =
  | 'every_15min' | 'every_30min' | 'every_hour'
  | 'daily_open' | 'daily_close' | 'daily_twice'
  | 'weekly' | 'custom'

/** 机器人信息（详情中的 bots 字段） */
export interface BotInfo {
  id: number
  name: string
  channel: string
}

/** 订阅列表项 */
export interface SubscriptionListItem {
  id: number
  name: string
  strategy_id: number
  strategy_name: string
  scope: MonitorScope
  preset_type: PresetType
  cron_expr: string
  trading_hours_only: boolean
  is_active: boolean
  last_run_at: string
  bots: BotInfo[]
  bot_count: number
  created_at: string
}

/** 订阅详情 */
export interface SubscriptionDetail {
  id: number
  uid: number
  name: string
  strategy_id: number
  strategy_name: string
  scope: MonitorScope
  custom_stocks: string      // JSON: string[]
  preset_type: PresetType
  cron_expr: string
  trading_hours_only: boolean
  is_active: boolean
  template: string
  last_run_at: string
  bots: BotInfo[]
  created_at: string
  updated_at: string
}

/** 创建订阅请求 */
export interface CreateSubscriptionReq {
  name: string
  strategy_id: number
  scope?: MonitorScope
  custom_stocks?: string[]
  preset_type?: PresetType
  cron_expr?: string
  trading_hours_only?: boolean
  template?: string
  bot_ids?: number[]
}

/** 更新订阅请求（部分更新，null 字段不更新） */
export interface UpdateSubscriptionReq {
  name?: string
  strategy_id?: number
  scope?: MonitorScope
  custom_stocks?: string[]
  preset_type?: PresetType
  cron_expr?: string
  trading_hours_only?: boolean
  is_active?: boolean
  template?: string
}

/** 订阅列表响应 */
export interface SubscriptionListResp {
  list: SubscriptionListItem[]
  total: number
}

/** 手动触发结果 */
export interface TriggerResult {
  message: string
}

// ========== 常量映射 ==========

export const ScopeLabels: Record<MonitorScope, string> = {
  all: '全部A股',
  held: '我的持仓',
  custom: '自选股票',
}

export const PresetTypeLabels: Record<PresetType, string> = {
  every_15min: '每15分钟',
  every_30min: '每30分钟',
  every_hour: '每小时',
  daily_open: '每日开盘',
  daily_close: '每日收盘',
  daily_twice: '每日两次',
  weekly: '每周',
  custom: '自定义',
}

// ========== API 函数 ==========

/** 获取订阅列表（分页 + 可选状态过滤） */
export function listSubscriptions(
  page = 1,
  pageSize = 20,
  isActive?: boolean,
): Promise<SubscriptionListResp> {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  })
  if (isActive !== undefined) {
    params.set('is_active', String(isActive))
  }
  return request(`${BASE}?${params}`).then((d) => d.data)
}

/** 获取订阅详情 */
export function getSubscription(id: number): Promise<SubscriptionDetail> {
  return request(`${BASE}/${id}`).then((d) => d.data)
}

/** 创建订阅 */
export function createSubscription(data: CreateSubscriptionReq): Promise<SubscriptionDetail> {
  return request(BASE, {
    method: 'POST',
    body: JSON.stringify(data),
  }).then((d) => d.data)
}

/** 更新订阅（部分更新） */
export function updateSubscription(
  id: number,
  data: UpdateSubscriptionReq,
): Promise<SubscriptionDetail> {
  return request(`${BASE}/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  }).then((d) => d.data)
}

/** 删除订阅 */
export function deleteSubscription(id: number): Promise<void> {
  return request(`${BASE}/${id}`, { method: 'DELETE' })
}

/** 切换订阅启停状态 */
export function setSubscriptionActive(id: number, active: boolean): Promise<void> {
  return request(`${BASE}/${id}/active`, {
    method: 'PATCH',
    body: JSON.stringify({ is_active: active }),
  })
}

/** 手动触发订阅执行 */
export function triggerSubscriptionRun(id: number): Promise<TriggerResult> {
  return request(`${BASE}/${id}/run`, {
    method: 'POST',
  }).then((d) => d.data)
}

/** 更新订阅关联的推送机器人 */
export function updateSubscriptionBots(id: number, botIDs: number[]): Promise<void> {
  return request(`${BASE}/${id}/bots`, {
    method: 'PUT',
    body: JSON.stringify({ bot_ids: botIDs }),
  })
}
