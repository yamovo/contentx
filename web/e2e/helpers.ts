import type { Page, Route } from '@playwright/test'

/**
 * Shared E2E helpers.
 *
 * The suite intercepts all /api/v1/* requests so tests run without a backend.
 * Each helper mocks a specific API endpoint with a realistic response shape
 * matching the real API contract (see web/src/api/index.ts).
 *
 * Routes use RegExp (not glob) for the URL pattern: RegExp matching is
 * unambiguous and avoids the precedence pitfalls of glob `**` patterns where
 * a broader fallback can shadow a specific path.
 */

// ─── Mock data ────────────────────────────────────────────────────────────

const adminUser = {
  id: 1,
  username: 'admin',
  email: 'admin@example.com',
  display_name: 'Admin',
  avatar: '',
  bio: '',
  website: '',
  role: {
    id: 1,
    name: 'Administrator',
    slug: 'admin',
    description: 'Super admin',
    permissions: [],
    is_system: true,
  },
  status: 'active',
  login_count: 1,
  preferences: {
    language: 'zh-cn',
    theme: 'light',
    email_notify: true,
    markdown_editor: true,
    items_per_page: 10,
    default_post_status: 'draft',
  },
  created_at: '2026-01-01T00:00:00Z',
}

const tokenPair = {
  access_token: 'mock-access-token',
  refresh_token: 'mock-refresh-token',
  token_type: 'bearer',
  expires_at: '2026-12-31T00:00:00Z',
  expires_in: 3600,
}

// ─── Route handlers ───────────────────────────────────────────────────────

/** Fulfil a route with a JSON body (200 by default). */
function fulfil(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

/**
 * Mock the /api/v1/auth/login endpoint. Returns a token pair + admin user.
 */
export async function mockLoginSuccess(page: Page) {
  await page.route(/\/api\/v1\/auth\/login$/, (route) =>
    fulfil(route, { data: { token: tokenPair, user: adminUser } }),
  )
}

/**
 * Mock the /api/v1/auth/login endpoint to fail (bad credentials).
 */
export async function mockLoginFailure(page: Page) {
  await page.route(/\/api\/v1\/auth\/login$/, (route) =>
    fulfil(route, { error: { code: 'invalid_credentials', message: '用户名或密码错误' } }, 401),
  )
}

/**
 * Mock the /api/v1/auth/me endpoint (used by fetchUser/fetchPermissions after
 * login and on page reload when a token is present).
 */
export async function mockAuthMe(page: Page, installSession = true) {
  if (installSession) {
    await installMockSession(page)
  }
  await page.route(/\/api\/v1\/auth\/me$/, (route) =>
    fulfil(route, { data: { user: adminUser, permissions: ['*'] } }),
  )
}

async function installMockSession(page: Page) {
  await page.addInitScript(({ accessToken, refreshToken }) => {
    localStorage.setItem('access_token', accessToken)
    localStorage.setItem('refresh_token', refreshToken)
  }, {
    accessToken: tokenPair.access_token,
    refreshToken: tokenPair.refresh_token,
  })
}

/**
 * Mock the /api/v1/auth/logout endpoint.
 */
export async function mockLogout(page: Page) {
  await page.route(/\/api\/v1\/auth\/logout$/, (route) => fulfil(route, { data: null }))
}

/**
 * Fallback: respond 404 to any unmocked /api/v1/* call so a missing mock
 * surfaces as a clear failure instead of hitting the (absent) backend.
 *
 * Playwright dispatches route handlers last-registered-first, so this MUST be
 * registered BEFORE the specific endpoint mocks in each test's beforeEach.
 */
export async function mockUnmockedApiAs404(page: Page) {
  await page.route(/\/api\/v1\//, (route) => {
    return fulfil(route, { error: { code: 'unmocked', message: 'unmocked API call' } }, 404)
  })
}

// ─── Article / taxonomy mocks ────────────────────────────────────────────

export interface MockArticle {
  id: number
  title: string
  slug: string
  status: 'draft' | 'published' | 'archived'
  post_type: 'post' | 'page'
  summary?: string
  content?: string
  author_id?: number
  category_id?: number
  tags?: { id: number; name: string }[]
  created_at?: string
  updated_at?: string
}

const defaultMeta = (total: number, page = 1, pageSize = 20) => ({
  page,
  page_size: pageSize,
  total,
  total_pages: Math.max(1, Math.ceil(total / pageSize)),
  has_next: false,
  has_prev: false,
})

/**
 * Mock the article list endpoint. Supports filtering by post_type via the
 * route URL query string (the real API uses ?post_type=post|page).
 */
export async function mockArticleList(page: Page, articles: MockArticle[]) {
  await page.route(/\/api\/v1\/articles/, (route) => {
    return fulfil(route, { data: { items: articles, ...defaultMeta(articles.length) } })
  })
}

/** Mock GET /api/v1/articles/:id */
export async function mockArticleDetail(page: Page, article: MockArticle) {
  await page.route(/\/api\/v1\/articles\/\d+$/, (route) =>
    fulfil(route, { data: article }),
  )
}

/** Mock POST /api/v1/articles (create) */
export async function mockArticleCreate(page: Page, created: Partial<MockArticle> = {}) {
  await page.route(/\/api\/v1\/articles$/, (route) =>
    fulfil(route, { data: { id: 999, ...created } }, 201),
  )
}

/** Mock PUT /api/v1/articles/:id (update) */
export async function mockArticleUpdate(page: Page, updated: Partial<MockArticle> = {}) {
  await page.route(/\/api\/v1\/articles\/\d+$/, (route) =>
    fulfil(route, { data: updated }),
  )
}

/** Mock DELETE /api/v1/articles/:id */
export async function mockArticleDelete(page: Page) {
  await page.route(/\/api\/v1\/articles\/\d+$/, (route) =>
    fulfil(route, { data: null }),
  )
}

/** Mock PATCH /api/v1/articles/:id/status (publish / archive etc.) */
export async function mockArticleStatusChange(page: Page) {
  await page.route(/\/api\/v1\/articles\/\d+\/status$/, (route) =>
    fulfil(route, { data: null }),
  )
}

/** Mock the categories list (needed by article editor). */
export async function mockCategoryList(page: Page, categories: { id: number; name: string; slug: string }[] = []) {
  await page.route(/\/api\/v1\/categories/, (route) =>
    fulfil(route, { data: { items: categories, ...defaultMeta(categories.length, 1, 100) } }),
  )
}

/** Mock the tags list (needed by article editor). */
export async function mockTagList(page: Page, tags: { id: number; name: string; slug: string }[] = []) {
  await page.route(/\/api\/v1\/tags/, (route) =>
    fulfil(route, { data: { items: tags, ...defaultMeta(tags.length, 1, 100) } }),
  )
}

// ─── Role-based auth mocks ───────────────────────────────────────────────

type RoleSlug = 'admin' | 'editor' | 'author' | 'subscriber'

const rolePermissions: Record<RoleSlug, string[]> = {
  admin: ['*'],
  editor: [
    'articles.read', 'articles.create', 'articles.update', 'articles.publish',
    'comments.read', 'comments.moderate',
  ],
  author: [
    'articles.read', 'articles.create', 'articles.update',
    'comments.read',
  ],
  subscriber: ['comments.read'],
}

/**
 * Mock /api/v1/auth/me with a specific role and its default permission set.
 */
export async function mockAuthMeAs(page: Page, role: RoleSlug) {
  await installMockSession(page)
  const user = {
    ...adminUser,
    role: {
      id: role === 'admin' ? 1 : role === 'editor' ? 2 : role === 'author' ? 3 : 4,
      name: role.charAt(0).toUpperCase() + role.slice(1),
      slug: role,
      description: role,
      permissions: rolePermissions[role],
      is_system: role === 'admin',
    },
  }
  await page.route(/\/api\/v1\/auth\/me$/, (route) =>
    fulfil(route, { data: { user, permissions: rolePermissions[role] } }),
  )
}
