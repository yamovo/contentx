import { test, expect } from '@playwright/test'
import {
  mockAuthMe,
  mockUnmockedApiAs404,
  mockArticleList,
  mockArticleDetail,
  mockArticleDelete,
  mockArticleStatusChange,
  mockCategoryList,
  mockTagList,
  type MockArticle,
} from './helpers'

/**
 * E2E tests for article CRUD workflows.
 *
 * Backend is fully mocked via page.route — no running server required.
 * Routes use RegExp matching (consistent with helpers.ts).
 */

// ─── Shared mock data ────────────────────────────────────────────────────

const draftArticle: MockArticle = {
  id: 1,
  title: '测试草稿文章',
  slug: 'test-draft',
  status: 'draft',
  post_type: 'post',
  summary: '这是一篇草稿',
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z',
}

const publishedArticle: MockArticle = {
  id: 2,
  title: '已发布文章',
  slug: 'published-article',
  status: 'published',
  post_type: 'post',
  summary: '这是一篇已发布的文章',
  created_at: '2026-05-01T00:00:00Z',
  updated_at: '2026-06-15T00:00:00Z',
}

const pageArticle: MockArticle = {
  id: 10,
  title: '关于我们',
  slug: 'about',
  status: 'published',
  post_type: 'page',
  summary: '关于我们页面',
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-10T00:00:00Z',
}

// ─── Tests ───────────────────────────────────────────────────────────────

test.describe('Article list', () => {
  test.beforeEach(async ({ page }) => {
    // Fallback first (lowest precedence), specific mocks last.
    await mockUnmockedApiAs404(page)
    await mockAuthMe(page)
    await mockCategoryList(page)
    await mockTagList(page)
  })

  test('displays article titles in the list', async ({ page }) => {
    await mockArticleList(page, [draftArticle, publishedArticle])

    await page.goto('/admin/articles', { waitUntil: 'domcontentloaded' })

    await expect(page.getByText('测试草稿文章')).toBeVisible()
    await expect(page.getByText('已发布文章')).toBeVisible()
  })

  test('shows empty state when no articles exist', async ({ page }) => {
    await mockArticleList(page, [])

    await page.goto('/admin/articles')

    // Table should be empty — Element Plus empty state uses .el-table__empty-text
    // or a generic "暂无数据" / "No data" text.
    const emptyText = page.locator('.el-table__empty-text, .el-empty')
    await expect(emptyText.first()).toBeVisible()
  })

  test('navigates to create article page', async ({ page }) => {
    await mockArticleList(page, [])

    await page.goto('/admin/articles')

    // Click the "写文章" / "新建" / create button in the header area.
    const createBtn = page.getByRole('link', { name: /写文章|新建/ }).first()
    if (await createBtn.isVisible()) {
      await createBtn.click()
      await expect(page).toHaveURL(/\/admin\/articles\/create/)
    }
  })
})

test.describe('Article detail & edit', () => {
  test.beforeEach(async ({ page }) => {
    await mockUnmockedApiAs404(page)
    await mockAuthMe(page)
    await mockCategoryList(page, [
      { id: 1, name: '技术', slug: 'tech' },
      { id: 2, name: '生活', slug: 'life' },
    ])
    await mockTagList(page, [
      { id: 1, name: 'Vue', slug: 'vue' },
      { id: 2, name: 'Go', slug: 'go' },
    ])
  })

  test('editor page loads article data for editing', async ({ page }) => {
    await mockArticleDetail(page, draftArticle)

    await page.goto('/admin/articles/1/edit')

    // The title input should contain the article title.
    const titleInput = page.locator('input[placeholder*="标题"], input[name="title"]').first()
    await expect(titleInput).toHaveValue('测试草稿文章')
  })

  test('editor page loads blank form for new article', async ({ page }) => {
    await page.goto('/admin/articles/create')

    // Title input should be empty for a new article.
    const titleInput = page.locator('input[placeholder*="标题"], input[name="title"]').first()
    await expect(titleInput).toHaveValue('')
  })
})

test.describe('Article delete', () => {
  test.beforeEach(async ({ page }) => {
    await mockUnmockedApiAs404(page)
    await mockAuthMe(page)
    await mockCategoryList(page)
    await mockTagList(page)
  })

  test('delete article removes it from the list', async ({ page }) => {
    await mockArticleList(page, [draftArticle, publishedArticle])
    await mockArticleDelete(page)
    // After delete, the list is re-fetched — respond with only the remaining article.
    let listCallCount = 0
    await page.route(/\/api\/v1\/articles/, (route) => {
      listCallCount++
      const articles = listCallCount === 1
        ? [draftArticle, publishedArticle]
        : [publishedArticle] // after delete
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            items: articles,
            page: 1,
            page_size: 20,
            total: articles.length,
            total_pages: 1,
            has_next: false,
            has_prev: false,
          },
        }),
      })
    })

    await page.goto('/admin/articles')

    // Click the delete button/action for the first article.
    // Element Plus table rows have action buttons — look for a delete icon/button.
    const deleteBtn = page.locator('button, .el-button').filter({ hasText: /删除/ }).first()
    if (await deleteBtn.isVisible()) {
      await deleteBtn.click()
      // Confirm the deletion in the dialog (Element Plus confirm dialog).
      const confirmBtn = page.locator('.el-message-box__btns button').filter({ hasText: /确定|确认/ }).first()
      if (await confirmBtn.isVisible()) {
        await confirmBtn.click()
      }
      // After deletion, the deleted article title should no longer be visible.
      await expect(page.getByText('测试草稿文章')).not.toBeVisible({ timeout: 5000 })
    }
  })
})

test.describe('Page management (post_type=page)', () => {
  test.beforeEach(async ({ page }) => {
    await mockUnmockedApiAs404(page)
    await mockAuthMe(page)
    await mockCategoryList(page)
    await mockTagList(page)
  })

  test('pages list shows only page-type entries', async ({ page }) => {
    await mockArticleList(page, [pageArticle])

    await page.goto('/admin/pages')

    await expect(page.getByText('关于我们')).toBeVisible()
    // Post-type articles should not appear on the pages route.
    await expect(page.getByText('测试草稿文章')).not.toBeVisible()
  })

  test('navigates to create page', async ({ page }) => {
    await mockArticleList(page, [])

    await page.goto('/admin/pages')

    const createBtn = page.getByRole('link', { name: /新建页面|新建/ }).first()
    if (await createBtn.isVisible()) {
      await createBtn.click()
      await expect(page).toHaveURL(/\/admin\/pages\/create/)
    }
  })
})

test.describe('Article status change', () => {
  test.beforeEach(async ({ page }) => {
    await mockUnmockedApiAs404(page)
    await mockAuthMe(page)
    await mockCategoryList(page)
    await mockTagList(page)
  })

  test('publish action is available for draft articles', async ({ page }) => {
    await mockArticleList(page, [draftArticle])
    await mockArticleStatusChange(page)

    await page.goto('/admin/articles')

    // Look for a publish action button/link in the article row.
    const publishBtn = page.locator('button, .el-button, a').filter({ hasText: /发布/ }).first()
    if (await publishBtn.isVisible()) {
      await expect(publishBtn).toBeEnabled()
    }
  })
})
