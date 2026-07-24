<template>
  <div class="comment-page">
    <div class="page-header">
      <h2>评论管理</h2>
      <div
        v-if="stats"
        class="comment-stats"
      >
        <el-tag>全部 {{ stats.total }}</el-tag>
        <el-tag type="warning">
          待审 {{ stats.pending }}
        </el-tag>
        <el-tag type="success">
          已批准 {{ stats.approved }}
        </el-tag>
        <el-tag type="danger">
          垃圾 {{ stats.spam }}
        </el-tag>
      </div>
    </div>

    <el-card shadow="never">
      <el-form
        :inline="true"
        class="filter-form"
      >
        <el-form-item>
          <el-select
            v-model="filters.status"
            placeholder="状态"
            clearable
            @change="fetchComments"
          >
            <el-option
              label="待审核"
              value="pending"
            />
            <el-option
              label="已批准"
              value="approved"
            />
            <el-option
              label="垃圾"
              value="spam"
            />
            <el-option
              label="回收站"
              value="trash"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="filters.search"
            placeholder="搜索评论..."
            clearable
            @clear="fetchComments"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            @click="fetchComments"
          >
            搜索
          </el-button>
        </el-form-item>
      </el-form>

      <!-- Bulk Actions -->
      <div
        v-if="selectedIds.length"
        class="bulk-bar"
      >
        <span>已选 {{ selectedIds.length }} 项</span>
        <el-button
          size="small"
          @click="bulk('approve')"
        >
          批准
        </el-button>
        <el-button
          size="small"
          @click="bulk('spam')"
        >
          标记垃圾
        </el-button>
        <el-button
          size="small"
          type="danger"
          @click="bulk('delete')"
        >
          删除
        </el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="comments"
        @selection-change="(rows: Comment[]) => selectedIds = rows.map(r => r.id)"
      >
        <el-table-column
          type="selection"
          width="50"
        />
        <el-table-column
          label="评论内容"
          min-width="300"
        >
          <template #default="{ row }">
            <div class="comment-cell">
              <div class="comment-author">
                <el-avatar :size="28">
                  {{ (row.author_name || 'A')[0] }}
                </el-avatar>
                <div>
                  <strong>{{ row.author_name || row.user?.display_name || '匿名' }}</strong>
                  <span class="comment-email">{{ row.author_email }}</span>
                </div>
              </div>
              <p class="comment-text">
                {{ row.content }}
              </p>
              <div class="comment-meta">
                文章: <router-link :to="`/admin/articles/${row.article_id}/edit`">
                  {{ row.article?.title || `#${row.article_id}` }}
                </router-link>
                · {{ formatDate(row.created_at, 'MM-DD HH:mm') }}
                · IP: {{ row.author_ip }}
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="100"
        >
          <template #default="{ row }">
            <el-tag
              :type="statusType(row.status)"
              size="small"
            >
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="240"
        >
          <template #default="{ row }">
            <el-button
              v-if="row.status !== 'approved'"
              text
              size="small"
              type="success"
              @click="approve(row.id)"
            >
              批准
            </el-button>
            <el-button
              v-if="row.status !== 'spam'"
              text
              size="small"
              type="warning"
              @click="markSpam(row.id)"
            >
              垃圾
            </el-button>
            <el-button
              text
              size="small"
              @click="replyTo(row as Comment)"
            >
              回复
            </el-button>
            <el-popconfirm
              title="确认删除？"
              @confirm="deleteComment(row.id)"
            >
              <template #reference>
                <el-button
                  text
                  size="small"
                  type="danger"
                >
                  删除
                </el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="fetchComments"
        />
      </div>
    </el-card>

    <!-- Reply Dialog -->
    <el-dialog
      v-model="replyVisible"
      title="回复评论"
      width="500px"
    >
      <p
        v-if="replyTarget"
        class="reply-context"
      >
        <strong>{{ replyTarget.author_name }}</strong>: {{ replyTarget.content }}
      </p>
      <el-input
        v-model="replyContent"
        type="textarea"
        :rows="4"
        placeholder="输入回复内容..."
      />
      <template #footer>
        <el-button @click="replyVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          @click="submitReply"
        >
          回复
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { commentApi, type Comment } from '@/api'
import { ElMessage } from 'element-plus'
import { formatDate } from '@/utils'

const comments = ref<Comment[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const selectedIds = ref<number[]>([])
interface CommentStats {
  total: number
  pending: number
  approved: number
  spam: number
  today: number
}

const stats = ref<CommentStats | null>(null)
const replyVisible = ref(false)
const replyTarget = ref<Comment | null>(null)
const replyContent = ref('')

const filters = reactive({ status: '', search: '' })

async function fetchComments() {
  loading.value = true
  try {
    const res = await commentApi.list({ page: page.value, page_size: pageSize.value, ...filters })
    comments.value = res.items
    total.value = res.total
  } catch { comments.value = [] }
  finally { loading.value = false }
}

async function fetchStats() {
  try { stats.value = (await commentApi.stats()).data } catch {}
}

async function approve(id: number) {
  await commentApi.approve(id); ElMessage.success('已批准'); fetchComments(); fetchStats()
}
async function markSpam(id: number) {
  await commentApi.spam(id); ElMessage.success('已标记垃圾'); fetchComments(); fetchStats()
}
async function deleteComment(id: number) {
  await commentApi.bulk({ comment_ids: [id], action: 'delete' }); ElMessage.success('已删除'); fetchComments(); fetchStats()
}
async function bulk(action: string) {
  await commentApi.bulk({ comment_ids: selectedIds.value, action })
  ElMessage.success('操作成功'); fetchComments(); fetchStats()
}

function replyTo(comment: Comment) {
  replyTarget.value = comment
  replyContent.value = ''
  replyVisible.value = true
}

async function submitReply() {
  if (!replyContent.value.trim() || !replyTarget.value) return
  await commentApi.create({
    article_id: replyTarget.value.article_id,
    parent_id: replyTarget.value.id,
    content: replyContent.value,
  })
  ElMessage.success('回复已提交')
  replyVisible.value = false
  fetchComments()
}

function statusType(s: string) {
  return s === 'approved' ? 'success' : s === 'pending' ? 'warning' : 'danger'
}


onMounted(() => { fetchComments(); fetchStats() })
</script>

<style lang="scss" scoped>
.comment-page {
  .page-header {
    display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;
    h2 { margin: 0; }
    .comment-stats { display: flex; gap: 8px; }
  }
  .bulk-bar {
    display: flex; align-items: center; gap: 8px;
    padding: 12px; margin-bottom: 12px;
    background: #ecf5ff; border-radius: 4px; font-size: 13px;
  }
  .comment-cell {
    .comment-author { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
    .comment-email { font-size: 12px; color: #909399; margin-left: 8px; }
    .comment-text { margin: 4px 0; font-size: 14px; color: #303133; }
    .comment-meta {
      font-size: 12px; color: #909399;
      a { color: #409eff; text-decoration: none; }
    }
  }
  .reply-context {
    background: #f5f7fa; padding: 12px; border-radius: 4px;
    margin-bottom: 12px; font-size: 13px;
  }
  .pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 16px; }
}
</style>
