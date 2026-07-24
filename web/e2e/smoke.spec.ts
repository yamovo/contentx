import { test, expect } from '@playwright/test'
import { mockUnmockedApiAs404 } from './helpers'

/**
 * Smoke tests for public pages that do not require authentication.
 * These run against the Vite dev server with no backend; unmocked API calls
 * return 404 so a missing mock is surfaced explicitly.
 */

test.describe('Public pages smoke', () => {
  test.beforeEach(async ({ page }) => {
    await mockUnmockedApiAs404(page)
  })

  test('home page renders hero and feature cards', async ({ page }) => {
    await page.goto('/')
    // Logo / brand.
    await expect(page.locator('.hero .logo')).toHaveText('ContentX')
    // Hero headline.
    await expect(page.locator('.hero-content h1')).toHaveText('现代化内容管理系统')
    // Feature cards are rendered from a static array of 6.
    await expect(page.locator('.feature-card')).toHaveCount(6)
    // Footer copyright.
    await expect(page.locator('.footer p')).toContainText('© 2026 ContentX')
  })

  test('home page "进入后台" button navigates to login', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: '进入后台' }).click()
    await expect(page).toHaveURL(/\/login$/)
  })

  test('login page renders form, logo, and register link', async ({ page }) => {
    await page.goto('/login')
    // Brand title.
    await expect(page.locator('.login-header .logo-text')).toHaveText('ContentX')
    await expect(page.locator('.login-header .subtitle')).toHaveText('内容管理系统')
    // Inputs present.
    await expect(page.getByPlaceholder('用户名或邮箱')).toBeVisible()
    await expect(page.getByPlaceholder('密码')).toBeVisible()
    // Submit button.
    await expect(page.getByRole('button', { name: /登\s*录/ })).toBeVisible()
    // Register link.
    await expect(page.getByRole('link', { name: '立即注册' })).toHaveAttribute('href', '/register')
  })

  test('login page "立即注册" link navigates to register page', async ({ page }) => {
    await page.goto('/login')
    await page.getByRole('link', { name: '立即注册' }).click()
    await expect(page).toHaveURL(/\/register$/)
  })

  test('register page renders', async ({ page }) => {
    await page.goto('/register')
    // Register page should load without redirecting away (guest route).
    await expect(page).toHaveURL(/\/register$/)
  })

  test('unknown route renders 404 page', async ({ page }) => {
    await page.goto('/this-route-does-not-exist')
    await expect(page.locator('.not-found h1')).toHaveText('404')
    await expect(page.locator('.not-found p')).toHaveText('页面不存在')
    await expect(page.getByRole('button', { name: '返回首页' })).toBeVisible()
  })

  test('404 "返回首页" button navigates to home', async ({ page }) => {
    await page.goto('/no-such-page')
    await page.getByRole('button', { name: '返回首页' }).click()
    await expect(page).toHaveURL('/')
  })

  test('unauthenticated access to /admin redirects to login with redirect query', async ({ page }) => {
    await page.goto('/admin/articles')
    // Should be redirected to /login?redirect=/admin/articles (vue-router
    // leaves the slash unencoded in the query value).
    await expect(page).toHaveURL(/\/login\?redirect=\/admin\/articles$/)
  })
})
