import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { reactive } from 'vue'

// Mock localStorage.
let mockStore: Record<string, string> = {}
const localStorageMock = {
  getItem: vi.fn((key: string) => mockStore[key] || null),
  setItem: vi.fn((key: string, value: string) => { mockStore[key] = value }),
  removeItem: vi.fn((key: string) => { delete mockStore[key] }),
  clear: vi.fn(() => { mockStore = {} }),
  get length() { return Object.keys(mockStore).length },
  key: vi.fn((index: number) => Object.keys(mockStore)[index] || null),
}

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock })

vi.mock('@/api/domains/tenants', () => ({
  tenantsApi: {
    list: vi.fn(),
    get: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    listMembers: vi.fn(),
    addMember: vi.fn(),
    updateMemberRole: vi.fn(),
    removeMember: vi.fn(),
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn(),
}))

import { useTenantStore } from '@/stores/tenant'
import { tenantsApi } from '@/api/domains/tenants'
import { useAuthStore } from '@/stores/auth'

const mockedTenantsApi = vi.mocked(tenantsApi)
const mockedUseAuthStore = vi.mocked(useAuthStore)

// The store reads the auth state inside a computed, so the mock must be a
// reactive object (like the real Pinia store) for the computed to re-evaluate.
const authState = reactive({ isAdmin: false })

function mockAuth(isAdmin: boolean) {
  authState.isAdmin = isAdmin
  mockedUseAuthStore.mockReturnValue(authState as ReturnType<typeof useAuthStore>)
}

const tenantRows = [
  { id: 1, name: 'Default', slug: 'default', status: 'active' as const, max_users: 0 },
  { id: 2, name: 'Acme', slug: 'acme', status: 'active' as const, max_users: 25 },
  { id: 3, name: 'Gone', slug: 'gone', status: 'suspended' as const, max_users: 0 },
]

describe('tenant store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockStore = {}
    vi.clearAllMocks()
  })

  it('loads tenants only for platform administrators', async () => {
    mockAuth(false)
    let store = useTenantStore()
    await store.loadTenants()
    expect(mockedTenantsApi.list).not.toHaveBeenCalled()
    expect(store.tenants).toEqual([])

    mockAuth(true)
    store = useTenantStore()
    mockedTenantsApi.list.mockResolvedValueOnce({ data: tenantRows })
    await store.loadTenants()
    expect(mockedTenantsApi.list).toHaveBeenCalledTimes(1)
    expect(store.tenants).toEqual(tenantRows)
    expect(store.canSwitch).toBe(true)
  })

  it('persists the selection and clears a stale one on reload', async () => {
    mockAuth(true)
    mockedTenantsApi.list.mockResolvedValue({ data: tenantRows })
    const store = useTenantStore()

    store.switchTo(2)
    expect(mockStore['current_tenant_id']).toBe('2')
    expect(store.currentTenantID).toBe(2)

    // After the list loads, the selection resolves to the tenant.
    await store.loadTenants()
    expect(store.currentTenant?.slug).toBe('acme')
    expect(store.currentTenantID).toBe(2)

    // The selected tenant disappears from the platform: fall back to default.
    mockedTenantsApi.list.mockResolvedValueOnce({
      data: [tenantRows[0], tenantRows[2]],
    })
    await store.loadTenants()
    expect(store.currentTenantID).toBeNull()
    expect(mockStore['current_tenant_id']).toBeUndefined()
  })

  it('switching to the default tenant clears the stored selection', () => {
    mockAuth(true)
    const store = useTenantStore()
    store.switchTo(2)
    store.switchTo(null)
    expect(store.currentTenantID).toBeNull()
    expect(mockStore['current_tenant_id']).toBeUndefined()
  })
})
