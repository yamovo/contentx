import { describe, it, expect, beforeEach, vi } from 'vitest'

// Mock the http helpers.
const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}))

vi.mock('./http', () => ({
  get: mocks.get,
  post: mocks.post,
  put: mocks.put,
  del: mocks.del,
  default: {},
}))

import {
  articleApi,
  categoryApi,
  tagApi,
  commentApi,
  mediaApi,
  userApi,
  roleApi,
  settingsApi,
  seoApi,
  menuApi,
  pluginApi,
  themeApi,
  systemApi,
  analyticsApi,
  authApi,
} from '@/api'

describe('API module', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('authApi', () => {
    it('login posts to /auth/login', () => {
      authApi.login({ username: 'u', password: 'p' })
      expect(mocks.post).toHaveBeenCalledWith('/auth/login', { username: 'u', password: 'p' })
    })

    it('refresh posts refresh_token', () => {
      authApi.refresh('rt')
      expect(mocks.post).toHaveBeenCalledWith('/auth/refresh', { refresh_token: 'rt' })
    })

    it('me gets /auth/me', () => {
      authApi.me()
      expect(mocks.get).toHaveBeenCalledWith('/auth/me')
    })
  })

  describe('articleApi', () => {
    it('list calls /articles with params', () => {
      articleApi.list({ page: 1, status: 'published' })
      expect(mocks.get).toHaveBeenCalledWith('/articles', { page: 1, status: 'published' })
    })

    it('get calls /articles/:id', () => {
      articleApi.get(42)
      expect(mocks.get).toHaveBeenCalledWith('/articles/42')
    })

    it('create posts to /articles', () => {
      articleApi.create({ title: 'New' })
      expect(mocks.post).toHaveBeenCalledWith('/articles', { title: 'New' })
    })

    it('update puts to /articles/:id', () => {
      articleApi.update(5, { title: 'Updated' })
      expect(mocks.put).toHaveBeenCalledWith('/articles/5', { title: 'Updated' })
    })

    it('delete deletes /articles/:id', () => {
      articleApi.delete(7)
      expect(mocks.del).toHaveBeenCalledWith('/articles/7')
    })

    it('bulk posts to /articles/bulk', () => {
      articleApi.bulk({ article_ids: [1, 2], action: 'publish' })
      expect(mocks.post).toHaveBeenCalledWith('/articles/bulk', { article_ids: [1, 2], action: 'publish' })
    })

    it('revisions gets /articles/:id/revisions', () => {
      articleApi.revisions(10)
      expect(mocks.get).toHaveBeenCalledWith('/articles/10/revisions')
    })
  })

  describe('categoryApi', () => {
    it('list calls /categories with params', () => {
      categoryApi.list({ all: 'true' })
      expect(mocks.get).toHaveBeenCalledWith('/categories', { all: 'true' })
    })

    it('create posts to /categories', () => {
      categoryApi.create({ name: 'Tech' })
      expect(mocks.post).toHaveBeenCalledWith('/categories', { name: 'Tech' })
    })

    it('reorder puts to /categories/reorder', () => {
      categoryApi.reorder([{ id: 1, sort_order: 0 }])
      expect(mocks.put).toHaveBeenCalledWith('/categories/reorder', { items: [{ id: 1, sort_order: 0 }] })
    })
  })

  describe('tagApi', () => {
    it('list calls /tags with params', () => {
      tagApi.list()
      expect(mocks.get).toHaveBeenCalledWith('/tags', undefined)
    })

    it('merge posts to /tags/merge', () => {
      tagApi.merge({ source_ids: [1], target_id: 2, delete_old: true })
      expect(mocks.post).toHaveBeenCalledWith('/tags/merge', { source_ids: [1], target_id: 2, delete_old: true })
    })
  })

  describe('commentApi', () => {
    it('list calls /comments with params', () => {
      commentApi.list({ page: 1 })
      expect(mocks.get).toHaveBeenCalledWith('/comments', { page: 1 })
    })

    it('approve posts to /comments/:id/approve', () => {
      commentApi.approve(3)
      expect(mocks.post).toHaveBeenCalledWith('/comments/3/approve')
    })

    it('stats gets /comments/stats', () => {
      commentApi.stats()
      expect(mocks.get).toHaveBeenCalledWith('/comments/stats')
    })
  })

  describe('mediaApi', () => {
    it('list calls /media with params', () => {
      mediaApi.list({ page: 1 })
      expect(mocks.get).toHaveBeenCalledWith('/media', { page: 1 })
    })

    it('upload posts FormData to /media/upload', () => {
      const fd = new FormData()
      mediaApi.upload(fd)
      expect(mocks.post).toHaveBeenCalledWith('/media/upload', fd)
    })

    it('folders gets /media/folders', () => {
      mediaApi.folders()
      expect(mocks.get).toHaveBeenCalledWith('/media/folders')
    })

    it('stats gets /media/stats', () => {
      mediaApi.stats()
      expect(mocks.get).toHaveBeenCalledWith('/media/stats')
    })
  })

  describe('userApi', () => {
    it('list calls /users with params', () => {
      userApi.list()
      expect(mocks.get).toHaveBeenCalledWith('/users', undefined)
    })

    it('resetPassword posts to /users/:id/reset-password', () => {
      userApi.resetPassword(1, 'newpass')
      expect(mocks.post).toHaveBeenCalledWith('/users/1/reset-password', { new_password: 'newpass' })
    })
  })

  describe('roleApi', () => {
    it('permissions gets /roles/permissions', () => {
      roleApi.permissions()
      expect(mocks.get).toHaveBeenCalledWith('/roles/permissions')
    })
  })

  describe('settingsApi', () => {
    it('list calls /settings with group param', () => {
      settingsApi.list('general')
      expect(mocks.get).toHaveBeenCalledWith('/settings', { group: 'general' })
    })

    it('public gets /settings/public', () => {
      settingsApi.public()
      expect(mocks.get).toHaveBeenCalledWith('/settings/public')
    })
  })

  describe('seoApi', () => {
    it('listRedirects gets /seo/redirects', () => {
      seoApi.listRedirects()
      expect(mocks.get).toHaveBeenCalledWith('/seo/redirects')
    })

    it('createRedirect posts to /seo/redirects', () => {
      seoApi.createRedirect({ from_path: '/old', to_path: '/new' })
      expect(mocks.post).toHaveBeenCalledWith('/seo/redirects', { from_path: '/old', to_path: '/new' })
    })
  })

  describe('menuApi', () => {
    it('list gets /menus', () => {
      menuApi.list()
      expect(mocks.get).toHaveBeenCalledWith('/menus')
    })

    it('addItem posts to /menus/:id/items', () => {
      menuApi.addItem(1, { title: 'Home' })
      expect(mocks.post).toHaveBeenCalledWith('/menus/1/items', { title: 'Home' })
    })
  })

  describe('pluginApi', () => {
    it('list gets /plugins', () => {
      mocks.get.mockReturnValueOnce(Promise.resolve({ data: [] }))
      pluginApi.list()
      expect(mocks.get).toHaveBeenCalledWith('/plugins')
    })

    it('enable posts to /plugins/:id/enable', () => {
      pluginApi.enable(1)
      expect(mocks.post).toHaveBeenCalledWith('/plugins/1/enable')
    })

    it('disable posts to /plugins/:id/disable', () => {
      pluginApi.disable(2)
      expect(mocks.post).toHaveBeenCalledWith('/plugins/2/disable')
    })
  })

  describe('themeApi', () => {
    it('activate posts to /themes/:id/activate', () => {
      themeApi.activate(1)
      expect(mocks.post).toHaveBeenCalledWith('/themes/1/activate')
    })
  })

  describe('systemApi', () => {
    it('health gets /system/health', () => {
      systemApi.health()
      expect(mocks.get).toHaveBeenCalledWith('/system/health')
    })

    it('activity gets /system/activity with params', () => {
      systemApi.activity({ page: 1 })
      expect(mocks.get).toHaveBeenCalledWith('/system/activity', { page: 1 })
    })
  })

  describe('analyticsApi', () => {
    it('dashboard gets /analytics/dashboard', () => {
      analyticsApi.dashboard()
      expect(mocks.get).toHaveBeenCalledWith('/analytics/dashboard')
    })

    it('recordView posts to /analytics/record', () => {
      analyticsApi.recordView({ path: '/test' })
      expect(mocks.post).toHaveBeenCalledWith('/analytics/record', { path: '/test' })
    })
  })
})
