import { get, post, put, del } from '../http'

// ─── Types ───────────────────────────────────────────────

export interface Category {
  id: number
  name: string
  slug: string
  description: string
  parent_id: number | null
  children?: Category[]
  image: string
  color: string
  sort_order: number
  post_count: number
  is_active: boolean
}

// ─── API ─────────────────────────────────────────────────

export const categoryApi = {
  list: (params?: Record<string, unknown>) => get<{ data: Category[] }>('/categories', params),
  get: (id: number) => get<{ data: Category }>(`/categories/${id}`),
  create: (data: Partial<Category>) => post<{ data: Category }>('/categories', data),
  update: (id: number, data: Partial<Category>) => put(`/categories/${id}`, data),
  delete: (id: number) => del(`/categories/${id}`),
  reorder: (items: { id: number; sort_order: number; parent_id?: number }[]) =>
    put('/categories/reorder', { items }),
}
