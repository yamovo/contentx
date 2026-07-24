import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, type User, type TokenPair } from '@/api'

export const useAuthStore = defineStore('auth', () => {
  // State.
  const user = ref<User | null>(null)
  const token = ref<string>(localStorage.getItem('access_token') || '')
  const refreshToken = ref<string>(localStorage.getItem('refresh_token') || '')
  const permissions = ref<string[]>([])
  const loading = ref(false)

  // Getters.
  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role?.slug === 'admin')
  const isEditor = computed(() => ['admin', 'editor'].includes(user.value?.role?.slug || ''))

  // Actions.
  async function login(username: string, password: string) {
    loading.value = true
    try {
      const res = await authApi.login({ username, password })
      setTokens(res.data.token)
      user.value = res.data.user
      await fetchPermissions()
      return res
    } finally {
      loading.value = false
    }
  }

  async function register(data: { username: string; email: string; password: string; display_name?: string }) {
    loading.value = true
    try {
      const res = await authApi.register(data)
      setTokens(res.data.token)
      user.value = res.data.user
      await fetchPermissions()
      return res
    } finally {
      loading.value = false
    }
  }

  async function fetchUser() {
    if (!token.value) return
    try {
      const res = await authApi.me()
      user.value = res.data.user
      permissions.value = res.data.permissions
    } catch {
      clearAuth()
    }
  }

  async function fetchPermissions() {
    try {
      const res = await authApi.me()
      user.value = res.data.user
      permissions.value = res.data.permissions
    } catch {
      // Ignore.
    }
  }

  async function refreshAccessToken() {
    if (!refreshToken.value) throw new Error('No refresh token')
    const res = await authApi.refresh(refreshToken.value)
    setTokens(res.data)
    return res.data.access_token
  }

  function setTokens(pair: TokenPair) {
    token.value = pair.access_token
    refreshToken.value = pair.refresh_token
    localStorage.setItem('access_token', pair.access_token)
    localStorage.setItem('refresh_token', pair.refresh_token)
  }

  // clearAuth clears local auth state only (no backend call). Used when the
  // token is already known to be invalid (e.g. 401 after failed refresh).
  function clearAuth() {
    user.value = null
    token.value = ''
    refreshToken.value = ''
    permissions.value = []
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
  }

  // logout invalidates the session on the backend (blacklist the refresh
  // token) and then clears local state. Network failures are swallowed so
  // the user is still logged out locally (A-3 fix).
  async function logout() {
    try {
      if (refreshToken.value) {
        await authApi.logout(refreshToken.value)
      }
    } catch {
      // Best-effort: ignore network errors so local state is still cleared.
    } finally {
      clearAuth()
    }
  }

  function hasPermission(slug: string): boolean {
    if (isAdmin.value) return true
    return permissions.value.includes(slug)
  }

  // Initialize: fetch user if token exists.
  if (token.value) {
    fetchUser()
  }

  return {
    user, token, refreshToken, permissions, loading,
    isAuthenticated, isAdmin, isEditor,
    login, register, fetchUser, fetchPermissions, refreshAccessToken,
    setTokens, logout, clearAuth, hasPermission,
  }
})
