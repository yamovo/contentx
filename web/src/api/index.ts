import { get, post, put, del } from './http'

// ─── Types ───────────────────────────────────────────────

export interface User {
  id: number
  username: string
  email: string
  display_name: string
  avatar: string
  bio: string
  website: string
  role: Role
  status: string
  login_count: number
  preferences: UserPreferences
  created_at: string
}

export interface UserPreferences {
  language: string
  theme: string
  email_notify: boolean
  markdown_editor: boolean
  items_per_page: number
  default_post_status: string
}

export interface Role {
  id: number
  name: string
  slug: string
  description: string
  permissions: Permission[]
  is_system: boolean
  user_count?: number
}

export interface Permission {
  id: number
  name: string
  slug: string
  module: string
  description: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  expires_at: string
  expires_in: number
}

export interface Article {
  id: number
  title: string
  slug: string
  content: string
  excerpt: string
  author: User
  author_id: number
  category: Category | null
  category_id: number | null
  tags: Tag[]
  featured_image: string
  status: 'draft' | 'published' | 'pending' | 'scheduled' | 'trash' | 'archived'
  post_type: 'post' | 'page'
  format: string
  visibility: 'public' | 'private' | 'password'
  is_pinned: boolean
  is_featured: boolean
  allow_comment: boolean
  view_count: number
  like_count: number
  word_count: number
  reading_time: number
  published_at: string | null
  scheduled_at: string | null
  meta_title: string
  meta_desc: string
  meta_keywords: string
  comment_count: number
  created_at: string
  updated_at: string
}

export interface Category {
  id: number
  name: string
  slug: string
  description: string
  parent_id: number | null
  children?: Category[]
  image: string
  color: string
  sort_order: number
  post_count: number
  is_active: boolean
}

export interface Tag {
  id: number
  name: string
  slug: string
  count: number
  color: string
}

export interface Comment {
  id: number
  article_id: number
  article?: Article
  user_id: number | null
  user?: User
  parent_id: number | null
  children?: Comment[]
  author_name: string
  author_email: string
  author_url: string
  content: string
  status: 'pending' | 'approved' | 'spam' | 'trash'
  depth: number
  like_count: number
  is_sticky: boolean
  created_at: string
}

export interface Media {
  id: number
  filename: string
  original_name: string
  url: string
  thumbnail_url: string
  mime_type: string
  file_size: number
  width?: number
  height?: number
  alt: string
  title: string
  caption: string
  folder: string
  created_at: string
}

export interface SiteSetting {
  id: number
  key: string
  value: string
  type: string
  group: string
  label: string
  help_text: string
}

export interface Menu {
  id: number
  name: string
  slug: string
  locations: string
  items: MenuItem[]
}

export interface MenuItem {
  id: number
  menu_id: number
  parent_id: number | null
  children?: MenuItem[]
  title: string
  url: string
  target: string
  css_class: string
  icon: string
  sort_order: number
  is_active: boolean
}

export interface ListResponse<T> {
  items: T[]
  page: number
  page_size: number
  total: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
}

export interface DashboardStats {
  total_articles: number
  published_articles: number
  total_comments: number
  pending_comments: number
  total_users: number
  total_media: number
  views_today: number
  views_this_week: number
  views_this_month: number
  total_views: number
}

export interface Revision {
  id: number
  article_id: number
  title: string
  content: string
  excerpt: string
  editor: User
  version: number
  note: string
  created_at: string
}

// ─── API Functions ───────────────────────────────────────

// Auth
export const authApi = {
  login: (data: { username: string; password: string }) =>
    post<{ data: { token: TokenPair; user: User } }>('/auth/login', data),
  register: (data: { username: string; email: string; password: string; display_name?: string }) =>
    post<{ data: { token: TokenPair; user: User } }>('/auth/register', data),
  refresh: (refresh_token: string) =>
    post<{ data: TokenPair }>('/auth/refresh', { refresh_token }),
  logout: () => post('/auth/logout'),
  me: () => get<{ data: { user: User; permissions: string[] } }>('/auth/me'),
  updateProfile: (data: Partial<User>) => put<{ data: User }>('/auth/profile', data),
  changePassword: (data: { old_password: string; new_password: string }) =>
    put('/auth/password', data),
}

// Articles
export const articleApi = {
  list: (params?: Record<string, any>) =>
    get<ListResponse<Article>>('/articles', params),
  get: (id: number) => get<{ data: Article }>(`/articles/${id}`),
  getBySlug: (slug: string) => get<{ data: Article }>(`/articles/slug/${slug}`),
  create: (data: Partial<Article>) => post<{ data: Article }>('/articles', data),
  update: (id: number, data: Partial<Article>) => put<{ data: Article }>(`/articles/${id}`, data),
  delete: (id: number) => del(`/articles/${id}`),
  bulk: (data: { article_ids: number[]; action: string; status?: string; category_id?: number }) =>
    post('/articles/bulk', data),
  revisions: (id: number) => get<{ data: Revision[] }>(`/articles/${id}/revisions`),
  restoreRevision: (id: number, revisionId: number) =>
    post(`/articles/${id}/revisions/${revisionId}/restore`),
  like: (id: number) => post(`/articles/${id}/like`),
}

// Categories
export const categoryApi = {
  list: (params?: Record<string, any>) => get<{ data: Category[] }>('/categories', params),
  get: (id: number) => get<{ data: Category }>(`/categories/${id}`),
  create: (data: Partial<Category>) => post<{ data: Category }>('/categories', data),
  update: (id: number, data: Partial<Category>) => put(`/categories/${id}`, data),
  delete: (id: number) => del(`/categories/${id}`),
  reorder: (items: { id: number; sort_order: number; parent_id?: number }[]) =>
    put('/categories/reorder', { items }),
}

// Tags
export const tagApi = {
  list: (params?: Record<string, any>) => get<{ data: Tag[]; total: number }>('/tags', params),
  get: (id: number) => get<{ data: Tag }>(`/tags/${id}`),
  create: (data: Partial<Tag>) => post<{ data: Tag }>('/tags', data),
  update: (id: number, data: Partial<Tag>) => put(`/tags/${id}`, data),
  delete: (id: number) => del(`/tags/${id}`),
  merge: (data: { source_ids: number[]; target_id: number; delete_old: boolean }) =>
    post('/tags/merge', data),
}

// Comments
export const commentApi = {
  list: (params?: Record<string, any>) => get<ListResponse<Comment>>('/comments', params),
  get: (id: number) => get<{ data: Comment }>(`/comments/${id}`),
  create: (data: Partial<Comment>) => post<{ data: Comment }>('/comments', data),
  update: (id: number, data: Partial<Comment>) => put(`/comments/${id}`, data),
  approve: (id: number) => post(`/comments/${id}/approve`),
  spam: (id: number) => post(`/comments/${id}/spam`),
  trash: (id: number) => post(`/comments/${id}/trash`),
  bulk: (data: { comment_ids: number[]; action: string }) => post('/comments/bulk', data),
  stats: () => get<{ data: { total: number; pending: number; approved: number; spam: number; today: number } }>('/comments/stats'),
  articleComments: (articleId: number) => get<{ data: Comment[] }>(`/articles/${articleId}/comments`),
}

// Media
export const mediaApi = {
  list: (params?: Record<string, any>) => get<ListResponse<Media>>('/media', params),
  get: (id: number) => get<{ data: Media }>(`/media/${id}`),
  upload: (formData: FormData) =>
    post<{ data: Media }>('/media/upload', formData),
  update: (id: number, data: Partial<Media>) => put(`/media/${id}`, data),
  delete: (id: number) => del(`/media/${id}`),
  bulkDelete: (ids: number[]) => post('/media/bulk-delete', { ids }),
  folders: () => get<{ data: string[] }>('/media/folders'),
  stats: () => get<{ data: { total_files: number; total_size: number; images: number; videos: number; documents: number } }>('/media/stats'),
}

// Users
export const userApi = {
  list: (params?: Record<string, any>) => get<ListResponse<User>>('/users', params),
  get: (id: number) => get<{ data: User }>(`/users/${id}`),
  create: (data: Partial<User> & { password: string }) => post<{ data: User }>('/users', data),
  update: (id: number, data: Partial<User>) => put(`/users/${id}`, data),
  delete: (id: number) => del(`/users/${id}`),
  resetPassword: (id: number, new_password: string) =>
    post(`/users/${id}/reset-password`, { new_password }),
}

// Roles
export const roleApi = {
  list: () => get<{ data: Role[] }>('/roles'),
  create: (data: Partial<Role>) => post('/roles', data),
  update: (id: number, data: Partial<Role>) => put(`/roles/${id}`, data),
  delete: (id: number) => del(`/roles/${id}`),
  permissions: () => get<{ data: Permission[]; grouped: Record<string, Permission[]> }>('/roles/permissions'),
}

// Settings
export const settingsApi = {
  list: (group?: string) => get<{ data: SiteSetting[]; grouped: Record<string, SiteSetting[]> }>('/settings', { group }),
  get: (key: string) => get<{ data: SiteSetting }>(`/settings/${key}`),
  update: (data: Record<string, any>) => put('/settings', data),
  public: () => get<{ data: Record<string, string> }>('/settings/public'),
}

// SEO
export const seoApi = {
  getSetting: (type: string, id: number) => get(`/seo/${type}/${id}`),
  updateSetting: (type: string, id: number, data: any) => put(`/seo/${type}/${id}`, data),
  sitemap: () => get('/seo/sitemap'),
  robotsTxt: () => get('/seo/robots.txt'),
  listRedirects: () => get('/seo/redirects'),
  createRedirect: (data: any) => post('/seo/redirects', data),
  deleteRedirect: (id: number) => del(`/seo/redirects/${id}`),
}

// Menus
export const menuApi = {
  list: () => get<{ data: Menu[] }>('/menus'),
  get: (id: number) => get<{ data: Menu }>(`/menus/${id}`),
  create: (data: Partial<Menu>) => post('/menus', data),
  update: (id: number, data: Partial<Menu>) => put(`/menus/${id}`, data),
  delete: (id: number) => del(`/menus/${id}`),
  addItem: (menuId: number, data: Partial<MenuItem>) => post(`/menus/${menuId}/items`, data),
  updateItem: (menuId: number, itemId: number, data: Partial<MenuItem>) =>
    put(`/menus/${menuId}/items/${itemId}`, data),
  deleteItem: (menuId: number, itemId: number) => del(`/menus/${menuId}/items/${itemId}`),
  reorderItems: (menuId: number, items: { id: number; sort_order: number; parent_id?: number }[]) =>
    put(`/menus/${menuId}/items/reorder`, { items }),
}

// Analytics
export const analyticsApi = {
  dashboard: () => get<{ stats: DashboardStats; recent_articles: Article[]; recent_comments: Comment[]; popular_articles: Article[] }>('/analytics/dashboard'),
  viewsOverTime: (days?: number) => get<{ data: { date: string; views: number }[] }>('/analytics/views', { days }),
  topReferrers: () => get<{ data: { referrer: string; count: number }[] }>('/analytics/referrers'),
  deviceBreakdown: () => get('/analytics/devices'),
  recordView: (data: { article_id?: number; path: string; duration?: number }) =>
    post('/analytics/record', data),
}

// Plugins & Themes
export const pluginApi = {
  list: () => get('/plugins'),
  enable: (id: number) => post(`/plugins/${id}/enable`),
  disable: (id: number) => post(`/plugins/${id}/disable`),
  updateConfig: (id: number, config: any) => put(`/plugins/${id}/config`, config),
}

export const themeApi = {
  list: () => get('/themes'),
  activate: (id: number) => post(`/themes/${id}/activate`),
  updateConfig: (id: number, config: any) => put(`/themes/${id}/config`, config),
}

// System
export const systemApi = {
  info: () => get('/system/info'),
  health: () => get('/system/health'),
  activity: (params?: Record<string, any>) => get('/system/activity', params),
}
