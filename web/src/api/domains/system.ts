import http, { get, post, del } from '../http'

// ─── Types ───────────────────────────────────────────────

export interface APIToken {
  id: number
  name: string
  permissions: string[] | null
  is_active: boolean
  expires_at: string | null
  last_used_at: string | null
  use_count: number
  created_by_id: number
  created_at: string
}

// Returned once after creation — the plaintext token is never shown again.
export interface TokenCreated {
  id: number
  name: string
  token: string
  permissions: string[] | null
  expires_at: string | null
  created_at: string
}

export interface BackupInfo {
  name: string
  path: string
  size: number
  created_at: string
}

export interface Webhook {
  id: number
  name: string
  url: string
  events: string[] | null
  headers: string[] | null
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface WebhookLog {
  id: number
  webhook_id: number
  event: string
  payload: string
  response: number
  duration: number
  success: boolean
  error?: string
  retries: number
  created_at: string
}

export interface SearchHit {
  id: number
  type: string
  title: string
  excerpt: string
  slug: string
  score: number
  highlight?: string
  locale?: string
  author_name?: string
  published_at?: string
}

export interface SearchResult {
  hits: SearchHit[] | null
  total: number
  page: number
  page_size: number
  total_pages: number
  took: string
}

// ─── API ─────────────────────────────────────────────────

export const systemApi = {
  info: () => get('/system/info'),
  health: () => get('/system/health'),
  activity: (params?: Record<string, unknown>) => get('/system/activity', params),
}

export const tokenApi = {
  list: () => get<{ data: APIToken[] }>('/system/tokens'),
  create: (data: { name: string; permissions?: string[]; expires_at?: string }) =>
    post<{ data: TokenCreated }>('/system/tokens', data),
  delete: (id: number) => del(`/system/tokens/${id}`),
}

export const backupApi = {
  list: () => get<{ data: BackupInfo[] }>('/admin/backup'),
  create: (type: 'db' | 'media' | 'all' = 'all') =>
    post<{ data: Record<string, string> }>(`/admin/backup?type=${type}`),
  restore: (file: string) =>
    post<{ data: Record<string, unknown> }>(`/admin/backup/${encodeURIComponent(file)}/restore`),
  delete: (file: string) => del(`/admin/backup/${encodeURIComponent(file)}`),
  download: (file: string) =>
    http.get<Blob>(`/admin/backup/${encodeURIComponent(file)}/download`, { responseType: 'blob' })
      .then(r => r.data),
}

export const webhookApi = {
  list: () => get<{ data: Webhook[] }>('/webhooks'),
  create: (data: { name: string; url: string; events: string[]; headers?: string[]; secret?: string }) =>
    post<{ data: Webhook }>('/webhooks', data),
  delete: (id: number) => del(`/webhooks/${id}`),
  logs: (id: number, limit = 50) => get<{ data: WebhookLog[] }>(`/webhooks/${id}/logs`, { limit }),
}

export const searchApi = {
  admin: (params: { q: string; type?: string; status?: string; page?: number; page_size?: number }) =>
    get<{ data: SearchResult }>('/search/admin', params),
  reindex: () => post<{ data: { indexed: number } }>('/search/reindex'),
}
