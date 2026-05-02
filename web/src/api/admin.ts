import { getToken } from '../utils/auth'

const BASE = 'http://localhost:9100/api/v1/auth/admin'

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

/** 用户列表项 */
export interface AdminUserItem {
  id: number
  username: string
  nickname: string
  avatar: string
  role: string
  status: number // 1=正常 0=禁用
  created_at: string
}

/** 获取所有用户列表 */
export function listUsers(): Promise<AdminUserItem[]> {
  return request(`${BASE}/users`).then((d) => d.data)
}

/** 启用/禁用用户 */
export function toggleUserStatus(userId: number, status: number): Promise<void> {
  return request(`${BASE}/users/${userId}/status`, {
    method: 'PUT',
    body: JSON.stringify({ status }),
  })
}

/** 重置用户密码 */
export function resetUserPassword(userId: number, password: string): Promise<void> {
  return request(`${BASE}/users/${userId}/password`, {
    method: 'PUT',
    body: JSON.stringify({ password }),
  })
}
