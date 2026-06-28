/**
 * 软件配置相关 API
 * 对应后端: /api/v1/auth/software-configs
 */

const BASE = 'http://localhost:9100/api/v1/auth/software-configs'

// ========== 类型定义 ==========

export interface SoftwareMeta {
  name: string
  display_name: string
  description: string
}

export interface SoftwareConfigItem {
  software_name: string
  display_name: string
  cookie: string
  extra: string
  enabled: boolean
  updated_at: string
}

export interface UpdateSoftwareConfigReq {
  cookie?: string
  extra?: Record<string, string>
  enabled?: boolean
}

// ========== 请求封装（带 token） ==========

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(url, { ...options, headers })
  if (!res.ok) {
    if (res.status === 401) localStorage.removeItem('auth_token')
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  const json = await res.json()
  return json.data ?? json
}

// ========== 接口 ==========

/** 获取支持的软件列表 */
export async function listSupportedSoftware(): Promise<SoftwareMeta[]> {
  return request<SoftwareMeta[]>(`${BASE}/supported`)
}

/** 获取当前用户的软件配置列表 */
export async function listSoftwareConfigs(): Promise<SoftwareConfigItem[]> {
  return request<SoftwareConfigItem[]>(BASE)
}

/** 更新某个软件配置 */
export async function updateSoftwareConfig(
  name: string,
  data: UpdateSoftwareConfigReq,
): Promise<SoftwareConfigItem> {
  return request<SoftwareConfigItem>(`${BASE}/${encodeURIComponent(name)}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}
