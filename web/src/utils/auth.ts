/**
 * 认证工具函数
 * Token 存储在 localStorage
 */

const TOKEN_KEY = 'auth_token'

/** 保存 token */
export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

/** 获取 token */
export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

/** 删除 token（退出登录） */
export function removeToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

/** 是否已登录 */
export function isLoggedIn(): boolean {
  return !!getToken()
}
