import http, { get, post, put } from '../http'

// ─── Types ───────────────────────────────────────────────

export interface User {
  id: number
  username: string
  email: string
  display_name: string
  avatar: string
  bio: string
  website: string
  role: Role
  status: string
  login_count: number
  preferences: UserPreferences
  created_at: string
}

export interface UserPreferences {
  language: string
  theme: string
  email_notify: boolean
  markdown_editor: boolean
  items_per_page: number
  default_post_status: string
}

export interface Role {
  id: number
  name: string
  slug: string
  description: string
  permissions: Permission[]
  is_system: boolean
  user_count?: number
}

export interface Permission {
  id: number
  name: string
  slug: string
  module: string
  description: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  expires_at: string
  expires_in: number
}

// ─── API ─────────────────────────────────────────────────

// Auth
export const authApi = {
  login: (data: { username: string; password: string; totp_code?: string }) =>
    post<{ data: { token: TokenPair; user: User } }>('/auth/login', data),
  register: (data: { username: string; email: string; password: string; display_name?: string }) =>
    post<{ data: { token: TokenPair; user: User } }>('/auth/register', data),
  // _retry=true 让 HTTP 拦截器跳过 refresh 流程：refresh 请求本身 401 时
  // 直接 reject，否则拦截器会再次调 refreshAccessToken → 死锁，导致
  // clearAuth() 永远不被调用、token 永远清不掉、用户卡在空白页。
  refresh: (refresh_token: string) =>
    http.post<{ data: TokenPair }>('/auth/refresh', { refresh_token }, { _retry: true }).then(r => r.data),
  logout: (refresh_token?: string) => post('/auth/logout', { refresh_token }),
  me: () => get<{ data: { user: User; permissions: string[] } }>('/auth/me'),
  updateProfile: (data: Partial<User>) => put<{ data: User }>('/auth/profile', data),
  changePassword: (data: { old_password: string; new_password: string }) =>
    put('/auth/password', data),
}

// TOTP two-factor authentication (current user)
export const totpApi = {
  status: () => get<{ data: { enabled: boolean } }>('/auth/totp/status'),
  setup: () => post<{ data: { secret: string; otpauth_uri: string } }>('/auth/totp/setup'),
  enable: (code: string) => post<{ data: { backup_codes: string[] } }>('/auth/totp/enable', { code }),
  disable: (password: string) => post('/auth/totp/disable', { password }),
}
