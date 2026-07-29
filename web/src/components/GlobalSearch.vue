<template>
  <el-tooltip content="全局搜索">
    <el-icon
      class="search-trigger"
      @click="open"
    >
      <Search />
    </el-icon>
  </el-tooltip>

  <el-dialog
    v-model="visible"
    title="全局搜索"
    width="640px"
    :append-to-body="true"
    @opened="focusInput"
  >
    <div class="search-bar">
      <el-input
        ref="inputRef"
        v-model="keyword"
        placeholder="搜索文章和页面（支持草稿等全部状态）"
        clearable
        @keyup.enter="search"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-select
        v-model="status"
        style="width: 130px"
        @change="search"
      >
        <el-option
          value=""
          label="全部状态"
        />
        <el-option
          v-for="s in statusOptions"
          :key="s.value"
          :value="s.value"
          :label="s.label"
        />
      </el-select>
      <el-button
        type="primary"
        :loading="searching"
        @click="search"
      >
        搜索
      </el-button>
    </div>

    <div
      v-loading="searching"
      class="search-results"
    >
      <template v-if="result">
        <div class="result-meta">
          共 {{ result.total }} 条结果（耗时 {{ result.took }}）
        </div>
        <div
          v-for="hit in result.hits || []"
          :key="`${hit.type}-${hit.id}`"
          class="result-item"
          @click="openHit(hit)"
        >
          <div class="result-title">
            <el-tag
              size="small"
              :type="hit.type === 'page' ? 'warning' : 'info'"
            >
              {{ hit.type === 'page' ? '页面' : '文章' }}
            </el-tag>
            <span>{{ hit.title }}</span>
          </div>
          <!-- Highlight is NOT HTML-escaped by the backend; split on the
               <mark> wrapper and render as text so article content can never
               inject HTML into the admin (XSS). -->
          <div
            v-if="hit.highlight"
            class="result-excerpt"
          >
            <template
              v-for="(seg, i) in highlightSegments(hit.highlight)"
              :key="i"
            >
              <mark v-if="i % 2 === 1">{{ seg }}</mark>
              <template v-else>
                {{ seg }}
              </template>
            </template>
          </div>
          <div
            v-else-if="hit.excerpt"
            class="result-excerpt"
          >
            {{ hit.excerpt }}
          </div>
        </div>
        <el-empty
          v-if="!result.hits || !result.hits.length"
          description="没有匹配的内容"
          :image-size="80"
        />
      </template>
      <el-empty
        v-else
        description="输入关键词后回车搜索"
        :image-size="80"
      />
    </div>

    <template #footer>
      <div class="search-footer">
        <el-popconfirm
          v-if="authStore.isAdmin"
          title="重建索引会全量扫描数据库，确认执行？"
          width="240"
          @confirm="reindex"
        >
          <template #reference>
            <el-button
              size="small"
              :loading="reindexing"
            >
              <el-icon><Refresh /></el-icon> 重建搜索索引
            </el-button>
          </template>
        </el-popconfirm>
        <span v-else />
        <el-button
          size="small"
          @click="visible = false"
        >
          关闭
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { searchApi, type SearchResult, type SearchHit } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const visible = ref(false)
const keyword = ref('')
const status = ref('')
const searching = ref(false)
const reindexing = ref(false)
const result = ref<SearchResult | null>(null)
const inputRef = ref<{ focus: () => void } | null>(null)

const statusOptions = [
  { value: 'published', label: '已发布' },
  { value: 'draft', label: '草稿' },
  { value: 'pending', label: '待审核' },
  { value: 'scheduled', label: '定时' },
  { value: 'archived', label: '已归档' },
  { value: 'trash', label: '回收站' },
]

function open() {
  visible.value = true
}

function focusInput() {
  inputRef.value?.focus()
}

async function search() {
  const q = keyword.value.trim()
  if (!q) return
  searching.value = true
  try {
    result.value = (await searchApi.admin({ q, status: status.value || undefined, page_size: 20 })).data
  } finally {
    searching.value = false
  }
}

function openHit(hit: SearchHit) {
  visible.value = false
  // Articles and pages share the same editor route.
  router.push(`/admin/articles/${hit.id}/edit`)
}

// Splits "a <mark>b</mark> c" into ['a ', 'b', ' c']; odd indexes are matches.
function highlightSegments(highlight: string): string[] {
  return highlight.split(/<\/?mark>/)
}

async function reindex() {
  reindexing.value = true
  try {
    const res = await searchApi.reindex()
    ElMessage.success(`索引已重建，共索引 ${res.data.indexed} 篇内容`)
  } finally {
    reindexing.value = false
  }
}
</script>

<style lang="scss" scoped>
.search-trigger {
  font-size: 18px;
  cursor: pointer;
  color: var(--el-text-color-regular);
  &:hover { color: var(--el-color-primary); }
}

.search-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.search-results {
  min-height: 200px;
  max-height: 420px;
  overflow-y: auto;

  .result-meta {
    font-size: 12px;
    color: var(--el-text-color-secondary);
    margin-bottom: 8px;
  }

  .result-item {
    padding: 10px 12px;
    border-radius: 6px;
    cursor: pointer;
    &:hover { background: var(--el-fill-color-light); }

    .result-title {
      display: flex;
      align-items: center;
      gap: 8px;
      font-weight: 500;
    }

    .result-excerpt {
      margin-top: 4px;
      font-size: 12px;
      color: var(--el-text-color-secondary);
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;

      mark {
        color: var(--el-color-primary);
        background: transparent;
        font-weight: 600;
      }
    }
  }
}

.search-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
