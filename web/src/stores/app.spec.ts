import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock localStorage.
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: vi.fn((key: string) => store[key] || null),
    setItem: vi.fn((key: string, value: string) => { store[key] = value }),
    removeItem: vi.fn((key: string) => { delete store[key] }),
    clear: vi.fn(() => { store = {} }),
    get length() { return Object.keys(store).length },
    key: vi.fn((index: number) => Object.keys(store)[index] || null),
  }
})()

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock })

// Mock the api module.
vi.mock('@/api', () => ({
  settingsApi: {
    public: vi.fn(),
  },
}))

import { useAppStore } from '@/stores/app'
import { settingsApi } from '@/api'

describe('app store', () => {
  beforeEach(() => {
    localStorageMock.clear()
    vi.clearAllMocks()
    document.documentElement.removeAttribute('data-theme')
    document.documentElement.classList.remove('dark')
    setActivePinia(createPinia())
  })

  describe('initial state', () => {
    it('has correct defaults', () => {
      const store = useAppStore()
      expect(store.sidebarCollapsed).toBe(false)
      expect(store.theme).toBe('light')
      expect(store.language).toBe('zh')
      expect(store.loading).toBe(false)
      expect(store.settings).toEqual({})
    })

    it('restores sidebar state from localStorage', () => {
      localStorageMock.setItem('sidebar_collapsed', 'true')
      const store = useAppStore()
      expect(store.sidebarCollapsed).toBe(true)
    })

    it('restores theme from localStorage', () => {
      localStorageMock.setItem('theme', 'dark')
      const store = useAppStore()
      expect(store.theme).toBe('dark')
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })
  })

  describe('toggleSidebar', () => {
    it('flips collapsed state and persists to localStorage', () => {
      const store = useAppStore()
      expect(store.sidebarCollapsed).toBe(false)

      store.toggleSidebar()
      expect(store.sidebarCollapsed).toBe(true)
      expect(localStorageMock.getItem('sidebar_collapsed')).toBe('true')

      store.toggleSidebar()
      expect(store.sidebarCollapsed).toBe(false)
      expect(localStorageMock.getItem('sidebar_collapsed')).toBe('false')
    })
  })

  describe('theme', () => {
    it('setTheme applies DOM attributes and persists', () => {
      const store = useAppStore()
      store.setTheme('dark')

      expect(store.theme).toBe('dark')
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
      expect(document.documentElement.classList.contains('dark')).toBe(true)
      expect(localStorageMock.getItem('theme')).toBe('dark')
    })

    it('setTheme light removes dark class', () => {
      const store = useAppStore()
      store.setTheme('dark')
      store.setTheme('light')

      expect(store.theme).toBe('light')
      expect(document.documentElement.classList.contains('dark')).toBe(false)
    })

    it('toggleTheme switches between light and dark', () => {
      const store = useAppStore()
      expect(store.theme).toBe('light')

      store.toggleTheme()
      expect(store.theme).toBe('dark')

      store.toggleTheme()
      expect(store.theme).toBe('light')
    })
  })

  describe('fetchPublicSettings', () => {
    it('populates settings on success', async () => {
      const mockSettings = { site_title: 'My Site', site_desc: 'Welcome' }
      vi.mocked(settingsApi.public).mockResolvedValue({ data: mockSettings } as any)

      const store = useAppStore()
      await store.fetchPublicSettings()

      expect(settingsApi.public).toHaveBeenCalledOnce()
      expect(store.settings).toEqual(mockSettings)
    })

    it('silently ignores API failure', async () => {
      vi.mocked(settingsApi.public).mockRejectedValue(new Error('network'))

      const store = useAppStore()
      await expect(store.fetchPublicSettings()).resolves.toBeUndefined()
      expect(store.settings).toEqual({})
    })
  })
})
