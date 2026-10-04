/**
 * 策略选股复盘 API
 * 对应后端: GET /api/v1/strategies/:id/picks
 */

import { getToken } from '../utils/auth'

const BASE = '/api/v1/strategies'

// ========== 类型定义 ==========

/** 价格维度：开盘 / 最高 / 最低 / 收盘 */
export type PriceDim = 'open' | 'high' | 'low' | 'close'

/** 选股日之后某个交易日的表现（相对选股日收盘价，单位 %） */
export interface ForwardDay {
  offset: number          // 相对选股日的交易日序号，1 起
  date: string            // 交易日 YYYY-MM-DD
  open_pct: number        // 开盘价收益率(%)
  high_pct: number        // 最高价收益率(%)
  low_pct: number         // 最低价收益率(%)
  close_pct: number       // 收盘价收益率(%)
}

/** 单条选股记录 */
export interface StockPick {
  date: string            // 选股日 YYYY-MM-DD
  code: string            // 股票代码
  name: string            // 股票名称
  signal_id: string       // 命中的信号ID
  message: string         // 信号描述
  base_close: number      // 基准价(元)：选股日收盘价
  max_pct: number         // 窗口内最高价收益率(%)
  min_pct: number         // 窗口内最低价收益率(%)
  returns: ForwardDay[]   // 之后逐交易日表现
}

/** 单个交易日的选股结果 */
export interface PickDayResult {
  date: string
  scanned: number         // 当日参与筛选股票数
  pick_count: number      // 当日命中数量
  picks: StockPick[]
}

/** 复盘结果 */
export interface StrategyPicksResult {
  strategy_id: number
  strategy_name: string
  start_date: string      // 实际处理的起始交易日
  end_date: string        // 实际处理的结束交易日
  hold_days: number       // 观察窗口(交易日)
  scan_days: number       // 实际扫描的交易日数
  total_picks: number     // 命中总次数
  days: PickDayResult[]
}

export interface StrategyPicksParams {
  start_date: string      // YYYY-MM-DD
  end_date: string        // YYYY-MM-DD
  hold_days?: number      // 观察窗口(交易日)，默认 5
}

/** 复盘区间的交易日列表（前端按天轮询用） */
export interface StrategyPickDaysMeta {
  strategy_id: number
  strategy_name: string
  hold_days: number
  start_date: string
  end_date: string
  dates: string[]         // 需要回放的交易日，升序
}

// ========== 请求封装 ==========

/** 将 HTTP 状态码转换为可读的错误信息 */
function httpErrorMessage(status: number, fallback: string): string {
  if (status === 502 || status === 503 || status === 504) {
    return '后端服务不可用，请确认服务已启动后再试'
  }
  if (status === 401) return '登录已过期，请重新登录'
  return fallback || `请求失败 (HTTP ${status})`
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const token = getToken()
  let res: Response
  try {
    res = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options?.headers,
      },
    })
  } catch {
    // fetch 抛错通常是网络不通 / 代理目标不可达
    throw new Error('网络错误，无法连接到后端服务，请确认服务已启动')
  }
  if (!res.ok) {
    if (res.status === 401) localStorage.removeItem('auth_token')
    const body = await res.json().catch(() => ({}))
    throw new Error(httpErrorMessage(res.status, body.error))
  }
  const json = await res.json()
  return json.data ?? json
}

// ========== API 接口 ==========

/**
 * 取复盘区间内需要回放的交易日列表（跳过周末/节假日）。
 * 前端拿到 dates 后逐天调用 fetchStrategyPickDay，自行汇总。
 */
export async function fetchStrategyPickDays(
  strategyId: number,
  params: StrategyPicksParams,
): Promise<StrategyPickDaysMeta> {
  const search = new URLSearchParams({
    start_date: params.start_date,
    end_date: params.end_date,
  })
  if (params.hold_days) search.set('hold_days', String(params.hold_days))
  return request<StrategyPickDaysMeta>(`${BASE}/${strategyId}/picks/days?${search.toString()}`)
}

/**
 * 回放单个交易日的选股结果，附带每只入选股票之后 N 个交易日的 OHLC 走势。
 * @param strategyId 策略 ID
 * @param params 交易日与观察窗口参数
 */
export async function fetchStrategyPickDay(
  strategyId: number,
  params: { date: string; hold_days?: number },
): Promise<PickDayResult> {
  const search = new URLSearchParams({ date: params.date })
  if (params.hold_days) search.set('hold_days', String(params.hold_days))
  return request<PickDayResult>(`${BASE}/${strategyId}/picks/day?${search.toString()}`)
}

// ========== 辅助函数 ==========

/** 取某个价格维度对应的收益率(%) */
export function pickDimValue(day: ForwardDay, dim: PriceDim): number | null {
  if (!day) return null
  switch (dim) {
    case 'open': return day.open_pct
    case 'high': return day.high_pct
    case 'low': return day.low_pct
    case 'close': return day.close_pct
    default: return null
  }
}

/** 选股记录唯一标识（同一只股票可能被不同交易日选中） */
export function pickKey(pick: StockPick): string {
  return `${pick.date}|${pick.code}`
}
