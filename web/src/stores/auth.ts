import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, type User, type TokenPair } from '@/api/domains/auth'
import {
  ADMIN_WORKSPACE_PERMISSIONS,
  isPermissionSlug,
  type PermissionSlug,
} from '@/shared/auth/permissions'
import { isApiError } from '@/shared/api/types'

export const useAuthStore = defineStore('auth', () => {
  // State.
  const user = ref<User | null>(null)
  const token = ref<string>(localStorage.getItem('access_token') || '')
  const refreshToken = ref<string>(localStorage.getItem('refresh_token') || '')
  const permissions = ref<PermissionSlug[]>([])
  const loading = ref(false)
  const restoreError = ref<unknown>(null)

  // Getters.
  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role?.slug === 'admin')
  const isEditor = computed(() => ['admin', 'editor'].includes(user.value?.role?.slug || ''))
  const canAccessAdmin = computed(() => {
    if (isAdmin.value) return true
    if (!user.value || user.value.role?.slug === 'subscriber') return false
    return ADMIN_WORKSPACE_PERMISSIONS.some((slug) => permissions.value.includes(slug))
  })

  function setSessionPermissions(values: string[]) {
    permissions.value = values.filter(isPermissionSlug)
  }

  // Actions.
  async function login(username: string, password: string, totpCode?: string) {
    loading.value = true
    try {
      const res = await authApi.login({ username, password, totp_code: totpCode })
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
    restoreError.value = null
    try {
      const res = await authApi.me()
      user.value = res.data.user
      setSessionPermissions(res.data.permissions)
    } catch (error) {
      if (isApiError(error) && error.status === 401) {
        clearAuth()
        return
      }
      restoreError.value = error
      throw error
    }
  }

  // In-flight /me request, shared so concurrent callers (store init + router
  // guard) don't fire duplicate requests.
  let userPromise: Promise<void> | null = null

  // ensureUserLoaded resolves once user/permissions are available. The router
  // guard awaits this before permission checks: on a hard refresh the store
  // init fetch is still in flight, and checking permissions synchronously
  // would always fail and bounce the user back to the dashboard.
  function ensureUserLoaded(): Promise<void> {
    if (!token.value || user.value) return Promise.resolve()
    if (!userPromise) {
      userPromise = fetchUser().finally(() => {
        userPromise = null
      })
    }
    return userPromise
  }

  async function fetchPermissions() {
    const res = await authApi.me()
    user.value = res.data.user
    setSessionPermissions(res.data.permissions)
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
    restoreError.value = null
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

  function hasPermission(slug: PermissionSlug): boolean {
    if (isAdmin.value) return true
    return permissions.value.includes(slug)
  }

  function hasAnyPermission(slugs: readonly PermissionSlug[]): boolean {
    if (isAdmin.value) return true
    return slugs.some((slug) => permissions.value.includes(slug))
  }

  // Initialize: fetch user if token exists.
  if (token.value) {
    ensureUserLoaded()
  }

  return {
    user, token, refreshToken, permissions, loading, restoreError,
    isAuthenticated, isAdmin, isEditor, canAccessAdmin,
    login, register, fetchUser, fetchPermissions, refreshAccessToken,
    ensureUserLoaded, setTokens, logout, clearAuth, hasPermission, hasAnyPermission,
  }
})
