import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

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

// Mock the api module.
vi.mock('@/api', () => ({
  authApi: {
    login: vi.fn(),
    register: vi.fn(),
    me: vi.fn(),
    refresh: vi.fn(),
  },
}))

// Import after mocking.
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'

const mockUser = {
  id: 1,
  username: 'testuser',
  email: 'test@example.com',
  display_name: 'Test User',
  avatar: '',
  bio: '',
  website: '',
  role: { id: 1, name: 'Admin', slug: 'admin', description: '', permissions: [], is_system: true },
  status: 'active',
  login_count: 0,
  preferences: {
    language: 'zh',
    theme: 'light',
    email_notify: true,
    markdown_editor: true,
    items_per_page: 20,
    default_post_status: 'draft',
  },
  created_at: '2024-01-01T00:00:00Z',
}

const mockTokenPair = {
  access_token: 'access-jwt-token',
  refresh_token: 'refresh-jwt-token',
  token_type: 'bearer',
  expires_at: '2024-01-02T00:00:00Z',
  expires_in: 86400,
}

describe('Auth Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockStore = {}
    vi.clearAllMocks()
    // Restore the default getItem implementation (some tests override it).
    localStorageMock.getItem.mockImplementation((key: string) => mockStore[key] || null)
  })

  describe('initial state', () => {
    it('should start unauthenticated when no token in localStorage', () => {
      const store = useAuthStore()
      expect(store.isAuthenticated).toBe(false)
      expect(store.user).toBeNull()
      expect(store.token).toBe('')
      expect(store.refreshToken).toBe('')
      expect(store.permissions).toEqual([])
    })

    it('should restore token from localStorage', () => {
      localStorageMock.getItem.mockImplementation((key: string) => {
        if (key === 'access_token') return 'stored-token'
        if (key === 'refresh_token') return 'stored-refresh'
        return null
      })

      const store = useAuthStore()
      expect(store.token).toBe('stored-token')
      expect(store.refreshToken).toBe('stored-refresh')
      expect(store.isAuthenticated).toBe(true)
    })
  })

  describe('computed properties', () => {
    it('isAdmin should return true for admin role', () => {
      const store = useAuthStore()
      store.user = mockUser
      expect(store.isAdmin).toBe(true)
    })

    it('isAdmin should return false for non-admin role', () => {
      const store = useAuthStore()
      store.user = { ...mockUser, role: { ...mockUser.role, slug: 'editor' } }
      expect(store.isAdmin).toBe(false)
    })

    it('isEditor should return true for editor and admin roles', () => {
      const store = useAuthStore()
      store.user = mockUser // admin
      expect(store.isEditor).toBe(true)

      store.user = { ...mockUser, role: { ...mockUser.role, slug: 'editor' } }
      expect(store.isEditor).toBe(true)
    })

    it('isEditor should return false for subscriber role', () => {
      const store = useAuthStore()
      store.user = { ...mockUser, role: { ...mockUser.role, slug: 'subscriber' } }
      expect(store.isEditor).toBe(false)
    })
  })

  describe('logout', () => {
    it('should clear all state and localStorage', () => {
      const store = useAuthStore()
      store.user = mockUser
      store.token = 'some-token'
      store.refreshToken = 'some-refresh'
      store.permissions = ['articles.create']

      store.logout()

      expect(store.user).toBeNull()
      expect(store.token).toBe('')
      expect(store.refreshToken).toBe('')
      expect(store.permissions).toEqual([])
      expect(localStorageMock.removeItem).toHaveBeenCalledWith('access_token')
      expect(localStorageMock.removeItem).toHaveBeenCalledWith('refresh_token')
    })
  })

  describe('setTokens', () => {
    it('should set tokens in state and localStorage', () => {
      const store = useAuthStore()
      store.setTokens(mockTokenPair)

      expect(store.token).toBe('access-jwt-token')
      expect(store.refreshToken).toBe('refresh-jwt-token')
      expect(localStorageMock.setItem).toHaveBeenCalledWith('access_token', 'access-jwt-token')
      expect(localStorageMock.setItem).toHaveBeenCalledWith('refresh_token', 'refresh-jwt-token')
    })
  })

  describe('hasPermission', () => {
    it('admin should have all permissions', () => {
      const store = useAuthStore()
      store.user = mockUser // admin
      expect(store.hasPermission('articles.create')).toBe(true)
      expect(store.hasPermission('users.delete')).toBe(true)
    })

    it('non-admin should check permissions list', () => {
      const store = useAuthStore()
      store.user = { ...mockUser, role: { ...mockUser.role, slug: 'editor' } }
      store.permissions = ['articles.create', 'articles.edit']

      expect(store.hasPermission('articles.create')).toBe(true)
      expect(store.hasPermission('articles.edit')).toBe(true)
      expect(store.hasPermission('users.delete')).toBe(false)
    })
  })

  describe('login', () => {
    it('should set tokens and user on successful login', async () => {
      vi.mocked(authApi.login).mockResolvedValueOnce({
        data: { token: mockTokenPair, user: mockUser },
      } as any)

      vi.mocked(authApi.me).mockResolvedValueOnce({
        data: { user: mockUser, permissions: ['articles.create'] },
      } as any)

      const store = useAuthStore()
      const result = await store.login('testuser', 'password123')

      expect(authApi.login).toHaveBeenCalledWith({ username: 'testuser', password: 'password123' })
      expect(store.token).toBe('access-jwt-token')
      expect(store.user).toEqual(mockUser)
      expect(store.loading).toBe(false)
    })

    it('should reset loading state on failed login', async () => {
      vi.mocked(authApi.login).mockRejectedValueOnce(new Error('Invalid credentials'))

      const store = useAuthStore()
      await expect(store.login('bad', 'creds')).rejects.toThrow('Invalid credentials')
      expect(store.loading).toBe(false)
    })
  })

  describe('refreshAccessToken', () => {
    it('should throw when no refresh token available', async () => {
      const store = useAuthStore()
      await expect(store.refreshAccessToken()).rejects.toThrow('No refresh token')
    })

    it('should return new access token on success', async () => {
      const store = useAuthStore()
      store.refreshToken = 'old-refresh-token'

      vi.mocked(authApi.refresh).mockResolvedValueOnce({
        data: { ...mockTokenPair, access_token: 'new-access-token' },
      } as any)

      const newToken = await store.refreshAccessToken()
      expect(newToken).toBe('new-access-token')
      expect(store.token).toBe('new-access-token')
    })
  })
})
