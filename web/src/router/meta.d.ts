import 'vue-router'
import type { PermissionSlug } from '@/shared/auth/permissions'

export {}

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    guest?: boolean
    allowWithoutAdmin?: boolean
    title?: string
    icon?: string
    permission?: PermissionSlug
    adminOnly?: boolean
    postType?: 'post' | 'page'
  }
}
