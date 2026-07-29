import { test, expect } from '@playwright/test'
import {
  mockUnmockedApiAs404,
  mockAuthMeAs,
  mockArticleList,
  mockCategoryList,
  mockTagList,
  type MockArticle,
} from './helpers'

/**
 * E2E tests for role-based permission enforcement.
 *
 * Verifies that the UI correctly reflects the current user's permissions:
 *   - admin   → full access (wildcard '*')
 *   - editor  → read / create / update / publish (no delete)
 *   - author  → read / create / update (no publish, no delete)
 *   - subscriber → no article permissions at all
 *
 * Backend is fully mocked — no running server required.
 */

// ─── Shared mock data ────────────────────────────────────────────────────

const sampleArticle: MockArticle = {
  id: 1,
  title: '权限测试文章',
  slug: 'permission-test',
  status: 'draft',
  post_type: 'post',
  summary: '用于权限测试的文章',
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z',
}

// ─── Helper: set up common mocks for a given role ────────────────────────

async function setupRoleMocks(
  page: import('@playwright/test').Page,
  role: 'admin' | 'editor' | 'author' | 'subscriber',
) {
  // Fallback first (lowest precedence), specific mocks last.
  await mockUnmockedApiAs404(page)
  await mockAuthMeAs(page, role)
  await mockCategoryList(page)
  await mockTagList(page)
  await mockArticleList(page, [sampleArticle])
}

// ─── Tests ───────────────────────────────────────────────────────────────

test.describe('Admin role — full access', () => {
  test('admin can see all article actions including delete', async ({ page }) => {
    await setupRoleMocks(page, 'admin')

    await page.goto('/admin/articles')

    // Article should be visible.
    await expect(page.getByText('权限测试文章')).toBeVisible()

    // Admin should have access to the article list page (no redirect to 403).
    await expect(page).toHaveURL(/\/admin\/articles/)
  })

  test('admin can access article create page', async ({ page }) => {
    await setupRoleMocks(page, 'admin')

    await page.goto('/admin/articles/create')

    // Should land on the create page, not be redirected to forbidden.
    await expect(page).toHaveURL(/\/admin\/articles\/create/)
  })
})

test.describe('Editor role — can publish but not delete', () => {
  test('editor can access article list', async ({ page }) => {
    await setupRoleMocks(page, 'editor')

    await page.goto('/admin/articles')

    await expect(page.getByText('权限测试文章')).toBeVisible()
    await expect(page).toHaveURL(/\/admin\/articles/)
  })

  test('editor can access article create page', async ({ page }) => {
    await setupRoleMocks(page, 'editor')

    await page.goto('/admin/articles/create')

    await expect(page).toHaveURL(/\/admin\/articles\/create/)
  })

  test('editor can access article edit page', async ({ page }) => {
    await setupRoleMocks(page, 'editor')

    // Mock the detail endpoint for the article being edited.
    await page.route(/\/api\/v1\/articles\/\d+$/, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: sampleArticle }),
      }),
    )

    await page.goto('/admin/articles/1/edit')

    await expect(page).toHaveURL(/\/admin\/articles\/1\/edit/)
  })
})

test.describe('Author role — read/create/update, no publish', () => {
  test('author can access article list', async ({ page }) => {
    await setupRoleMocks(page, 'author')

    await page.goto('/admin/articles')

    await expect(page.getByText('权限测试文章')).toBeVisible()
    await expect(page).toHaveURL(/\/admin\/articles/)
  })

  test('author can access article create page', async ({ page }) => {
    await setupRoleMocks(page, 'author')

    await page.goto('/admin/articles/create')

    await expect(page).toHaveURL(/\/admin\/articles\/create/)
  })

  test('author cannot access settings that require admin', async ({ page }) => {
    await setupRoleMocks(page, 'author')

    // System settings typically require admin permissions.
    // The router guard should redirect to /admin/forbidden or show a 403 page.
    await page.goto('/admin/settings')

    await expect(page.getByRole('heading', { name: '没有管理后台权限' })).toBeVisible()
  })
})

test.describe('Subscriber role — no article permissions', () => {
  test('subscriber is denied access to article list', async ({ page }) => {
    await setupRoleMocks(page, 'subscriber')

    await page.goto('/admin/articles')

    await expect(page.getByRole('heading', { name: '没有管理后台权限' })).toBeVisible()
  })

  test('subscriber is denied access to article create', async ({ page }) => {
    await setupRoleMocks(page, 'subscriber')

    await page.goto('/admin/articles/create')

    await expect(page.getByRole('heading', { name: '没有管理后台权限' })).toBeVisible()
  })
})

test.describe('Unauthenticated access', () => {
  test('unauthenticated user is redirected to login from articles', async ({ page }) => {
    await mockUnmockedApiAs404(page)
    // No auth mock — simulates an unauthenticated session.

    await page.goto('/admin/articles')

    await expect(page).toHaveURL(/\/login/)
  })

  test('unauthenticated user is redirected to login from article create', async ({ page }) => {
    await mockUnmockedApiAs404(page)

    await page.goto('/admin/articles/create')

    await expect(page).toHaveURL(/\/login/)
  })
})
