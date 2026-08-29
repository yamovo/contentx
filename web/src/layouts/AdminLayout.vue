<template>
  <el-container class="admin-layout">
    <!-- Sidebar -->
    <el-aside
      :width="appStore.sidebarCollapsed ? '64px' : '240px'"
      class="admin-sidebar"
    >
      <div class="sidebar-header">
        <img
          v-if="!appStore.sidebarCollapsed"
          src="@/assets/logo.svg"
          alt="Logo"
          class="logo"
        >
        <img
          v-else
          src="@/assets/logo-mini.svg"
          alt="Logo"
          class="logo-mini"
        >
      </div>

      <el-scrollbar>
        <el-menu
          :default-active="activeMenu"
          :default-openeds="defaultOpeneds"
          :collapse="appStore.sidebarCollapsed"
          :collapse-transition="false"
          router
          class="sidebar-menu"
        >
          <template
            v-for="item in menuItems"
            :key="item.path"
          >
            <!-- Single item -->
            <el-menu-item
              v-if="!item.children && hasPermission(item.permission)"
              :index="item.path"
            >
              <el-icon v-if="item.icon">
                <component :is="item.icon" />
              </el-icon>
              <template #title>
                {{ item.title }}
              </template>
            </el-menu-item>

            <!-- Submenu (children are already permission-filtered in menuItems;
                 v-if on the same node as v-for is invalid in Vue 3: v-if is
                 evaluated first and cannot see the v-for variable) -->
            <el-sub-menu
              v-else-if="item.children && item.children.length"
              :index="`group-${item.path}`"
            >
              <template #title>
                <el-icon v-if="item.icon">
                  <component :is="item.icon" />
                </el-icon>
                <span>{{ item.title }}</span>
              </template>
              <el-menu-item
                v-for="child in item.children"
                :key="child.path"
                :index="child.path"
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
      <el-header
        class="admin-header"
        height="56px"
      >
        <div class="header-left">
          <el-icon
            class="collapse-btn"
            @click="appStore.toggleSidebar"
          >
            <Fold v-if="!appStore.sidebarCollapsed" />
            <Expand v-else />
          </el-icon>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/admin' }">
              首页
            </el-breadcrumb-item>
            <el-breadcrumb-item v-if="currentTitle">
              {{ currentTitle }}
            </el-breadcrumb-item>
          </el-breadcrumb>
        </div>

        <div class="header-right">
          <TenantSwitcher />

          <GlobalSearch />

          <el-tooltip content="切换主题">
            <el-icon
              class="header-action"
              @click="appStore.toggleTheme"
            >
              <Sunny v-if="appStore.theme === 'dark'" />
              <Moon v-else />
            </el-icon>
          </el-tooltip>

          <el-dropdown
            trigger="click"
            @command="handleCommand"
          >
            <div class="user-info">
              <el-avatar
                :src="authStore.user?.avatar"
                :size="32"
              >
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
                <el-dropdown-item
                  command="settings"
                  divided
                >
                  <el-icon><Setting /></el-icon>系统设置
                </el-dropdown-item>
                <el-dropdown-item
                  command="logout"
                  divided
                >
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
          <transition
            name="page-slide"
            mode="out-in"
          >
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>

    <!-- ⌘K 全局命令面板 -->
    <CommandPalette />
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import CommandPalette from '@/components/CommandPalette.vue'
import type { PermissionSlug } from '@/shared/auth/permissions'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const activeMenu = computed(() => route.path)
const currentTitle = computed(() => route.meta.title as string || '')

// Menu structure: only grouping + parent group labels live here.
// icon / permission / child titles are sourced from route meta at render
// time, so the menu and route meta no longer maintain duplicate copies of
// those fields (audit F-9).
interface MenuGroupConfig {
  /** Group display label — only for groups with children (leaf items use route meta title). */
  title?: string
  /** Index path; for groups, this is the first child's path. */
  path: string
  /** Child paths; absent means this is a leaf item. */
  children?: string[]
  /** Icon name (Element Plus icon); falls back to route meta.icon. */
  icon?: string
}

const menuConfig: MenuGroupConfig[] = [
  // Dashboard: 子路由 path 为 '' 时，router.getRoutes() 扁平化后 routeMetaMap
  // 拿不到对应的 meta，所以这里显式指定 title/icon 作为兜底。
  { path: '/admin', title: '仪表盘', icon: 'Odometer' },
  {
    title: '内容管理',
    path: '/admin/articles',
    children: ['/admin/articles', '/admin/pages', '/admin/categories', '/admin/tags', '/admin/content-types'],
  },
  { path: '/admin/comments' },
  { path: '/admin/media' },
  {
    title: '用户与权限',
    path: '/admin/users',
    children: ['/admin/users', '/admin/roles'],
  },
  { path: '/admin/menus' },
  { path: '/admin/seo' },
  { path: '/admin/analytics' },
  {
    title: '外观',
    path: '/admin/themes',
    children: ['/admin/themes', '/admin/plugins'],
  },
  {
    title: '系统',
    path: '/admin/settings',
    children: ['/admin/settings', '/admin/activity', '/admin/tokens', '/admin/backup', '/admin/webhooks'],
  },
]

interface ResolvedMenuItem {
  title: string
  path: string
  icon: string
  permission?: PermissionSlug
  adminOnly?: boolean
  children?: ResolvedMenuItem[]
}

// Build a path → meta lookup from the router so we can pull title/icon/permission
// from the single source of truth (route definitions).
const routeMetaMap = computed(() => {
  const map = new Map<string, { title?: string; icon?: string; permission?: PermissionSlug; adminOnly?: boolean }>()
  for (const r of router.getRoutes()) {
    // Only index routes that carry a menu title. Without this filter the
    // parent layout route (path '/admin', empty meta) can overwrite the
    // dashboard child route's entry, producing empty menu titles/icons
    // (and an "invalid vnode type" warning from <component :is="''">).
    if (r.path.startsWith('/admin') && r.meta?.title) {
      map.set(r.path, {
        title: r.meta.title as string,
        icon: r.meta.icon as string | undefined,
        permission: r.meta.permission,
        adminOnly: r.meta.adminOnly as boolean | undefined,
      })
    }
  }
  return map
})

function metaFor(path: string) {
  return routeMetaMap.value.get(path) || {}
}

const menuItems = computed<ResolvedMenuItem[]>(() =>
  menuConfig.map((group) => {
    const meta = metaFor(group.path)
    if (group.children) {
      return {
        title: group.title || meta.title || '',
        path: group.path,
        icon: group.icon || meta.icon || '',
        // Permission filtering happens here (not in the template) so the
        // rendered list is always consistent with the user's permissions.
        children: group.children
          .map((childPath) => {
            const childMeta = metaFor(childPath)
            return {
              title: childMeta.title || '',
              path: childPath,
              icon: childMeta.icon || '',
              permission: childMeta.permission,
              adminOnly: childMeta.adminOnly,
            }
          })
          .filter((child) => hasPermission(child.permission) && (!child.adminOnly || authStore.isAdmin)),
      }
    }
    return {
      title: group.title || meta.title || '',
      path: group.path,
      icon: group.icon || meta.icon || '',
      permission: meta.permission,
    }
  }),
)

// 默认展开所有有子菜单的项（el-sub-menu 的 index 为 `group-${path}`）
const defaultOpeneds = computed(() =>
  menuItems.value
    .filter((item) => item.children && item.children.length)
    .map((item) => `group-${item.path}`),
)

function hasPermission(perm?: PermissionSlug): boolean {
  if (!perm) return true
  return authStore.hasPermission(perm)
}

async function handleCommand(cmd: string) {
  switch (cmd) {
    case 'profile':
      router.push('/admin/profile')
      break
    case 'settings':
      router.push('/admin/settings')
      break
    case 'logout':
      await authStore.logout()
      router.push('/login')
      break
  }
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

  // The header above is 56px tall; without subtracting it the scrollbar
  // is 56px taller than the visible area and the last menu items can
  // never be scrolled into view.
  :deep(.el-scrollbar) {
    height: calc(100% - 56px);
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
  transition: opacity 0.3s ease, transform 0.3s ease;
  will-change: opacity, transform;
}

.page-slide-enter-from {
  opacity: 0;
  transform: translateX(12px);
}

.page-slide-leave-to {
  opacity: 0;
  transform: translateX(-8px);
}
</style>
