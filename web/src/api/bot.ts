import { getToken } from '../utils/auth'

const BASE = 'http://localhost:9100/api/v1/bots'

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
    if (!res.ok) throw new Error(data.error || '请求失败')
    return data
  })
}

// ====== 类型定义 ======

export type ChannelType = 'wecom' | 'dingtalk' | 'feishu'

/** 渠道显示名映射 */
export const ChannelLabels: Record<ChannelType, string> = {
  wecom: '企微',
  dingtalk: '钉钉',
  feishu: '飞书',
}

export interface PushBotItem {
  id: number
  user_id: number
  name: string
  channel: ChannelType
  webhook_url: string   // Webhook URL（企微/钉钉/飞书通用）
  secret: string        // 钉钉: 加签 Secret；其他渠道留空
  status: number        // 1=启用 0=禁用
  created_at: string
  updated_at: string
}

export interface CreatePushBotReq {
  name: string
  channel: ChannelType
  webhook_url?: string   // Webhook URL
  secret?: string        // 钉钉: 加签 Secret
}

export interface UpdatePushBotReq {
  name?: string
  channel?: ChannelType
  webhook_url?: string
  secret?: string
}

export interface TestPushResult {
  success: boolean
  message: string
}

// ====== API 函数 ======

/** 获取当前用户的推送机器人列表 */
export function listPushBots(): Promise<PushBotItem[]> {
  return request(BASE).then((d) => d.data)
}

/** 创建推送机器人 */
export function createPushBot(data: CreatePushBotReq): Promise<{ id: number }> {
  return request(BASE, {
    method: 'POST',
    body: JSON.stringify(data),
  }).then((d) => d.data)
}

/** 更新推送机器人 */
export function updatePushBot(id: number, data: UpdatePushBotReq): Promise<void> {
  return request(`${BASE}/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

/** 删除推送机器人 */
export function deletePushBot(id: number): Promise<void> {
  return request(`${BASE}/${id}`, {
    method: 'DELETE',
  })
}

/** 切换推送机器人状态（启用/禁用） */
export function togglePushBotStatus(id: number, status: number): Promise<void> {
  return request(`${BASE}/${id}/status`, {
    method: 'PUT',
    body: JSON.stringify({ status }),
  })
}

/** 测试推送 */
export function testPushBot(id: number): Promise<TestPushResult> {
  return request(`${BASE}/${id}/test`, {
    method: 'POST',
  }).then((d) => d.data)
}
