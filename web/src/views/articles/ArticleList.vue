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
            v-model="filters.status"
            placeholder="状态"
            clearable
          >
            <el-option
              label="已发布"
              value="published"
            />
            <el-option
              label="草稿"
              value="draft"
            />
            <el-option
              label="待审核"
              value="pending"
            />
            <el-option
              label="定时发布"
              value="scheduled"
            />
            <el-option
              label="回收站"
              value="trash"
            />
          </el-select>
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

    <!-- Bulk Actions -->
    <div
      v-if="selectedIds.length > 0"
      class="bulk-actions"
    >
      <span>已选择 {{ selectedIds.length }} 项</span>
      <el-button
        size="small"
        @click="bulkAction('publish')"
      >
        发布
      </el-button>
      <el-button
        size="small"
        @click="bulkAction('draft')"
      >
        转为草稿
      </el-button>
      <el-button
        size="small"
        @click="bulkAction('trash')"
      >
        移至回收站
      </el-button>
      <el-popconfirm
        title="确认删除？"
        @confirm="bulkAction('delete')"
      >
        <template #reference>
          <el-button
            size="small"
            type="danger"
          >
            删除
          </el-button>
        </template>
      </el-popconfirm>
    </div>

    <!-- Table -->
    <el-card shadow="never">
      <el-table
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

const filters = reactive({
  search: '',
  status: '',
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

const { updateArticle, deleteArticle, bulkAction: bulkActionMutation, publishArticle } = useArticleMutations()

function handleSelectionChange(rows: Article[]) {
  selectedIds.value = rows.map(r => r.id)
}

async function bulkAction(action: string) {
  try {
    await bulkActionMutation.mutateAsync({
      article_ids: selectedIds.value,
      action: action as 'publish' | 'unpublish' | 'draft' | 'trash' | 'delete',
    })
    ElMessage.success('操作成功')
  } catch {
    ElMessage.error('操作失败')
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
    trash: 'danger',
  }
  return tones[s] || 'neutral'
}
function statusLabel(s: string) {
  return { published: '已发布', draft: '草稿', pending: '待审', scheduled: '定时', trash: '回收站' }[s] || s
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

  .bulk-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
    padding: 12px 16px;
    background: #ecf5ff;
    border-radius: 4px;
    font-size: 13px;
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
