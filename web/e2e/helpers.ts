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
export async function mockAuthMe(page: Page) {
  await page.route(/\/api\/v1\/auth\/me$/, (route) =>
    fulfil(route, { data: { user: adminUser, permissions: ['*'] } }),
  )
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
