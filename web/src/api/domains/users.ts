import { get, post, put, del } from '../http'
import { getList } from '../helpers'
import type { User } from './auth'

// ─── API ─────────────────────────────────────────────────

export const userApi = {
  list: (params?: Record<string, unknown>) => getList<User>('/users', params),
  get: (id: number) => get<{ data: User }>(`/users/${id}`),
  create: (data: Partial<User> & { password: string }) => post<{ data: User }>('/users', data),
  update: (id: number, data: Partial<User>) => put(`/users/${id}`, data),
  delete: (id: number) => del(`/users/${id}`),
  resetPassword: (id: number, new_password: string) =>
    post(`/users/${id}/reset-password`, { new_password }),
}
