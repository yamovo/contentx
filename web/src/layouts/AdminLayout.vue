<template>
  <el-container class="admin-layout">
    <!-- Sidebar -->
    <el-aside :width="appStore.sidebarCollapsed ? '64px' : '240px'" class="admin-sidebar">
      <div class="sidebar-header">
        <img src="@/assets/logo.svg" alt="Logo" class="logo" v-if="!appStore.sidebarCollapsed" />
        <img src="@/assets/logo-mini.svg" alt="Logo" class="logo-mini" v-else />
      </div>

      <el-scrollbar>
        <el-menu
          :default-active="activeMenu"
          :collapse="appStore.sidebarCollapsed"
          :collapse-transition="false"
          router
          class="sidebar-menu"
        >
          <template v-for="item in menuItems" :key="item.path">
            <!-- Single item -->
            <el-menu-item
              v-if="!item.children"
              :index="item.path"
              v-show="hasPermission(item.permission)"
            >
              <el-icon><component :is="item.icon" /></el-icon>
              <template #title>{{ item.title }}</template>
            </el-menu-item>

            <!-- Submenu -->
            <el-sub-menu
              v-else
              :index="item.path"
              v-show="hasAnyPermission(item.children)"
            >
              <template #title>
                <el-icon><component :is="item.icon" /></el-icon>
                <span>{{ item.title }}</span>
              </template>
              <el-menu-item
                v-for="child in item.children"
                :key="child.path"
                :index="child.path"
                v-show="hasPermission(child.permission)"
              >
                {{ child.title }}
              </el-menu-item>
            </el-sub-menu>
          </template>
        </el-menu>
      </el-scrollbar>
    </el-aside>

    <!-- Main content -->
    <el-container class="admin-main-container">
      <!-- Header -->
      <el-header class="admin-header" height="56px">
        <div class="header-left">
          <el-icon class="collapse-btn" @click="appStore.toggleSidebar">
            <Fold v-if="!appStore.sidebarCollapsed" />
            <Expand v-else />
          </el-icon>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/admin' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item v-if="currentTitle">{{ currentTitle }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>

        <div class="header-right">
          <el-tooltip content="切换主题">
            <el-icon class="header-action" @click="appStore.toggleTheme">
              <Sunny v-if="appStore.theme === 'dark'" />
              <Moon v-else />
            </el-icon>
          </el-tooltip>

          <el-dropdown trigger="click" @command="handleCommand">
            <div class="user-info">
              <el-avatar :src="authStore.user?.avatar" :size="32">
                {{ authStore.user?.display_name?.[0] || 'U' }}
              </el-avatar>
              <span class="username">{{ authStore.user?.display_name || authStore.user?.username }}</span>
              <el-icon><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">
                  <el-icon><User /></el-icon>个人资料
                </el-dropdown-item>
                <el-dropdown-item command="settings" divided>
                  <el-icon><Setting /></el-icon>系统设置
                </el-dropdown-item>
                <el-dropdown-item command="logout" divided>
                  <el-icon><SwitchButton /></el-icon>退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- Content -->
      <el-main class="admin-content">
        <router-view v-slot="{ Component }">
          <transition name="page-slide" mode="out-in" @before-enter="onBeforeEnter" @enter="onEnter" @leave="onLeave">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { animate } from 'animejs'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const activeMenu = computed(() => route.path)
const currentTitle = computed(() => route.meta.title as string || '')

interface MenuItem {
  title: string
  path: string
  icon: string
  permission?: string
  children?: { title: string; path: string; permission?: string }[]
}

const menuItems: MenuItem[] = [
  { title: '仪表盘', path: '/admin', icon: 'Odometer' },
  {
    title: '内容管理', path: '/admin/articles', icon: 'Document',
    children: [
      { title: '文章', path: '/admin/articles', permission: 'articles.view' },
      { title: '页面', path: '/admin/pages' },
      { title: '分类', path: '/admin/categories', permission: 'categories.view' },
      { title: '标签', path: '/admin/tags', permission: 'tags.view' },
    ],
  },
  { title: '评论管理', path: '/admin/comments', icon: 'ChatDotSquare', permission: 'comments.view' },
  { title: '媒体库', path: '/admin/media', icon: 'Picture', permission: 'media.view' },
  {
    title: '用户与权限', path: '/admin/users', icon: 'User',
    children: [
      { title: '用户管理', path: '/admin/users', permission: 'users.view' },
      { title: '角色权限', path: '/admin/roles', permission: 'roles.view' },
    ],
  },
  { title: '导航菜单', path: '/admin/menus', icon: 'Menu', permission: 'menus.manage' },
  { title: 'SEO 管理', path: '/admin/seo', icon: 'Search', permission: 'seo.manage' },
  { title: '数据分析', path: '/admin/analytics', icon: 'TrendCharts', permission: 'analytics.view' },
  {
    title: '外观', path: '/admin/themes', icon: 'Brush',
    children: [
      { title: '主题管理', path: '/admin/themes', permission: 'themes.manage' },
      { title: '插件管理', path: '/admin/plugins', permission: 'plugins.manage' },
    ],
  },
  {
    title: '系统', path: '/admin/settings', icon: 'Setting',
    children: [
      { title: '系统设置', path: '/admin/settings', permission: 'settings.manage' },
      { title: '操作日志', path: '/admin/activity', permission: 'system.activity_log' },
    ],
  },
]

function hasPermission(perm?: string): boolean {
  if (!perm) return true
  return authStore.hasPermission(perm)
}

function hasAnyPermission(children?: { permission?: string }[]): boolean {
  if (!children) return true
  return children.some(c => hasPermission(c.permission))
}

function handleCommand(cmd: string) {
  switch (cmd) {
    case 'profile':
      // Navigate to profile
      break
    case 'settings':
      router.push('/admin/settings')
      break
    case 'logout':
      authStore.logout()
      router.push('/login')
      break
  }
}


function onBeforeEnter(el: Element) {
  (el as HTMLElement).style.opacity = '0'
  (el as HTMLElement).style.transform = 'translateX(12px)'
}

function onEnter(el: Element, done: () => void) {
  animate(el as HTMLElement, {
    opacity: { from: 0 },
    translateX: { from: 12 },
    duration: 350,
    ease: 'outQuint',
    onComplete: done,
  })
}

function onLeave(el: Element, done: () => void) {
  animate(el as HTMLElement, {
    opacity: 0,
    translateX: -8,
    duration: 200,
    ease: 'inQuad',
    onComplete: done,
  })
}
</script>

<style lang="scss" scoped>
.admin-layout {
  height: 100vh;
  overflow: hidden;
}

.admin-sidebar {
  background: var(--sidebar-bg, #1d1e2c);
  transition: width 0.3s;
  overflow: hidden;

  .sidebar-header {
    height: 56px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);

    .logo {
      height: 28px;
    }
    .logo-mini {
      height: 28px;
    }
  }

  .sidebar-menu {
    border-right: none;
    background: transparent;

    // Override Element Plus dark mode CSS variables so the sidebar
    // always keeps its own dark styling regardless of theme.
    --el-menu-bg-color: transparent;
    --el-menu-text-color: var(--sidebar-text, #a3a6b4);
    --el-menu-active-color: var(--sidebar-active, #409eff);
    --el-menu-hover-bg-color: rgba(255, 255, 255, 0.05);
    --el-menu-hover-text-color: #fff;
    --el-sub-menu-bg-color: transparent;

    :deep(.el-menu-item),
    :deep(.el-sub-menu__title) {
      color: var(--sidebar-text, #a3a6b4) !important;
      background: transparent !important;
      &:hover {
        background: rgba(255, 255, 255, 0.05) !important;
        color: #fff !important;
      }
      &.is-active {
        background: rgba(64, 158, 255, 0.15) !important;
        color: var(--sidebar-active, #409eff) !important;
      }
    }

    // Submenu expanded children
    :deep(.el-menu--inline) {
      background: rgba(0, 0, 0, 0.15) !important;

      .el-menu-item {
        padding-left: 48px !important;
        color: var(--sidebar-text, #a3a6b4) !important;
        background: transparent !important;
        &:hover {
          background: rgba(255, 255, 255, 0.05) !important;
          color: #fff !important;
        }
        &.is-active {
          background: rgba(64, 158, 255, 0.15) !important;
          color: var(--sidebar-active, #409eff) !important;
        }
      }
    }
  }
}

.admin-main-container {
  flex-direction: column;
  overflow: hidden;
}

.admin-header {
  background: var(--header-bg, #fff);
  border-bottom: 1px solid var(--border-color, #e4e7ed);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;

  .header-left {
    display: flex;
    align-items: center;
    gap: 16px;

    .collapse-btn {
      font-size: 20px;
      cursor: pointer;
      color: var(--text-secondary, #606266);
      &:hover { color: #409eff; }
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 20px;

    .header-action {
      font-size: 18px;
      cursor: pointer;
      color: var(--text-secondary, #606266);
      &:hover { color: #409eff; }
    }

    .user-info {
      display: flex;
      align-items: center;
      gap: 8px;
      cursor: pointer;

      .username {
        font-size: 14px;
        color: var(--text-primary, #303133);
      }
    }
  }
}

.admin-content {
  background: var(--bg-primary, #f5f7fa);
  overflow-y: auto;
  padding: 20px;
}

.page-slide-enter-active,
.page-slide-leave-active {
  will-change: opacity, transform;
}
</style>
