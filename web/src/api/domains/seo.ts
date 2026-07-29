import { get, post, put, del } from '../http'

// ─── Types ───────────────────────────────────────────────

export interface Redirect {
  id: number
  from_path: string
  to_path: string
  status_code: number
  hit_count: number
  is_active: boolean
  note: string
}

// ─── API ─────────────────────────────────────────────────

export const seoApi = {
  getSetting: (type: string, id: number) => get(`/seo/${type}/${id}`),
  updateSetting: (type: string, id: number, data: Record<string, unknown>) => put(`/seo/${type}/${id}`, data),
  sitemap: () => get('/seo/sitemap'),
  robotsTxt: () => get('/seo/robots.txt'),
  listRedirects: () => get<{ data: Redirect[] }>('/seo/redirects'),
  createRedirect: (data: Partial<Redirect>) => post('/seo/redirects', data),
  deleteRedirect: (id: number) => del(`/seo/redirects/${id}`),
}
