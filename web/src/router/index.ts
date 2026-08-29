import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import { useAuthStore } from '@/stores/auth'
import { PERMISSIONS } from '@/shared/auth/permissions'

NProgress.configure({ showSpinner: false })

const AdminLayout = () => import('@/layouts/AdminLayout.vue')
const LoginView = () => import('@/views/login/LoginView.vue')

const routes: RouteRecordRaw[] = [
  // Production entry: authenticated users are redirected by the guest guard.
  {
    path: '/',
    redirect: '/login',
  },

  // Login.
  {
    path: '/login',
    name: 'Login',
    component: LoginView,
    meta: { guest: true },
  },

  {
    path: '/forbidden',
    name: 'Forbidden',
    component: () => import('@/pages/errors/ForbiddenPage.vue'),
    meta: { requiresAuth: true, allowWithoutAdmin: true },
  },
  {
    path: '/unavailable',
    name: 'ServiceUnavailable',
    component: () => import('@/pages/errors/ServiceUnavailablePage.vue'),
    meta: { allowWithoutAdmin: true },
  },

  // Admin routes.
  {
    path: '/admin',
    component: AdminLayout,
    meta: { requiresAuth: true },
    children: [
      // Dashboard
      {
        path: '',
        name: 'AdminDashboard',
        component: () => import('@/pages/dashboard/AdminDashboardPage.vue'),
        meta: { title: '仪表盘', icon: 'Odometer', permission: PERMISSIONS.analytics.read },
      },

      // Articles
      {
        path: 'articles',
        name: 'ArticleList',
        component: () => import('@/pages/articles/ArticleListPage.vue'),
        meta: { title: '文章管理', icon: 'Document', permission: PERMISSIONS.articles.read },
      },
      {
        path: 'articles/create',
        name: 'ArticleCreate',
        component: () => import('@/pages/articles/ArticleEditorPage.vue'),
        meta: { title: '写文章', permission: PERMISSIONS.articles.create, postType: 'post' },
      },
      {
        path: 'articles/:id/edit',
        name: 'ArticleEdit',
        component: () => import('@/pages/articles/ArticleEditorPage.vue'),
        meta: { title: '编辑文章', permission: PERMISSIONS.articles.update, postType: 'post' },
      },
      {
        path: 'articles/:id/revisions',
        name: 'ArticleRevisions',
        component: () => import('@/pages/articles/ArticleRevisionsPage.vue'),
        meta: { title: '版本历史', permission: PERMISSIONS.articles.read, postType: 'post' },
      },

      // Pages
      {
        path: 'pages',
        name: 'PageList',
        component: () => import('@/pages/articles/ArticleListPage.vue'),
        meta: {
          title: '页面管理',
          icon: 'Notebook',
          permission: PERMISSIONS.articles.read,
          postType: 'page',
        },
      },
      {
        path: 'pages/create',
        name: 'PageCreate',
        component: () => import('@/pages/articles/ArticleEditorPage.vue'),
        meta: {
          title: '新建页面',
          permission: PERMISSIONS.articles.create,
          postType: 'page',
        },
      },
      {
        path: 'pages/:id/edit',
        name: 'PageEdit',
        component: () => import('@/pages/articles/ArticleEditorPage.vue'),
        meta: {
          title: '编辑页面',
          permission: PERMISSIONS.articles.update,
          postType: 'page',
        },
      },
      {
        path: 'pages/:id/revisions',
        name: 'PageRevisions',
        component: () => import('@/pages/articles/ArticleRevisionsPage.vue'),
        meta: {
          title: '页面版本历史',
          permission: PERMISSIONS.articles.read,
          postType: 'page',
        },
      },

      // Categories
      {
        path: 'categories',
        name: 'CategoryList',
        component: () => import('@/pages/categories/CategoryListPage.vue'),
        meta: { title: '分类管理', icon: 'Folder', permission: PERMISSIONS.categories.read },
      },

      // Custom content types (type CRUD is admin-only on the backend)
      {
        path: 'content-types',
        name: 'ContentTypeList',
        component: () => import('@/pages/content/ContentTypeListPage.vue'),
        meta: {
          title: '内容类型',
          icon: 'Grid',
          permission: PERMISSIONS.contentTypes.read,
          adminOnly: true,
        },
      },
      {
        path: 'content/:uid',
        name: 'ContentEntryList',
        component: () => import('@/pages/content/ContentEntryListPage.vue'),
        meta: { title: '内容条目', permission: PERMISSIONS.content.read, adminOnly: true },
      },

      // Tags
      {
        path: 'tags',
        name: 'TagList',
        component: () => import('@/pages/tags/TagListPage.vue'),
        meta: { title: '标签管理', icon: 'PriceTag', permission: PERMISSIONS.tags.read },
      },

      // Comments
      {
        path: 'comments',
        name: 'CommentList',
        component: () => import('@/pages/comments/CommentListPage.vue'),
        meta: { title: '评论管理', icon: 'ChatDotSquare', permission: PERMISSIONS.comments.read },
      },

      // Media
      {
        path: 'media',
        name: 'MediaLibrary',
        component: () => import('@/pages/media/MediaLibraryPage.vue'),
        meta: { title: '媒体库', icon: 'Picture', permission: PERMISSIONS.media.read },
      },

      // Users
      {
        path: 'users',
        name: 'UserList',
        component: () => import('@/pages/users/UserListPage.vue'),
        meta: { title: '用户管理', icon: 'User', permission: PERMISSIONS.users.read },
      },
      {
        path: 'users/:id',
        name: 'UserDetail',
        component: () => import('@/pages/users/UserDetailPage.vue'),
        meta: { title: '用户详情', permission: PERMISSIONS.users.read },
      },

      // Roles
      {
        path: 'roles',
        name: 'RoleList',
        component: () => import('@/pages/roles/RoleListPage.vue'),
        meta: { title: '角色权限', icon: 'Lock', permission: PERMISSIONS.roles.read },
      },

      // Menus
      {
        path: 'menus',
        name: 'MenuManager',
        component: () => import('@/pages/settings/MenuManagerPage.vue'),
        meta: { title: '导航菜单', icon: 'Menu', permission: PERMISSIONS.menus.read },
      },

      // SEO
      {
        path: 'seo',
        name: 'SEOManager',
        component: () => import('@/pages/seo/SEOManagerPage.vue'),
        meta: { title: 'SEO 管理', icon: 'Search', permission: PERMISSIONS.seo.read },
      },
      {
        path: 'seo/redirects',
        name: 'RedirectManager',
        component: () => import('@/pages/seo/RedirectManagerPage.vue'),
        meta: { title: 'URL 重定向', permission: PERMISSIONS.seo.read },
      },

      // Analytics
      {
        path: 'analytics',
        name: 'Analytics',
        component: () => import('@/pages/analytics/AnalyticsPage.vue'),
        meta: { title: '数据分析', icon: 'TrendCharts', permission: PERMISSIONS.analytics.read },
      },

      // Plugins
      {
        path: 'plugins',
        name: 'PluginList',
        component: () => import('@/pages/plugins/PluginListPage.vue'),
        meta: { title: '插件管理', icon: 'Connection', permission: PERMISSIONS.plugins.read },
      },

      // Themes
      {
        path: 'themes',
        name: 'ThemeList',
        component: () => import('@/pages/plugins/ThemeListPage.vue'),
        meta: { title: '主题管理', icon: 'Brush', permission: PERMISSIONS.themes.read },
      },

      // Settings
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/pages/settings/SettingsPage.vue'),
        meta: { title: '系统设置', icon: 'Setting', permission: PERMISSIONS.settings.read },
      },

      // Activity Log
      {
        path: 'activity',
        name: 'ActivityLog',
        component: () => import('@/pages/settings/ActivityLogPage.vue'),
        meta: {
          title: '操作日志',
          icon: 'Tickets',
          permission: PERMISSIONS.system.activityLog,
        },
      },

      // API Tokens (backend routes are admin-only)
      {
        path: 'tokens',
        name: 'TokenList',
        component: () => import('@/pages/system/TokenListPage.vue'),
        meta: {
          title: 'API 令牌',
          icon: 'Key',
          permission: PERMISSIONS.apiTokens.read,
          adminOnly: true,
        },
      },

      // Backup & restore (backend routes are admin-only)
      {
        path: 'backup',
        name: 'BackupManager',
        component: () => import('@/pages/system/BackupManagerPage.vue'),
        meta: {
          title: '备份恢复',
          icon: 'Coin',
          permission: PERMISSIONS.backups.read,
          adminOnly: true,
        },
      },

      // Webhooks (backend routes are admin-only)
      {
        path: 'webhooks',
        name: 'WebhookList',
        component: () => import('@/pages/system/WebhookListPage.vue'),
        meta: {
          title: 'Webhook',
          icon: 'Link',
          permission: PERMISSIONS.webhooks.read,
          adminOnly: true,
        },
      },

      // Tenants (platform administration, RFC-001 PR-5)
      {
        path: 'tenants',
        name: 'TenantList',
        component: () => import('@/pages/system/TenantListPage.vue'),
        meta: {
          title: '租户管理',
          icon: 'Grid',
          permission: PERMISSIONS.tenants.read,
          adminOnly: true,
        },
      },

      // Profile (current user)
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/pages/settings/ProfilePage.vue'),
        meta: { title: '个人资料' },
      },
    ],
  },

  // Catch-all 404.
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/shared/NotFound.vue'),
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

// Navigation guards.
router.beforeEach(async (to, _from, next) => {
  NProgress.start()

  const authStore = useAuthStore()

  if (to.meta.requiresAuth) {
    if (!authStore.isAuthenticated) {
      next({ name: 'Login', query: { redirect: to.fullPath } })
      return
    }
    // 等待 /api/v1/auth/me 完成。token 失效时 fetchUser 会调 clearAuth()
    // 把 token 清空，必须再次检查 isAuthenticated，否则用户会停在空白
    // 的 AdminDashboard（所有 API 都 401，页面拉不到数据）。
    try {
      await authStore.ensureUserLoaded()
    } catch {
      next({
        name: 'ServiceUnavailable',
        query: { redirect: to.fullPath },
      })
      return
    }
    if (!authStore.isAuthenticated) {
      next({ name: 'Login', query: { redirect: to.fullPath } })
      return
    }
    if (!to.meta.allowWithoutAdmin && !authStore.canAccessAdmin) {
      next({ name: 'Forbidden' })
      return
    }
  }

  if (to.meta.guest && authStore.isAuthenticated) {
    next({ name: 'AdminDashboard' })
    return
  }

  // Check permissions. ensureUserLoaded 已在 requiresAuth 分支里 await 过，
  // 这里 user/permissions 一定已就绪，可以直接同步检查。
  if (to.meta.permission && !authStore.hasPermission(to.meta.permission)) {
    next({ name: 'Forbidden', query: { from: to.fullPath } })
    return
  }

  // Admin-only pages (backend enforces RequireAdmin on these APIs).
  if (to.meta.adminOnly && !authStore.isAdmin) {
    next({ name: 'Forbidden', query: { from: to.fullPath } })
    return
  }

  next()
})

router.afterEach(() => {
  NProgress.done()
})

export default router
