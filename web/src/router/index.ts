import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import { useAuthStore } from '@/stores/auth'

NProgress.configure({ showSpinner: false })

const AdminLayout = () => import('@/layouts/AdminLayout.vue')
const LoginView = () => import('@/views/login/LoginView.vue')

const NotFound = () => import('@/views/NotFound.vue')

const routes: RouteRecordRaw[] = [
  // Public routes (front-end).
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/dashboard/HomeView.vue'),
  },

  // Login.
  {
    path: '/login',
    name: 'Login',
    component: LoginView,
    meta: { guest: true },
  },

  // Register.
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/login/RegisterView.vue'),
    meta: { guest: true },
  },


  // Public blog.
  {
    path: '/blog',
    component: () => import('@/views/blog/BlogLayout.vue'),
    children: [
      {
        path: '',
        name: 'BlogList',
        component: () => import('@/views/blog/BlogList.vue'),
      },
      {
        path: 'article/:slug',
        name: 'BlogArticle',
        component: () => import('@/views/blog/BlogArticle.vue'),
      },
      {
        path: 'category/:categorySlug',
        name: 'BlogCategory',
        component: () => import('@/views/blog/BlogList.vue'),
      },
      {
        path: 'tag/:tagSlug',
        name: 'BlogTag',
        component: () => import('@/views/blog/BlogList.vue'),
      },
      {
        path: 'categories',
        name: 'BlogCategories',
        component: () => import('@/views/blog/BlogList.vue'),
      },
      {
        path: 'tags',
        name: 'BlogTags',
        component: () => import('@/views/blog/BlogList.vue'),
      },
    ],
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
        component: () => import('@/views/dashboard/DashboardView.vue'),
        meta: { title: '仪表盘', icon: 'Odometer' },
      },

      // Articles
      {
        path: 'articles',
        name: 'ArticleList',
        component: () => import('@/views/articles/ArticleList.vue'),
        meta: { title: '文章管理', icon: 'Document', permission: 'articles.view' },
      },
      {
        path: 'articles/create',
        name: 'ArticleCreate',
        component: () => import('@/views/articles/ArticleEditor.vue'),
        meta: { title: '写文章', permission: 'articles.create' },
      },
      {
        path: 'articles/:id/edit',
        name: 'ArticleEdit',
        component: () => import('@/views/articles/ArticleEditor.vue'),
        meta: { title: '编辑文章', permission: 'articles.edit' },
      },
      {
        path: 'articles/:id/revisions',
        name: 'ArticleRevisions',
        component: () => import('@/views/articles/ArticleRevisions.vue'),
        meta: { title: '版本历史' },
      },

      // Pages
      {
        path: 'pages',
        name: 'PageList',
        component: () => import('@/views/articles/ArticleList.vue'),
        meta: { title: '页面管理', icon: 'Notebook', postType: 'page' },
      },
      {
        path: 'pages/create',
        name: 'PageCreate',
        component: () => import('@/views/articles/ArticleEditor.vue'),
        meta: { title: '新建页面', postType: 'page' },
      },

      // Categories
      {
        path: 'categories',
        name: 'CategoryList',
        component: () => import('@/views/categories/CategoryList.vue'),
        meta: { title: '分类管理', icon: 'Folder', permission: 'categories.view' },
      },

      // Tags
      {
        path: 'tags',
        name: 'TagList',
        component: () => import('@/views/tags/TagList.vue'),
        meta: { title: '标签管理', icon: 'PriceTag', permission: 'tags.view' },
      },

      // Comments
      {
        path: 'comments',
        name: 'CommentList',
        component: () => import('@/views/comments/CommentList.vue'),
        meta: { title: '评论管理', icon: 'ChatDotSquare', permission: 'comments.view' },
      },

      // Media
      {
        path: 'media',
        name: 'MediaLibrary',
        component: () => import('@/views/media/MediaLibrary.vue'),
        meta: { title: '媒体库', icon: 'Picture', permission: 'media.view' },
      },

      // Users
      {
        path: 'users',
        name: 'UserList',
        component: () => import('@/views/users/UserList.vue'),
        meta: { title: '用户管理', icon: 'User', permission: 'users.view' },
      },
      {
        path: 'users/:id',
        name: 'UserDetail',
        component: () => import('@/views/users/UserDetail.vue'),
        meta: { title: '用户详情', permission: 'users.view' },
      },

      // Roles
      {
        path: 'roles',
        name: 'RoleList',
        component: () => import('@/views/roles/RoleList.vue'),
        meta: { title: '角色权限', icon: 'Lock', permission: 'roles.view' },
      },

      // Menus
      {
        path: 'menus',
        name: 'MenuManager',
        component: () => import('@/views/settings/MenuManager.vue'),
        meta: { title: '导航菜单', icon: 'Menu', permission: 'menus.manage' },
      },

      // SEO
      {
        path: 'seo',
        name: 'SEOManager',
        component: () => import('@/views/seo/SEOManager.vue'),
        meta: { title: 'SEO 管理', icon: 'Search', permission: 'seo.manage' },
      },
      {
        path: 'seo/redirects',
        name: 'RedirectManager',
        component: () => import('@/views/seo/RedirectManager.vue'),
        meta: { title: 'URL 重定向' },
      },

      // Analytics
      {
        path: 'analytics',
        name: 'Analytics',
        component: () => import('@/views/analytics/AnalyticsView.vue'),
        meta: { title: '数据分析', icon: 'TrendCharts', permission: 'analytics.view' },
      },

      // Plugins
      {
        path: 'plugins',
        name: 'PluginList',
        component: () => import('@/views/plugins/PluginList.vue'),
        meta: { title: '插件管理', icon: 'Connection', permission: 'plugins.manage' },
      },

      // Themes
      {
        path: 'themes',
        name: 'ThemeList',
        component: () => import('@/views/themes/ThemeList.vue'),
        meta: { title: '主题管理', icon: 'Brush', permission: 'themes.manage' },
      },

      // Settings
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/settings/SettingsView.vue'),
        meta: { title: '系统设置', icon: 'Setting', permission: 'settings.manage' },
      },

      // Activity Log
      {
        path: 'activity',
        name: 'ActivityLog',
        component: () => import('@/views/settings/ActivityLog.vue'),
        meta: { title: '操作日志', icon: 'Tickets', permission: 'system.activity_log' },
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
router.beforeEach((to, _from, next) => {
  NProgress.start()

  const authStore = useAuthStore()

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }

  if (to.meta.guest && authStore.isAuthenticated) {
    next({ name: 'AdminDashboard' })
    return
  }

  // Check permissions.
  if (to.meta.permission && authStore.isAuthenticated) {
    if (!authStore.hasPermission(to.meta.permission as string)) {
      next({ name: 'AdminDashboard' })
      return
    }
  }

  next()
})

router.afterEach(() => {
  NProgress.done()
})

export default router
