<template>
  <div class="article-list-page">
    <!-- Header -->
    <PageHeader :title="pageTitle">
      <template #actions>
        <el-button
          type="primary"
          @click="$router.push(createPath)"
        >
          <el-icon><Plus /></el-icon> {{ createLabel }}
        </el-button>
      </template>
    </PageHeader>

    <!-- Filters -->
    <el-card
      shadow="never"
      class="filter-card"
    >
      <el-form
        :inline="true"
        :model="filters"
        @submit.prevent
      >
        <el-form-item>
          <el-input
            v-model="filters.search"
            placeholder="搜索文章..."
            :prefix-icon="Search"
            clearable
          />
        </el-form-item>
        <el-form-item>
          <el-select
            v-model="filters.category_id"
            placeholder="分类"
            clearable
          >
            <el-option
              v-for="cat in categories"
              :key="cat.id"
              :label="cat.name"
              :value="cat.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-select
            v-model="filters.sort"
            placeholder="排序"
          >
            <el-option
              label="最新"
              value="newest"
            />
            <el-option
              label="最旧"
              value="oldest"
            />
            <el-option
              label="标题"
              value="title"
            />
            <el-option
              label="最多浏览"
              value="views"
            />
            <el-option
              label="最多点赞"
              value="likes"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
          >
            搜索
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Status Tabs -->
    <el-card
      shadow="never"
      class="tabs-card"
    >
      <el-tabs
        :model-value="filters.status || 'all'"
        @tab-change="handleStatusTabChange"
      >
        <el-tab-pane
          v-for="tab in statusTabs"
          :key="tab.value"
          :name="tab.value || 'all'"
        >
          <template #label>
            {{ tab.label }}
            <span
              v-if="statusCounts[tab.value] != null"
              class="tab-count"
            >{{ statusCounts[tab.value] }}</span>
          </template>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- Bulk Actions -->
    <div
      v-if="selectedIds.length > 0"
      class="bulk-actions"
    >
      <span>已选择 {{ selectedIds.length }} 项</span>
      <el-button
        size="small"
        type="primary"
        @click="bulkAction('publish')"
      >
        批量发布
      </el-button>
      <el-button
        size="small"
        :loading="bulkArchiving"
        @click="bulkArchive"
      >
        批量归档
      </el-button>
      <el-button
        size="small"
        type="danger"
        @click="bulkDelete"
      >
        批量删除
      </el-button>
      <el-button
        size="small"
        text
        class="bulk-cancel"
        @click="clearSelection"
      >
        取消选择
      </el-button>
    </div>

    <!-- Table -->
    <el-card shadow="never">
      <el-table
        ref="tableRef"
        v-loading="loading"
        :data="articles"
        row-key="id"
        stripe
        @selection-change="handleSelectionChange"
      >
        <el-table-column
          type="selection"
          width="50"
        />
        <el-table-column
          label="标题"
          min-width="300"
        >
          <template #default="{ row }">
            <div class="article-title-cell">
              <router-link
                :to="`/admin/articles/${row.id}/edit`"
                class="title-link"
              >
                <el-icon
                  v-if="row.is_pinned"
                  class="pin-icon"
                >
                  <Top />
                </el-icon>
                <el-icon
                  v-if="row.is_featured"
                  class="featured-icon"
                >
                  <StarFilled />
                </el-icon>
                {{ row.title }}
              </router-link>
              <div class="article-meta">
                <span>{{ row.author?.display_name }}</span>
                <span>·</span>
                <span>{{ row.category?.name || '未分类' }}</span>
                <span>·</span>
                <span>{{ formatDate(row.created_at) }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="100"
        >
          <template #default="{ row }">
            <StatusBadge
              :label="statusLabel(row.status)"
              :tone="statusTone(row.status)"
            />
          </template>
        </el-table-column>
        <el-table-column
          label="浏览"
          prop="view_count"
          width="80"
          align="center"
        />
        <el-table-column
          label="评论"
          prop="comment_count"
          width="80"
          align="center"
        />
        <el-table-column
          label="标签"
          width="200"
        >
          <template #default="{ row }">
            <el-tag
              v-for="tag in row.tags?.slice(0, 3)"
              :key="tag.id"
              size="small"
              class="tag-item"
            >
              {{ tag.name }}
            </el-tag>
            <el-tag
              v-if="row.tags?.length > 3"
              size="small"
              type="info"
            >
              +{{ row.tags.length - 3 }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="200"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              text
              type="primary"
              size="small"
              @click="$router.push(`/admin/articles/${row.id}/edit`)"
            >
              编辑
            </el-button>
            <el-button
              text
              size="small"
              @click="$router.push(`/admin/articles/${row.id}/revisions`)"
            >
              历史
            </el-button>
            <el-dropdown
              trigger="click"
              @command="(cmd: string) => handleCommand(cmd, row as Article)"
            >
              <el-button
                text
                size="small"
              >
                更多
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item
                    v-if="row.status !== 'published'"
                    command="publish"
                  >
                    发布
                  </el-dropdown-item>
                  <el-dropdown-item command="pin">
                    {{ row.is_pinned ? '取消置顶' : '置顶' }}
                  </el-dropdown-item>
                  <el-dropdown-item command="feature">
                    {{ row.is_featured ? '取消精选' : '精选' }}
                  </el-dropdown-item>
                  <el-dropdown-item
                    command="view"
                    divided
                  >
                    查看
                  </el-dropdown-item>
                  <el-dropdown-item
                    command="delete"
                    divided
                  >
                    <span style="color: #f56c6c">删除</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Plus, Search, StarFilled, Top } from '@element-plus/icons-vue'
import { type Article } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { formatDate } from '@/utils'
import { PageHeader, StatusBadge } from '@/shared/ui'
import { useArticleListQuery } from '@/features/articles/use-article-list-query'
import { useArticleMutations } from '@/features/articles/use-article-mutations'
import { useCategoryListQuery } from '@/features/categories/use-category-list-query'

const route = useRoute()
const postType = computed(() => route.meta.postType as string || 'post')

const pageTitle = computed(() => postType.value === 'page' ? '页面管理' : '文章管理')
const createLabel = computed(() => postType.value === 'page' ? '新建页面' : '写文章')
const createPath = computed(() => postType.value === 'page' ? '/admin/pages/create' : '/admin/articles/create')

const page = ref(1)
const pageSize = ref(20)
const selectedIds = ref<number[]>([])
const tableRef = ref<{ clearSelection: () => void } | null>(null)
const bulkArchiving = ref(false)

// 支持从仪表盘等待办入口带 ?status=pending 深链进来
const filters = reactive({
  search: '',
  status: (route.query.status as string) || '',
  category_id: '',
  sort: 'newest',
})

// Vue Query: reactive params drive the queryKey → automatic refetch on change
const queryParams = computed(() => ({
  page: page.value,
  page_size: pageSize.value,
  post_type: postType.value,
  search: filters.search,
  status: filters.status,
  category_id: filters.category_id,
  sort: filters.sort,
}))

const { data: articleData, isLoading: loading } = useArticleListQuery(queryParams)
const articles = computed(() => articleData.value?.items || [])
const total = computed(() => articleData.value?.total || 0)

const { data: categoriesData } = useCategoryListQuery({})
const categories = computed(() => categoriesData.value?.data || [])

// ─── 状态 Tabs ──────────────────────────────────────────
const statusTabs = [
  { value: '', label: '全部' },
  { value: 'published', label: '已发布' },
  { value: 'draft', label: '草稿' },
  { value: 'pending', label: '待审核' },
  { value: 'archived', label: '已归档' },
]

// 每个 Tab 的数量徽标：复用列表查询层，page_size=1 只为拿 total，
// 共享当前搜索/分类筛选（不含状态），切 Tab 时由 queryKey 自动缓存。
function useStatusCount(status: string) {
  const params = computed(() => ({
    page: 1,
    page_size: 1,
    post_type: postType.value,
    search: filters.search,
    status,
    category_id: filters.category_id,
  }))
  const { data } = useArticleListQuery(params)
  return computed(() => data.value?.total ?? null)
}

const allCount = useStatusCount('')
const publishedCount = useStatusCount('published')
const draftCount = useStatusCount('draft')
const pendingCount = useStatusCount('pending')
const archivedCount = useStatusCount('archived')

const statusCounts = computed<Record<string, number | null>>(() => ({
  '': allCount.value,
  published: publishedCount.value,
  draft: draftCount.value,
  pending: pendingCount.value,
  archived: archivedCount.value,
}))

function handleStatusTabChange(name: string | number) {
  filters.status = name === 'all' ? '' : String(name)
  page.value = 1
}

const { updateArticle, deleteArticle, bulkAction: bulkActionMutation, publishArticle, archiveArticle } = useArticleMutations()

function handleSelectionChange(rows: Article[]) {
  selectedIds.value = rows.map(r => r.id)
}

function clearSelection() {
  tableRef.value?.clearSelection()
  selectedIds.value = []
}

async function bulkAction(action: string) {
  try {
    await bulkActionMutation.mutateAsync({
      article_ids: selectedIds.value,
      action: action as 'publish' | 'unpublish' | 'draft' | 'trash' | 'delete',
    })
    ElMessage.success('操作成功')
    clearSelection()
  } catch {
    ElMessage.error('操作失败')
  }
}

async function bulkDelete() {
  try {
    await ElMessageBox.confirm(
      `确认删除选中的 ${selectedIds.value.length} 项？删除后不可恢复。`,
      '批量删除',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  await bulkAction('delete')
}

// 后端批量接口不支持 archive，逐条调用单条归档接口并汇总结果。
async function bulkArchive() {
  bulkArchiving.value = true
  try {
    const results = await Promise.allSettled(
      selectedIds.value.map((id) => archiveArticle.mutateAsync(id)),
    )
    const failed = results.filter((r) => r.status === 'rejected').length
    const succeeded = results.length - failed
    if (failed === 0) {
      ElMessage.success(`已归档 ${succeeded} 项`)
    } else {
      ElMessage.warning(`归档完成：成功 ${succeeded} 项，失败 ${failed} 项`)
    }
    clearSelection()
  } finally {
    bulkArchiving.value = false
  }
}

async function handleCommand(cmd: string, article: Article) {
  switch (cmd) {
    case 'publish':
      await publishArticle.mutateAsync(article.id)
      ElMessage.success('已发布')
      break
    case 'pin':
      await updateArticle.mutateAsync({ id: article.id, is_pinned: !article.is_pinned })
      break
    case 'feature':
      await updateArticle.mutateAsync({ id: article.id, is_featured: !article.is_featured })
      break
    case 'view':
      window.open(`/blog/article/${article.slug}`, '_blank')
      break
    case 'delete':
      await ElMessageBox.confirm('确认删除此文章？', '确认')
      await deleteArticle.mutateAsync(article.id)
      ElMessage.success('已删除')
      break
  }
}

function statusTone(s: string): 'success' | 'info' | 'warning' | 'danger' | 'neutral' {
  const tones: Record<string, 'success' | 'info' | 'warning' | 'danger' | 'neutral'> = {
    published: 'success',
    draft: 'info',
    pending: 'warning',
    scheduled: 'warning',
    archived: 'neutral',
    trash: 'danger',
  }
  return tones[s] || 'neutral'
}
function statusLabel(s: string) {
  return { published: '已发布', draft: '草稿', pending: '待审', scheduled: '定时', archived: '已归档', trash: '回收站' }[s] || s
}

// 同一组件实例在 /admin/articles ↔ /admin/pages 间切换时，重置筛选
// Vue Query 自动因 postType 变化（queryKey 改变）而重新请求
watch(postType, () => {
  page.value = 1
  filters.search = ''
  filters.status = ''
  filters.category_id = ''
})
</script>

<style lang="scss" scoped>
.article-list-page {
  .filter-card { margin-bottom: 16px; }

  .tabs-card {
    margin-bottom: 16px;
    :deep(.el-card__body) { padding: 0 20px; }
    :deep(.el-tabs__header) { margin-bottom: 0; }
    :deep(.el-tabs__nav-wrap::after) { display: none; }

    .tab-count {
      display: inline-block;
      margin-left: 6px;
      padding: 0 6px;
      font-size: 12px;
      line-height: 18px;
      border-radius: 9px;
      background: var(--el-fill-color);
      color: var(--el-text-color-secondary);
    }
  }

  .bulk-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
    padding: 12px 16px;
    background: var(--el-color-primary-light-9);
    border: 1px solid var(--el-color-primary-light-7);
    border-radius: 4px;
    font-size: 13px;

    .bulk-cancel { margin-left: auto; }
  }

  .article-title-cell {
    .title-link {
      color: #303133;
      text-decoration: none;
      font-weight: 500;
      &:hover { color: #409eff; }
    }
    .pin-icon { color: #e6a23c; margin-right: 4px; }
    .featured-icon { color: #f56c6c; margin-right: 4px; }
    .article-meta {
      font-size: 12px;
      color: #909399;
      margin-top: 4px;
      display: flex;
      gap: 6px;
    }
  }

  .tag-item { margin-right: 4px; margin-bottom: 2px; }

  .pagination-wrapper {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }
}
</style>
