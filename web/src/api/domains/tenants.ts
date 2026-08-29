import { get, post, put, del } from '../http'

// ─── Types ───────────────────────────────────────────────

export type TenantStatus = 'active' | 'suspended'

export interface Tenant {
  id: number
  name: string
  slug: string
  status: TenantStatus
  max_users: number
  created_at?: string
  updated_at?: string
}

export type TenantRole = 'admin' | 'editor' | 'member'

export interface TenantMember {
  tenant_id: number
  user_id: number
  role_slug: TenantRole
  joined_at: string
  username: string
  email: string
  display_name: string
  user_status: string
}

// ─── Tenant administration (RFC-001 PR-5) ────────────────

// http helpers resolve to the raw envelope {code, message, data}; the domain
// types the envelope and callers read `.data` (same convention as menus.ts).
export const tenantsApi = {
  list: () => get<{ data: Tenant[] }>('/admin/tenants'),

  get: (id: number) => get<{ data: Tenant }>(`/admin/tenants/${id}`),

  create: (data: { name: string; slug: string; max_users?: number }) =>
    post<{ data: Tenant }>('/admin/tenants', data),

  update: (id: number, data: { name?: string; status?: TenantStatus; max_users?: number }) =>
    put<{ data: Tenant }>(`/admin/tenants/${id}`, data),

  listMembers: (id: number) => get<{ data: TenantMember[] }>(`/admin/tenants/${id}/members`),

  addMember: (id: number, data: { user_id: number; role_slug: TenantRole }) =>
    post<{ data: TenantMember }>(`/admin/tenants/${id}/members`, data),

  updateMemberRole: (id: number, userId: number, role_slug: TenantRole) =>
    put<{ data: { message: string } }>(`/admin/tenants/${id}/members/${userId}`, { role_slug }),

  removeMember: (id: number, userId: number) =>
    del<{ data: { message: string } }>(`/admin/tenants/${id}/members/${userId}`),
}
