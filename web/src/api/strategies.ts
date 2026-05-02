/**
 * 策略管理 API
 * 对应后端: /api/v1/strategies
 *
 * 后端端口: 9100 (Go Gin)
 * 前端端口: 5175 (Vite dev server)
 */

import { getToken } from '../utils/auth'

const BASE = 'http://localhost:9100/api/v1/strategies'

// ========== 类型定义 ==========

export interface StrategySignal {
  uid: number
  id: string
  name: string
  category: string       // technical/fundamental/market/financial
  operator: string        // < > = between cross_up 等
  opSym: string           // 运算符符号 < > ↑ ↗ ∈
  opLbl: string           // 运算符中文
  params: Record<string, any>
  paramText: string       // 参数文本描述
}

export interface StrategyDetail {
  id: number
  uid: number             // 用户ID(预留)
  name: string
  logical_op: string      // "and" | "or"
  signals: StrategySignal[]
  description: string
  backtest_count: number
  created_at: string
  updated_at: string
}

export interface StrategyListItem {
  id: number
  uid: number
  name: string
  logical_op: string
  description: string
  backtest_count: number
  last_run_at: string | null   // 最后运行时间
  is_public: boolean           // 是否公开
  created_at: string           // 创建时间
  updated_at: string
}

export interface ListResponse {
  list: StrategyListItem[] | StrategyDetail[]
  total: number
  page: number
  size: number
}

// ========== 请求封装 ==========

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

// ========== CRUD 接口 ==========

/** 获取策略列表（支持搜索+分页） */
export async function fetchStrategies(keyword = '', page = 1, size = 100): Promise<ListResponse> {
  const params = new URLSearchParams({ page: String(page), size: String(size) })
  if (keyword) params.set('keyword', keyword)
  return request<ListResponse>(`${BASE}?${params}`)
}

/** 获取单个策略详情（含 signals） */
export async function fetchStrategyById(id: number): Promise<StrategyDetail> {
  return request<StrategyDetail>(`${BASE}/${id}`)
}

/** 创建策略 */
export async function createStrategy(data: {
  name: string; logical_op?: string; signals?: StrategySignal[]; description?: string
}): Promise<StrategyDetail> {
  return request<StrategyDetail>(BASE, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

/** 更新策略（全量） */
export async function updateStrategy(id: number, data: {
  name: string; logical_op?: string; signals?: StrategySignal[]; description?: string
}): Promise<StrategyDetail> {
  return request<StrategyDetail>(`${BASE}/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

/** 删除策略（软删除） */
export async function deleteStrategy(id: number): Promise<void> {
  await request<void>(`${BASE}/${id}`, { method: 'DELETE' })
}

/** 批量删除策略 */
export async function batchDeleteStrategies(ids: number[]): Promise<void> {
  await request<void>(`${BASE}/batch`, {
    method: 'DELETE',
    body: JSON.stringify({ ids }),
  })
}

/** 重命名策略 */
export async function renameStrategy(id: number, newName: string): Promise<void> {
  await request<void>(`${BASE}/${id}/rename`, {
    method: 'PUT',
    body: JSON.stringify({ name: newName }),
  })
}
