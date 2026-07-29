import { get, post, put, del } from '../http'
import type { Role, Permission } from './auth'

// ─── API ─────────────────────────────────────────────────

export const roleApi = {
  list: () => get<{ data: Role[] }>('/roles'),
  create: (data: Partial<Role>) => post('/roles', data),
  update: (id: number, data: Partial<Role>) => put(`/roles/${id}`, data),
  delete: (id: number) => del(`/roles/${id}`),
  // Backend shape: envelope.data = { data: Permission[], grouped } — unwrap.
  permissions: () =>
    get<{ data: { data: Permission[]; grouped: Record<string, Permission[]> } }>('/roles/permissions').then(r => r.data),
}
