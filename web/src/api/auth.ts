/**
 * 认证相关 API
 * 对应后端: /api/v1/auth
 */

const BASE = 'http://localhost:9100/api/v1/auth'

// ========== 类型定义 ==========

export interface UserInfo {
  id: number
  username: string
  nickname: string
  avatar: string
  role: string
  created_at: string
}

export interface LoginResponse {
  token: string
  user: UserInfo
}

// ========== 请求封装（带 token） ==========

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(url, { ...options, headers })
  if (!res.ok) {
    // 401 时清除过期 token
    if (res.status === 401) localStorage.removeItem('auth_token')
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  const json = await res.json()
  return json.data ?? json
}

// ========== 接口 ==========

/** 登录 */
export async function login(username: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>(`${BASE}/login`, {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

/** 注册 */
export async function register(username: string, password: string, nickname?: string): Promise<UserInfo> {
  return request<UserInfo>(`${BASE}/register`, {
    method: 'POST',
    body: JSON.stringify({ username, password, nickname }),
  })
}

/** 获取当前用户信息 */
export async function getMe(): Promise<UserInfo> {
  return request<UserInfo>(`${BASE}/me`)
}

/** 更新账号信息（昵称/头像/密码） */
export async function updateAccount(data: {
  nickname?: string; avatar?: string; old_password?: string; new_password?: string
}): Promise<UserInfo> {
  return request<UserInfo>(`${BASE}/account`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}
