import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Controllable auth state — the guard reads isAuthenticated and hasPermission.
const mockAuth = {
  isAuthenticated: false,
  hasPermission: vi.fn(() => true),
}

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => mockAuth,
}))

// nprogress is imported for side effects (CSS); stub it out.
vi.mock('nprogress', () => ({
  default: {
    configure: vi.fn(),
    start: vi.fn(),
    done: vi.fn(),
  },
}))

// Stub every lazy-loaded view so the dynamic imports don't pull in real
// components (and their transitive deps) during guard tests. The guard only
// cares about route records, not the rendered output. vi.hoisted runs the
// factory before vi.mock calls are hoisted, so the reference is safe.
const { stubView } = vi.hoisted(() => ({
  // Dynamic imports expect a module with a `default` export.
  stubView: () => ({ default: { template: '<div/>' } }),
}))
vi.mock('@/layouts/AdminLayout.vue', stubView)
vi.mock('@/views/login/LoginView.vue', stubView)
vi.mock('@/views/login/RegisterView.vue', stubView)
vi.mock('@/views/dashboard/HomeView.vue', stubView)
vi.mock('@/views/dashboard/DashboardView.vue', stubView)
vi.mock('@/views/blog/BlogLayout.vue', stubView)
vi.mock('@/views/blog/BlogList.vue', stubView)
vi.mock('@/views/blog/BlogArticle.vue', stubView)
vi.mock('@/views/articles/ArticleList.vue', stubView)
vi.mock('@/views/articles/ArticleEditor.vue', stubView)
vi.mock('@/views/articles/ArticleRevisions.vue', stubView)
vi.mock('@/views/categories/CategoryList.vue', stubView)
vi.mock('@/views/tags/TagList.vue', stubView)
vi.mock('@/views/comments/CommentList.vue', stubView)
vi.mock('@/views/media/MediaLibrary.vue', stubView)
vi.mock('@/views/users/UserList.vue', stubView)
vi.mock('@/views/users/UserDetail.vue', stubView)
vi.mock('@/views/roles/RoleList.vue', stubView)
vi.mock('@/views/settings/MenuManager.vue', stubView)
vi.mock('@/views/seo/SEOManager.vue', stubView)
vi.mock('@/views/seo/RedirectManager.vue', stubView)
vi.mock('@/views/analytics/AnalyticsView.vue', stubView)
vi.mock('@/views/plugins/PluginList.vue', stubView)
vi.mock('@/views/themes/ThemeList.vue', stubView)
vi.mock('@/views/settings/SettingsView.vue', stubView)
vi.mock('@/views/settings/ActivityLog.vue', stubView)
vi.mock('@/views/shared/NotFound.vue', stubView)

import router from '@/router'

// jsdom doesn't implement window.scrollTo; the router's scrollBehavior calls
// it on navigation. Stub it to keep test stderr clean.
if (typeof window !== 'undefined' && !window.scrollTo) {
  window.scrollTo = () => {}
}

describe('router guards', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    mockAuth.isAuthenticated = false
    mockAuth.hasPermission = vi.fn(() => true)

    // Reset to a clean location before each test. The router uses
    // createWebHistory, so we go back to '/' to start fresh.
    await router.push('/')
    await router.isReady()
  })

  it('redirects unauthenticated users from /admin/* to /login with redirect query', async () => {
    mockAuth.isAuthenticated = false

    await router.push('/admin/articles')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('Login')
    expect(router.currentRoute.value.query.redirect).toBe('/admin/articles')
  })

  it('redirects authenticated users away from guest routes (/login) to AdminDashboard', async () => {
    mockAuth.isAuthenticated = true

    await router.push('/login')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('AdminDashboard')
  })

  it('redirects authenticated users away from /register to AdminDashboard', async () => {
    mockAuth.isAuthenticated = true

    await router.push('/register')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('AdminDashboard')
  })

  it('redirects to AdminDashboard when authenticated but missing required permission', async () => {
    mockAuth.isAuthenticated = true
    mockAuth.hasPermission = vi.fn(() => false)

    await router.push('/admin/articles')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('AdminDashboard')
    // hasPermission should have been checked against 'articles.view'
    expect(mockAuth.hasPermission).toHaveBeenCalledWith('articles.view')
  })

  it('allows access to admin routes when authenticated with permission', async () => {
    mockAuth.isAuthenticated = true
    mockAuth.hasPermission = vi.fn(() => true)

    await router.push('/admin/articles')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('ArticleList')
  })

  it('allows unauthenticated access to public routes (/)', async () => {
    mockAuth.isAuthenticated = false

    await router.push('/')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('Home')
  })

  it('allows unauthenticated access to guest routes (/login)', async () => {
    mockAuth.isAuthenticated = false

    await router.push('/login')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('Login')
  })

  it('does not check permission when route has no permission meta', async () => {
    mockAuth.isAuthenticated = true

    // /admin/articles/:id/revisions has no permission meta
    await router.push('/admin/articles/1/revisions')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('ArticleRevisions')
    expect(mockAuth.hasPermission).not.toHaveBeenCalled()
  })
})
