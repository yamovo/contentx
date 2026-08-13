# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: smoke.spec.ts >> Public pages smoke >> 404 "返回首页" button navigates to the production login entry
- Location: e2e\smoke.spec.ts:60:3

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: locator.click: Test timeout of 30000ms exceeded.
Call log:
  - waiting for getByRole('button', { name: '返回首页' })

```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test'
  2  | import { mockUnmockedApiAs404 } from './helpers'
  3  | 
  4  | /**
  5  |  * Smoke tests for public pages that do not require authentication.
  6  |  * These run against the Vite dev server with no backend; unmocked API calls
  7  |  * return 404 so a missing mock is surfaced explicitly.
  8  |  */
  9  | 
  10 | test.describe('Public pages smoke', () => {
  11 |   test.beforeEach(async ({ page }) => {
  12 |     await mockUnmockedApiAs404(page)
  13 |   })
  14 | 
  15 |   test('production root redirects to login', async ({ page }) => {
  16 |     await page.goto('/')
  17 |     await expect(page).toHaveURL(/\/login$/)
  18 |     await expect(page.locator('.login-header .logo-text')).toHaveText('ContentX')
  19 |   })
  20 | 
  21 |   test('root entry renders the login form', async ({ page }) => {
  22 |     await page.goto('/')
  23 |     await expect(page).toHaveURL(/\/login$/)
  24 |     await expect(page.getByRole('button', { name: /登\s*录/ })).toBeVisible()
  25 |   })
  26 | 
  27 |   test('login page renders form, logo, and register link', async ({ page }) => {
  28 |     await page.goto('/login')
  29 |     // Brand title.
  30 |     await expect(page.locator('.login-header .logo-text')).toHaveText('ContentX')
  31 |     await expect(page.locator('.login-header .subtitle')).toHaveText('内容管理系统')
  32 |     // Inputs present.
  33 |     await expect(page.getByPlaceholder('用户名或邮箱')).toBeVisible()
  34 |     await expect(page.getByPlaceholder('密码')).toBeVisible()
  35 |     // Submit button.
  36 |     await expect(page.getByRole('button', { name: /登\s*录/ })).toBeVisible()
  37 |     // Register link.
  38 |     await expect(page.getByRole('link', { name: '立即注册' })).toHaveAttribute('href', '/register')
  39 |   })
  40 | 
  41 |   test('login page "立即注册" link navigates to register page', async ({ page }) => {
  42 |     await page.goto('/login')
  43 |     await page.getByRole('link', { name: '立即注册' }).click()
  44 |     await expect(page).toHaveURL(/\/register$/)
  45 |   })
  46 | 
  47 |   test('register page renders', async ({ page }) => {
  48 |     await page.goto('/register')
  49 |     // Register page should load without redirecting away (guest route).
  50 |     await expect(page).toHaveURL(/\/register$/)
  51 |   })
  52 | 
  53 |   test('unknown route renders 404 page', async ({ page }) => {
  54 |     await page.goto('/this-route-does-not-exist')
  55 |     await expect(page.locator('.not-found h1')).toHaveText('404')
  56 |     await expect(page.locator('.not-found p')).toHaveText('页面不存在')
  57 |     await expect(page.getByRole('button', { name: '返回首页' })).toBeVisible()
  58 |   })
  59 | 
  60 |   test('404 "返回首页" button navigates to the production login entry', async ({ page }) => {
  61 |     await page.goto('/no-such-page')
> 62 |     await page.getByRole('button', { name: '返回首页' }).click()
     |                                                      ^ Error: locator.click: Test timeout of 30000ms exceeded.
  63 |     await expect(page).toHaveURL(/\/login$/)
  64 |   })
  65 | 
  66 |   test('unauthenticated access to /admin redirects to login with redirect query', async ({ page }) => {
  67 |     await page.goto('/admin/articles')
  68 |     // Should be redirected to /login?redirect=/admin/articles (vue-router
  69 |     // leaves the slash unencoded in the query value).
  70 |     await expect(page).toHaveURL(/\/login\?redirect=\/admin\/articles$/)
  71 |   })
  72 | })
  73 | 
```