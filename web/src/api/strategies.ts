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

/** 前后端交互用的信号结构（不含前端展示字段） */
export interface StrategySignalInput {
  signal_id: string
  operator: string
  params: Record<string, any>
}

export interface StrategyDetail {
  id: number
  uid: number             // 用户ID(预留)
  name: string
  logical_op: string      // "and" | "or"
  signals: StrategySignalInput[]
  description: string
  exit_rules: string       // v1.1: 卖出规则 JSON
  position_rules: string   // v1.1: 仓位规则 JSON
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
  subscription_count: number
  last_run_at: string | null
  is_public: boolean
  created_at: string
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
  name: string; logical_op?: string; signals?: StrategySignalInput[]; description?: string
  exit_rules?: ExitRules; position_rules?: PositionRules
}): Promise<StrategyDetail> {
  return request<StrategyDetail>(BASE, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

/** 更新策略（全量） */
export async function updateStrategy(id: number, data: {
  name: string; logical_op?: string; signals?: StrategySignalInput[]; description?: string
  exit_rules?: ExitRules; position_rules?: PositionRules
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

/** 切换策略公开/私有状态 */
export async function setStrategyPublic(id: number, isPublic: boolean): Promise<void> {
  await request<void>(`${BASE}/${id}/public`, {
    method: 'PUT',
    body: JSON.stringify({ is_public: isPublic }),
  })
}

/** 复制策略（以当前用户身份创建副本） */
export async function copyStrategy(id: number): Promise<StrategyDetail> {
  return request<StrategyDetail>(`${BASE}/${id}/copy`, { method: 'POST' })
}

// ========== 回测 API ==========

const BACKTEST_BASE = 'http://localhost:9100/api/v1'

// --- 回测类型 ---

export type RunStatus = 'pending' | 'running' | 'done' | 'failed'

/** 单条卖出规则配置（v1.1 可插拔架构） */
export interface ExitRuleConfig {
  type: string               // "stop_loss" | "take_profit" | "time_exit" | "trailing_stop" | "segment_profit"
  enabled: boolean
  params: Record<string, any> // 类型专属参数
  priority?: number           // 可选，覆盖默认优先级
}

/** 卖出规则集（v1.1: rules[] 数组格式） */
export interface ExitRules {
  rules: ExitRuleConfig[]
  slippage_pct: number        // 默认 0.3
}

/** 仓位分配配置（v1.1 对象格式） */
export interface AllocationConfig {
  type: string                // "equal" | "signal_weighted" | "volatility_weighted" | "risk_parity" | "custom_weight"
  params?: Record<string, any>
}

/** 仓位管理规则 */
export interface PositionRules {
  max_positions: number       // 0 = 不限制
  max_single_pct: number      // 0 = 不限制
  allocation: string | AllocationConfig  // v1.1: "equal" 或 {"type":"equal","params":{}}
  cash_buffer_pct?: number    // 现金缓冲比例，默认 5
}

export interface BacktestRun {
  id: number
  strategy_id: number
  uid: number
  stock_pool: string[]         // JSON 数组
  start_date: string
  end_date: string
  initial_capital: number
  final_equity: number | null
  exit_rules: string           // JSON string, 前端用 JSON.parse
  position_rules: string       // JSON string
  status: RunStatus
  progress_pct: number
  error_message?: string
  total_return: number | null
  annual_return: number | null
  max_drawdown: number | null
  sharpe_ratio: number | null
  win_rate: number | null
  profit_factor: number | null
  trade_count: number
  stop_loss_count: number
  take_profit_count: number
  time_exit_count: number
  created_at: string
  updated_at: string
}

export interface BacktestTrade {
  id: number
  run_id: number
  stock_code: string
  stock_name: string            // 股票名称（后端查询时填充）
  trade_type: number           // 1=买入 2=卖出
  quantity: number
  price: number
  amount: number
  commission: number
  stamp_tax: number
  trade_date: string
  exit_reason?: string         // "stop_loss" | "take_profit" | "time_exit"
  pre_exit_price?: number
  profit_loss?: number
  profit_loss_pct?: number
  created_at: string
}

export interface DailySnapshot {
  id: number
  run_id: number
  snap_date: string
  total_equity: number
  cash: number
  market_value: number
  position_count: number
  daily_return: number | null
  cumulative_return: number | null
  benchmark_value: number | null
  created_at: string
}

export interface InitiateBacktestRequest {
  stock_pool: string[]
  start_date: string
  end_date: string
  initial_capital: number
  exit_rules_override?: ExitRules    // 可选覆盖策略默认规则
  position_rules_override?: PositionRules
}

export interface InitiateBacktestResponse {
  run_id: number
  status: RunStatus
}

export interface RunStatusResponse {
  status: RunStatus
  progress_pct: number
}

export interface TradeListResponse {
  total: number
  items: BacktestTrade[]
}

export interface SnapshotListResponse {
  snapshots: DailySnapshot[]
}

// --- 回测 API 方法 ---

/** 发起回测 */
export async function initiateBacktest(strategyId: number, data: InitiateBacktestRequest): Promise<InitiateBacktestResponse> {
  return request<InitiateBacktestResponse>(`${BACKTEST_BASE}/strategies/${strategyId}/backtest`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

/** 获取单次回测详情 */
export async function getBacktestRun(runId: number): Promise<BacktestRun> {
  return request<BacktestRun>(`${BACKTEST_BASE}/backtest/runs/${runId}`)
}

/** 获取回测运行状态（用于轮询进度） */
export async function getBacktestRunStatus(runId: number): Promise<RunStatusResponse> {
  return request<RunStatusResponse>(`${BACKTEST_BASE}/backtest/runs/${runId}/status`)
}

/** 获取回测交易明细（分页） */
export async function getBacktestTrades(runId: number, page = 1, pageSize = 20): Promise<TradeListResponse> {
  return request<TradeListResponse>(`${BACKTEST_BASE}/backtest/runs/${runId}/trades?page=${page}&page_size=${pageSize}`)
}

/** 停止正在运行的回测 */
export async function stopBacktest(runId: number): Promise<void> {
  return request<void>(`${BACKTEST_BASE}/backtest/runs/${runId}/stop`, { method: 'POST' })
}

/** 获取回测每日快照（净值曲线数据，支持增量加载） */
export async function getBacktestSnapshots(runId: number, afterId = 0): Promise<SnapshotListResponse> {
  return request<SnapshotListResponse>(`${BACKTEST_BASE}/backtest/runs/${runId}/snapshots?after_id=${afterId}`)
}

/** 获取策略的历史回测列表 */
export async function getBacktestRuns(strategyId: number, limit = 20): Promise<BacktestRun[]> {
  return request<BacktestRun[]>(`${BACKTEST_BASE}/strategies/${strategyId}/backtest/runs?limit=${limit}`)
}

/** 删除回测记录 */
export async function deleteBacktestRun(runId: number): Promise<void> {
  await request<void>(`${BACKTEST_BASE}/backtest/runs/${runId}`, { method: 'DELETE' })
}

// ========== 加入自选 ==========

export interface BatchAddFavoritesResp {
  gid: string
  gname: string
  total: number
  failed: string[]
}

/** 一键加入东财自选分组 */
export async function batchAddToFavorites(data: {
  strategy_id: number
  date: string       // YYYYMMDD
  stock_codes: string[]
}): Promise<BatchAddFavoritesResp> {
  return request<BatchAddFavoritesResp>(`${BACKTEST_BASE}/backtest/batch/favorites`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}
