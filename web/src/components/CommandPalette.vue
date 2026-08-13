<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="palette-overlay"
      @click.self="close"
    >
      <div class="palette-panel">
        <div class="palette-input-row">
          <el-icon class="palette-search-icon">
            <Search />
          </el-icon>
          <input
            ref="inputRef"
            v-model="keyword"
            class="palette-input"
            placeholder="搜索页面或执行命令..."
            @keydown.down.prevent="move(1)"
            @keydown.up.prevent="move(-1)"
            @keydown.enter.prevent="execute(activeIndex)"
            @keydown.esc="close"
          >
          <kbd class="palette-kbd">ESC</kbd>
        </div>

        <div class="palette-results">
          <template v-if="filteredPages.length">
            <div class="palette-group-title">
              页面
            </div>
            <div
              v-for="item in filteredPages"
              :key="`page-${item.path}`"
              class="palette-item"
              :class="{ active: item.index === activeIndex }"
              @click="execute(item.index)"
              @mouseenter="activeIndex = item.index"
            >
              <el-icon v-if="item.icon">
                <component :is="item.icon" />
              </el-icon>
              <span class="palette-item-label">{{ item.title }}</span>
              <span class="palette-item-path">{{ item.path }}</span>
            </div>
          </template>

          <template v-if="filteredCommands.length">
            <div class="palette-group-title">
              快捷命令
            </div>
            <div
              v-for="item in filteredCommands"
              :key="`cmd-${item.key}`"
              class="palette-item"
              :class="{ active: item.index === activeIndex }"
              @click="execute(item.index)"
              @mouseenter="activeIndex = item.index"
            >
              <el-icon>
                <component :is="item.icon" />
              </el-icon>
              <span class="palette-item-label">{{ item.title }}</span>
            </div>
          </template>

          <div
            v-if="!allItems.length"
            class="palette-empty"
          >
            没有匹配的结果
          </div>
        </div>

        <div class="palette-footer">
          <span><kbd class="palette-kbd">↑↓</kbd> 选择</span>
          <span><kbd class="palette-kbd">Enter</kbd> 执行</span>
          <span><kbd class="palette-kbd">ESC</kbd> 关闭</span>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { PERMISSIONS, type PermissionSlug } from '@/shared/auth/permissions'

interface PageItem {
  kind: 'page'
  index: number
  title: string
  path: string
  icon?: string
}

interface CommandItem {
  kind: 'command'
  index: number
  key: string
  title: string
  icon: string
  run: () => void
}

type PaletteItem = PageItem | CommandItem

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const visible = ref(false)
const keyword = ref('')
const activeIndex = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)

// 页面项来自路由表（单一事实来源），按 meta.permission / adminOnly
// 过滤掉当前用户无权访问的项；带参数的动态路由（:id）无法直接跳转，排除。
const pageItems = computed<Omit<PageItem, 'index'>[]>(() => {
  const items: Omit<PageItem, 'index'>[] = []
  const seen = new Set<string>()
  for (const r of router.getRoutes()) {
    if (!r.path.startsWith('/admin') || r.path.includes(':')) continue
    if (!r.meta?.title || seen.has(r.path)) continue
    const perm = r.meta.permission as PermissionSlug | undefined
    if (perm && !authStore.hasPermission(perm)) continue
    if (r.meta.adminOnly && !authStore.isAdmin) continue
    seen.add(r.path)
    items.push({
      kind: 'page',
      title: r.meta.title as string,
      path: r.path,
      icon: r.meta.icon as string | undefined,
    })
  }
  return items
})

const commandItems = computed<Omit<CommandItem, 'index'>[]>(() => {
  const items: Omit<CommandItem, 'index'>[] = []
  if (authStore.hasPermission(PERMISSIONS.articles.create)) {
    items.push({
      kind: 'command',
      key: 'create-article',
      title: '新建文章',
      icon: 'EditPen',
      run: () => router.push('/admin/articles/create'),
    })
  }
  items.push({
    kind: 'command',
    key: 'toggle-theme',
    title: appStore.theme === 'dark' ? '切换主题（切到亮色）' : '切换主题（切到暗色）',
    icon: 'Sunny',
    run: () => appStore.toggleTheme(),
  })
  return items
})

function matches(title: string) {
  const q = keyword.value.trim().toLowerCase()
  return !q || title.toLowerCase().includes(q)
}

// 两组过滤后统一编号，键盘 ↑↓ 在整张列表上移动。
const allItems = computed<PaletteItem[]>(() => {
  const pages = pageItems.value.filter((p) => matches(p.title))
  const commands = commandItems.value.filter((c) => matches(c.title))
  return [...pages, ...commands].map((item, index) => ({ ...item, index })) as PaletteItem[]
})

const filteredPages = computed(() => allItems.value.filter((i) => i.kind === 'page'))
const filteredCommands = computed(() => allItems.value.filter((i) => i.kind === 'command'))

watch(keyword, () => {
  activeIndex.value = 0
})

function open() {
  visible.value = true
  keyword.value = ''
  activeIndex.value = 0
  nextTick(() => inputRef.value?.focus())
}

function close() {
  visible.value = false
}

function move(delta: number) {
  const count = allItems.value.length
  if (!count) return
  activeIndex.value = (activeIndex.value + delta + count) % count
}

function execute(index: number) {
  const item = allItems.value[index]
  if (!item) return
  close()
  if (item.kind === 'page') {
    router.push(item.path)
  } else {
    item.run()
  }
}

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    if (visible.value) {
      close()
    } else {
      open()
    }
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onUnmounted(() => window.removeEventListener('keydown', onGlobalKeydown))
</script>

<style lang="scss" scoped>
.palette-overlay {
  position: fixed;
  inset: 0;
  z-index: 3000;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding-top: 12vh;
}

.palette-panel {
  width: 560px;
  max-width: calc(100vw - 32px);
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color);
  border-radius: 10px;
  box-shadow: var(--el-box-shadow-dark);
  overflow: hidden;
}

.palette-input-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--el-border-color-light);

  .palette-search-icon {
    font-size: 16px;
    color: var(--el-text-color-secondary);
  }

  .palette-input {
    flex: 1;
    border: none;
    outline: none;
    background: transparent;
    font-size: 15px;
    color: var(--el-text-color-primary);
  }
}

.palette-kbd {
  padding: 2px 6px;
  font-size: 11px;
  font-family: inherit;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
}

.palette-results {
  max-height: 360px;
  overflow-y: auto;
  padding: 6px;

  .palette-group-title {
    padding: 8px 10px 4px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .palette-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 10px;
    border-radius: 6px;
    cursor: pointer;
    color: var(--el-text-color-primary);

    &.active {
      background: var(--el-fill-color-light);
    }

    .palette-item-label {
      flex: 1;
      font-size: 14px;
    }

    .palette-item-path {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }

  .palette-empty {
    padding: 32px 0;
    text-align: center;
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }
}

.palette-footer {
  display: flex;
  gap: 16px;
  padding: 8px 16px;
  border-top: 1px solid var(--el-border-color-light);
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
