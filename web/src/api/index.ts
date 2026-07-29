// Re-export shared types
export type { ApiEnvelope, ApiError, PageMeta, PageResult } from '@/shared/api/types'

// ─── Types ───────────────────────────────────────────────

// Auth types
export type {
  User,
  UserPreferences,
  Role,
  Permission,
  TokenPair,
} from './domains/auth'

// Article types
export type {
  Article,
  ArticleCreateInput,
  ArticleUpdateInput,
  Revision,
} from './domains/articles'

// Category type
export type { Category } from './domains/categories'

// Tag type
export type { Tag } from './domains/tags'

// Comment type
export type { Comment } from './domains/comments'

// Media type
export type { Media } from './domains/media'

// Settings type
export type { SiteSetting } from './domains/settings'

// Menu types
export type { Menu, MenuItem } from './domains/menus'

// ListResponse
export type { ListResponse } from './helpers'

// Analytics types
export type {
  DashboardStats,
  DeviceBreakdownItem,
  DeviceBreakdownResponse,
} from './domains/analytics'

// SEO types
export type { Redirect } from './domains/seo'

// Plugin & Theme types
export type { Plugin, Theme } from './domains/plugins'

// System types
export type {
  APIToken,
  TokenCreated,
  BackupInfo,
  Webhook,
  WebhookLog,
  SearchHit,
  SearchResult,
} from './domains/system'

// Content types
export type {
  ContentField,
  ContentType,
  ContentEntry,
} from './domains/content'

// ─── API Namespaces ──────────────────────────────────────

export { authApi, totpApi } from './domains/auth'
export { articleApi } from './domains/articles'
export { categoryApi } from './domains/categories'
export { tagApi } from './domains/tags'
export { commentApi } from './domains/comments'
export { mediaApi } from './domains/media'
export { userApi } from './domains/users'
export { roleApi } from './domains/roles'
export { settingsApi } from './domains/settings'
export { seoApi } from './domains/seo'
export { menuApi } from './domains/menus'
export { analyticsApi } from './domains/analytics'
export { pluginApi, themeApi } from './domains/plugins'
export { systemApi, tokenApi, backupApi, webhookApi, searchApi } from './domains/system'
export { contentApi } from './domains/content'

// ─── Helpers ─────────────────────────────────────────────

export { getList } from './helpers'
