/**
 * K 线数据 API
 * 对应后端: /api/v1/kline/:code
 */

import { getToken } from '../utils/auth'

const BASE = '/api/v1/kline'

// ========== 类型定义 ==========

/** 单条 K 线数据（纯 OHLCV，不含指标） */
export interface KLineItem {
  date: string          // YYYY-MM-DD
  open: number           // 开盘价(元)
  high: number           // 最高价(元)
  low: number            // 最低价(元)
  close: number          // 收盘价(元)
  volume: number         // 成交量(股)
  amount: number         // 成交额(分)
}

/** 均线指标数据 */
export interface MAData {
  ma5?: number[]
  ma10?: number[]
  ma20?: number[]
  ma60?: number[]
}

/** MACD 指标数据 */
export interface MACDData {
  dif?: number[]
  dea?: number[]
  macd?: number[]       // 柱状值 = 2*(DIF-DEA)
}

/** KDJ 指标数据 */
export interface KDJData {
  k?: number[]
  d?: number[]
  j?: number[]
}

/** 技术指标汇总 */
export interface IndicatorData {
  ma?: MAData
  macd?: MACDData
  kdj?: KDJData
}

/** K 线查询响应 */
export interface KLineResponse {
  code: string          // 股票代码
  period: string        // daily | weekly | monthly
  name?: string         // 股票名称
  items: KLineItem[]    // 纯K线数据
  indicators?: IndicatorData // 技标指标（独立数组）
}

/** K 线周期类型 */
export type KLinePeriod = 'daily' | 'weekly' | 'monthly'

// ========== 请求封装 ==========

async function request<T>(url: string): Promise<T> {
  const token = getToken()
  const res = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  const json = await res.json()
  return json.data ?? json
}

// ========== API 接口 ==========

/**
 * 获取股票 K 线数据（含均线/MACD/KDJ 指标）
 * @param code 股票代码，如 "300153"
 * @param period 周期: daily | weekly | monthly
 * @param limit 返回条数，默认 250
 */
export async function fetchKLine(code: string, period: KLinePeriod = 'daily', limit = 250): Promise<KLineResponse> {
  return request<KLineResponse>(`${BASE}/${code}?period=${period}&limit=${limit}`)
}

// ========== 指数日线 ==========

/** 指数单日行情（价格单位：元） */
export interface IndexDailyPoint {
  date: string
  open: number
  high: number
  low: number
  close: number
}

/** 指数日线查询结果 */
export interface IndexDailyResult {
  code: string          // 指数代码，如 000001
  name: string          // 指数名称，如 上证指数
  items: IndexDailyPoint[]
}

/**
 * 获取指数日线（默认上证指数）
 * @param start 起始日期 YYYY-MM-DD
 * @param end 结束日期 YYYY-MM-DD
 * @param code 指数代码，默认 000001（上证指数）
 */
export async function fetchIndexDaily(start: string, end: string, code = '000001'): Promise<IndexDailyResult> {
  const search = new URLSearchParams({ code, start, end })
  return request<IndexDailyResult>(`/api/v1/index-kline?${search.toString()}`)
}
