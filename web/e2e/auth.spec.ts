import { test, expect } from '@playwright/test'
import {
  mockAuthMe,
  mockLoginFailure,
  mockLoginSuccess,
  mockLogout,
  mockUnmockedApiAs404,
} from './helpers'

/**
 * E2E tests for the login → admin flow.
 *
 * The backend is fully mocked via page.route:
 *   - POST /api/v1/auth/login  → token pair + user
 *   - GET  /api/v1/auth/me     → user + permissions (called after login)
 *   - POST /api/v1/auth/logout → ok
 *
 * These tests verify the real SPA navigation guard + store + form behaviour
 * end-to-end in a real browser, not a jsdom unit test.
 */

test.describe('Login flow', () => {
  test.beforeEach(async ({ page }) => {
    // Playwright routes are dispatched last-registered-first. Register the
    // 404 fallback FIRST so the specific mocks below take precedence.
    await mockUnmockedApiAs404(page)
    await mockLoginSuccess(page)
    await mockAuthMe(page)
  })

  test('successful login redirects to /admin', async ({ page }) => {
    await page.goto('/login')
    await page.getByPlaceholder('用户名或邮箱').fill('admin')
    await page.getByPlaceholder('密码').fill('password123')
    await page.getByRole('button', { name: /登\s*录/ }).click()

    // Should land on the admin dashboard.
    await expect(page).toHaveURL(/\/admin$/)
    // Auth tokens persisted to localStorage.
    await expect(page.evaluate(() => localStorage.getItem('access_token'))).resolves.toBe('mock-access-token')
  })

  test('successful login honours ?redirect query', async ({ page }) => {
    // Use the unencoded form, matching what the router guard actually
    // produces when redirecting unauthenticated users (see smoke spec).
    await page.goto('/login?redirect=/admin/articles')
    await page.getByPlaceholder('用户名或邮箱').fill('admin')
    await page.getByPlaceholder('密码').fill('password123')
    await page.getByRole('button', { name: /登\s*录/ }).click()

    await expect(page).toHaveURL(/\/admin\/articles$/)
  })

  test('form validation rejects empty submission', async ({ page }) => {
    await page.goto('/login')
    await page.getByRole('button', { name: /登\s*录/ }).click()
    // Element Plus renders validation messages in .el-form-item__error.
    await expect(page.locator('.el-form-item__error').first()).toBeVisible()
    // Still on the login page.
    await expect(page).toHaveURL(/\/login$/)
  })

  test('failed login shows error message and stays on page', async ({ page }) => {
    // Override the success mock with a failure for this test only.
    await mockLoginFailure(page)
    await page.goto('/login')
    await page.getByPlaceholder('用户名或邮箱').fill('admin')
    await page.getByPlaceholder('密码').fill('wrongpass')
    await page.getByRole('button', { name: /登\s*录/ }).click()

    // ElMessage error toast appears.
    await expect(page.locator('.el-message--error').first()).toBeVisible()
    await expect(page).toHaveURL(/\/login$/)
    // No token persisted.
    await expect(page.evaluate(() => localStorage.getItem('access_token'))).resolves.toBeNull()
  })

  test('logged-in user visiting /login is redirected to admin', async ({ page }) => {
    // Perform login first.
    await page.goto('/login')
    await page.getByPlaceholder('用户名或邮箱').fill('admin')
    await page.getByPlaceholder('密码').fill('password123')
    await page.getByRole('button', { name: /登\s*录/ }).click()
    await expect(page).toHaveURL(/\/admin$/)

    // Now visiting /login (a guest-only route) should bounce back to admin.
    await page.goto('/login')
    await expect(page).toHaveURL(/\/admin$/)
  })
})

test.describe('Logout flow', () => {
  test.beforeEach(async ({ page }) => {
    // Fallback first (lowest precedence), specific mocks last (highest).
    await mockUnmockedApiAs404(page)
    await mockLoginSuccess(page)
    await mockAuthMe(page)
    await mockLogout(page)
  })

  test('logout clears local state and returns to login', async ({ page }) => {
    // Log in.
    await page.goto('/login')
    await page.getByPlaceholder('用户名或邮箱').fill('admin')
    await page.getByPlaceholder('密码').fill('password123')
    await page.getByRole('button', { name: /登\s*录/ }).click()
    await expect(page).toHaveURL(/\/admin$/)

    // The admin layout exposes a logout action via the user dropdown. The
    // menu item text is "退出登录" (see AdminLayout.vue). Open the dropdown
    // first by clicking the avatar/user area in the header.
    const userTrigger = page.locator('.header-user, .user-info, .avatar-wrapper').first()
    await userTrigger.click()
    await page.getByText('退出登录').click()

    // Back to login, tokens gone.
    await expect(page).toHaveURL(/\/login$/)
    await expect(page.evaluate(() => localStorage.getItem('access_token'))).resolves.toBeNull()
    await expect(page.evaluate(() => localStorage.getItem('refresh_token'))).resolves.toBeNull()
  })
})
