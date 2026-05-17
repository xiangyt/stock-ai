/**
 * 持仓管理 API
 * 对应后端: /api/v1/positions + /api/v1/user/trade-config
 */

import { getToken } from '../utils/auth'

const BASE = 'http://localhost:9100/api/v1/positions'
const CONFIG_BASE = 'http://localhost:9100/api/v1/user/trade-config'

// ========== 类型定义 ==========

export interface PositionDetail {
  id: number
  uid: number
  stock_code: string
  stock_name?: string
  quantity: number
  avg_cost: number
  status: string          // 'holding' | 'closed'
  total_cost: number
  trade_count: number
  note: string
  created_at: string
  updated_at: string
  trades?: PositionTrade[]
}

export interface PositionTrade {
  id: number
  position_id: number
  trade_type: number      // 1=买 2=卖
  trade_type_name: string // '买入' | '卖出'
  quantity: number
  price: number
  amount: number
  commission: number
  trade_date: string
  note: string
}

export interface PositionListResp {
  list: PositionDetail[]
  total: number
  page: number
  size: number
  summary: {
    holding_count: number
    closed_count: number
    total_cost: number
    total_quantity: number
  }
}

export interface TradeConfig {
  commission_rate: number   // 万分之x (如 2.5)
  min_commission: boolean    // true=不免五(有最低5元)
}

export interface OpenPositionPayload {
  stock_code: string
  quantity: number
  price: number
  entry_price?: number
  trade_date: string        // YYYY-MM-DD
  note?: string
}

export interface TradePayload {
  quantity: number
  price: number
  trade_date: string
  note?: string
}

// ========== 请求封装 ==========

const FETCH_TIMEOUT = 10_000 // 10 秒超时

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const token = getToken()
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT)

  try {
    const res = await fetch(url, {
      ...options,
      signal: controller.signal,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options?.headers,
      },
    })
    clearTimeout(timer)

    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || `HTTP ${res.status}`)
    }
    const json = await res.json()
    return json.data ?? json
  } catch (err: any) {
    clearTimeout(timer)
    if (err.name === 'AbortError') {
      throw new Error('请求超时，请稍后重试')
    }
    throw err
  }
}

// ========== 持仓 CRUD ==========

/** 获取持仓列表（含统计概览） */
export async function fetchPositions(status = '', page = 1, size = 20): Promise<PositionListResp> {
  const params = new URLSearchParams({ page: String(page), size: String(size) })
  if (status && status !== 'all') params.set('status', status)
  return request<PositionListResp>(`${BASE}?${params}`)
}

/** 获取持仓详情（含交易记录） */
export async function fetchPositionById(id: number): Promise<PositionDetail> {
  return request<PositionDetail>(`${BASE}/${id}`)
}

/** 建仓 */
export async function openPosition(data: OpenPositionPayload): Promise<PositionDetail> {
  return request<PositionDetail>(BASE, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

/** 加仓 */
export async function buyMore(id: number, data: TradePayload): Promise<PositionDetail> {
  return request<PositionDetail>(`${BASE}/${id}/buy`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

/** 减仓 */
export async function sellPartial(id: number, data: TradePayload): Promise<PositionDetail> {
  return request<PositionDetail>(`${BASE}/${id}/sell`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

/** 清仓 */
export async function closePosition(id: number, price: number, tradeDate: string, note = ''): Promise<PositionDetail> {
  return request<PositionDetail>(`${BASE}/${id}/close`, {
    method: 'POST',
    body: JSON.stringify({ price: price, trade_date: tradeDate, note }),
  })
}

/** 更新持仓备注 */
export async function updatePositionNote(id: number, note: string): Promise<void> {
  return request<void>(`${BASE}/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ note }),
  })
}

/** 删除持仓记录 */
export async function deletePosition(id: number): Promise<void> {
  return request<void>(`${BASE}/${id}`, { method: 'DELETE' })
}

// ========== 交易配置 ==========

/** 获取用户交易配置 */
export async function fetchTradeConfig(): Promise<TradeConfig> {
  return request<TradeConfig>(CONFIG_BASE)
}

/** 更新用户交易配置 */
export async function updateTradeConfig(config: TradeConfig): Promise<void> {
  return request<void>(CONFIG_BASE, {
    method: 'PUT',
    body: JSON.stringify(config),
  })
}
