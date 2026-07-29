<template>
  <div class="webhook-page">
    <div class="page-header">
      <h2>Webhook</h2>
      <el-button
        type="primary"
        @click="openDialog"
      >
        <el-icon><Plus /></el-icon> 添加 Webhook
      </el-button>
    </div>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      class="usage-tip"
    >
      内容事件发生时向目标 URL 推送 JSON 通知。配置 Secret 后请求会携带 HMAC-SHA256 签名头，失败自动重试。
    </el-alert>

    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="webhooks"
      >
        <el-table-column
          label="名称"
          prop="name"
          min-width="140"
        />
        <el-table-column
          label="目标 URL"
          prop="url"
          min-width="240"
          show-overflow-tooltip
        />
        <el-table-column
          label="订阅事件"
          min-width="220"
        >
          <template #default="{ row }">
            <el-tag
              v-for="e in row.events || []"
              :key="e"
              size="small"
              class="event-tag"
            >
              {{ e }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="80"
        >
          <template #default="{ row }">
            <el-tag
              :type="row.is_active ? 'success' : 'info'"
              size="small"
            >
              {{ row.is_active ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="160"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              text
              size="small"
              @click="showLogs(row as Webhook)"
            >
              日志
            </el-button>
            <el-popconfirm
              title="确认删除该 Webhook？"
              @confirm="removeWebhook(row.id)"
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
    </el-card>

    <!-- Create dialog -->
    <el-dialog
      v-model="dialogVisible"
      title="添加 Webhook"
      width="560px"
    >
      <el-form
        :model="form"
        label-width="90px"
      >
        <el-form-item
          label="名称"
          required
        >
          <el-input
            v-model="form.name"
            maxlength="128"
            placeholder="例如：站点重建通知"
          />
        </el-form-item>
        <el-form-item
          label="目标 URL"
          required
        >
          <el-input
            v-model="form.url"
            placeholder="https://example.com/hooks/contentx"
          />
        </el-form-item>
        <el-form-item
          label="订阅事件"
          required
        >
          <el-select
            v-model="form.events"
            multiple
            style="width: 100%"
            placeholder="选择要订阅的事件"
          >
            <el-option
              v-for="e in eventOptions"
              :key="e"
              :value="e"
              :label="e"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Secret">
          <el-input
            v-model="form.secret"
            placeholder="可选；用于 HMAC-SHA256 签名校验"
            show-password
          />
        </el-form-item>
        <el-form-item label="自定义头">
          <el-input
            v-model="headersText"
            type="textarea"
            :rows="2"
            placeholder="每行一个，格式 Key: Value（可选）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="creating"
          @click="handleCreateWebhook"
        >
          添加
        </el-button>
      </template>
    </el-dialog>

    <!-- Delivery logs drawer -->
    <el-drawer
      v-model="logsVisible"
      :title="`投递日志 — ${currentWebhook?.name || ''}`"
      size="60%"
    >
      <el-table
        v-loading="logsLoading"
        :data="logs"
      >
        <el-table-column
          label="事件"
          prop="event"
          width="140"
        />
        <el-table-column
          label="结果"
          width="90"
        >
          <template #default="{ row }">
            <el-tag
              :type="row.success ? 'success' : 'danger'"
              size="small"
            >
              {{ row.success ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="HTTP"
          prop="response"
          width="80"
        />
        <el-table-column
          label="耗时"
          width="90"
        >
          <template #default="{ row }">
            {{ row.duration }}ms
          </template>
        </el-table-column>
        <el-table-column
          label="重试"
          prop="retries"
          width="70"
        />
        <el-table-column
          label="错误"
          prop="error"
          min-width="160"
          show-overflow-tooltip
        />
        <el-table-column
          label="时间"
          width="170"
        >
          <template #default="{ row }">
            {{ new Date(row.created_at).toLocaleString() }}
          </template>
        </el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { type Webhook, type WebhookLog } from '@/api'
import { ElMessage } from 'element-plus'
import { useWebhookListQuery, useWebhookLogsQuery } from '@/features/webhooks/use-webhook-list-query'
import { useWebhookMutations } from '@/features/webhooks/use-webhook-mutations'

// Mirrors backend models.WebhookEvent* constants.
const eventOptions = [
  'entry.create', 'entry.update', 'entry.delete',
  'entry.publish', 'entry.unpublish', 'entry.schedule',
  'media.create', 'media.delete',
  'comment.create', 'user.create',
]

const { data: queryData, isLoading: loading } = useWebhookListQuery()
const webhooks = computed<Webhook[]>(() => queryData.value?.data ?? [])

const { createWebhook: createWebhookMutation, deleteWebhook } = useWebhookMutations()

const dialogVisible = ref(false)
const creating = computed(() => createWebhookMutation.isPending.value)
const headersText = ref('')
const form = reactive({ name: '', url: '', events: [] as string[], secret: '' })

const logsVisible = ref(false)
const currentWebhook = ref<Webhook | null>(null)
const currentWebhookId = computed(() => currentWebhook.value?.id ?? 0)

const { data: logsData, isLoading: logsLoading, refetch: refetchLogs } = useWebhookLogsQuery(currentWebhookId.value)
const logs = computed<WebhookLog[]>(() => logsData.value?.data ?? [])

function openDialog() {
  Object.assign(form, { name: '', url: '', events: [], secret: '' })
  headersText.value = ''
  dialogVisible.value = true
}

async function handleCreateWebhook() {
  if (!form.name.trim() || !form.url.trim() || !form.events.length) {
    ElMessage.warning('请填写名称、URL 并至少选择一个事件')
    return
  }
  const headers = headersText.value
    .split('\n')
    .map(l => l.trim())
    .filter(Boolean)
  await createWebhookMutation.mutateAsync({
    name: form.name.trim(),
    url: form.url.trim(),
    events: form.events,
    headers,
    secret: form.secret || undefined,
  })
  ElMessage.success('Webhook 已添加')
  dialogVisible.value = false
}

async function removeWebhook(id: number) {
  await deleteWebhook.mutateAsync(id)
  ElMessage.success('Webhook 已删除')
}

function showLogs(wh: Webhook) {
  currentWebhook.value = wh
  logsVisible.value = true
  refetchLogs()
}
</script>

<style lang="scss" scoped>
.webhook-page {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    h2 { margin: 0; }
  }
  .usage-tip { margin-bottom: 16px; }
  .event-tag { margin: 2px 4px 2px 0; }
}
</style>
