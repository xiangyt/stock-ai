import { getToken } from '../utils/auth'

const BASE = 'http://localhost:9100/api/v1/datacollect'

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

/** 机器人信息 */
export interface BotInfo {
  id: number
  name: string
  channel: string
}

/** 数据采集任务 */
export interface DataCollectTaskItem {
  id: number
  name: string
  cron_expr: string
  params: string       // JSON 格式
  is_active: boolean
  bots: BotInfo[]
  created_at: string
  updated_at: string
}

/** 更新任务请求 */
export interface UpdateTaskReq {
  cron_expr: string
  params: string
  is_active: boolean
}

// ========== API 函数 ==========

/** 获取所有数据采集任务 */
export function listDataCollectTasks(): Promise<DataCollectTaskItem[]> {
  return request(BASE).then((d) => d.data)
}

/** 获取任务详情 */
export function getDataCollectTask(id: number): Promise<DataCollectTaskItem> {
  return request(`${BASE}/${id}`).then((d) => d.data)
}

/** 更新任务配置 */
export function updateDataCollectTask(id: number, data: UpdateTaskReq): Promise<DataCollectTaskItem> {
  return request(`${BASE}/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  }).then((d) => d.data)
}

/** 更新任务关联的推送机器人 */
export function updateDataCollectBots(id: number, botIDs: number[]): Promise<void> {
  return request(`${BASE}/${id}/bots`, {
    method: 'PUT',
    body: JSON.stringify({ bot_ids: botIDs }),
  })
}

/** 立即执行一次数据采集任务 */
export function executeDataCollectTask(id: number): Promise<{ message: string }> {
  return request(`${BASE}/${id}/execute`, {
    method: 'POST',
  })
}
