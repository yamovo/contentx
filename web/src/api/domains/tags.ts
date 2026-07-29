import { get, post, put, del } from '../http'

// ─── Types ───────────────────────────────────────────────

export interface Tag {
  id: number
  name: string
  slug: string
  count: number
  color: string
}

// ─── API ─────────────────────────────────────────────────

export const tagApi = {
  // Backend shape: envelope.data = { data: Tag[], total } — unwrap one level
  // so views keep reading `res.data` / `res.total`.
  list: (params?: Record<string, unknown>) =>
    get<{ data: { data: Tag[]; total: number } }>('/tags', params).then(r => r.data),
  get: (id: number) => get<{ data: Tag }>(`/tags/${id}`),
  create: (data: Partial<Tag>) => post<{ data: Tag }>('/tags', data),
  update: (id: number, data: Partial<Tag>) => put(`/tags/${id}`, data),
  delete: (id: number) => del(`/tags/${id}`),
  merge: (data: { source_ids: number[]; target_id: number; delete_old: boolean }) =>
    post('/tags/merge', data),
}
