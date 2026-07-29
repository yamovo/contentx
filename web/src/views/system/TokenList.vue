<template>
  <div class="token-page">
    <div class="page-header">
      <h2>API 令牌</h2>
      <el-button
        type="primary"
        @click="openDialog"
      >
        <el-icon><Plus /></el-icon> 创建令牌
      </el-button>
    </div>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      class="usage-tip"
    >
      API 令牌用于无头客户端和 MCP HTTP 接入，请求时携带 <code>Authorization: Bearer &lt;token&gt;</code>。令牌明文仅在创建时展示一次。
    </el-alert>

    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="tokens"
      >
        <el-table-column
          label="名称"
          prop="name"
          min-width="160"
        />
        <el-table-column
          label="权限"
          min-width="200"
        >
          <template #default="{ row }">
            <template v-if="row.permissions && row.permissions.length">
              <el-tag
                v-for="p in row.permissions"
                :key="p"
                size="small"
                class="perm-tag"
                :type="p === '*' ? 'danger' : 'info'"
              >
                {{ p === '*' ? '全部权限' : p }}
              </el-tag>
            </template>
            <span
              v-else
              class="text-muted"
            >完全访问</span>
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="80"
        >
          <template #default="{ row }">
            <el-tag
              :type="tokenStatus(row as APIToken).type"
              size="small"
            >
              {{ tokenStatus(row as APIToken).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="使用次数"
          prop="use_count"
          width="100"
        />
        <el-table-column
          label="最近使用"
          width="170"
        >
          <template #default="{ row }">
            {{ row.last_used_at ? formatTime(row.last_used_at) : '从未使用' }}
          </template>
        </el-table-column>
        <el-table-column
          label="过期时间"
          width="170"
        >
          <template #default="{ row }">
            {{ row.expires_at ? formatTime(row.expires_at) : '永不过期' }}
          </template>
        </el-table-column>
        <el-table-column
          label="创建时间"
          width="170"
        >
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="100"
          fixed="right"
        >
          <template #default="{ row }">
            <el-popconfirm
              title="删除后使用该令牌的客户端将立即失效，确认删除？"
              width="260"
              @confirm="removeToken(row.id)"
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
      title="创建 API 令牌"
      width="520px"
    >
      <el-form
        :model="form"
        label-width="80px"
      >
        <el-form-item
          label="名称"
          required
        >
          <el-input
            v-model="form.name"
            maxlength="128"
            placeholder="例如：MCP 客户端"
          />
        </el-form-item>
        <el-form-item label="权限">
          <el-checkbox
            v-model="form.fullAccess"
            class="full-access"
          >
            全部权限（仅限受信任的服务）
          </el-checkbox>
          <el-select
            v-if="!form.fullAccess"
            v-model="form.permissions"
            multiple
            filterable
            placeholder="选择最小必要权限"
            style="width: 100%"
          >
            <el-option
              v-for="p in permissionOptions"
              :key="p.slug"
              :value="p.slug"
              :label="`${p.slug} — ${p.description}`"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="过期时间">
          <el-date-picker
            v-model="form.expiresAt"
            type="datetime"
            placeholder="留空 = 永不过期"
            style="width: 100%"
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
          @click="handleCreateToken"
        >
          创建
        </el-button>
      </template>
    </el-dialog>

    <!-- One-time token reveal -->
    <el-dialog
      v-model="revealVisible"
      title="令牌已创建"
      width="560px"
      :close-on-click-modal="false"
    >
      <el-alert
        type="warning"
        :closable="false"
        show-icon
      >
        令牌明文仅显示这一次，请立即复制保存。关闭后将无法再次查看。
      </el-alert>
      <div class="token-reveal">
        <code>{{ createdToken }}</code>
        <el-button
          size="small"
          @click="copyToken"
        >
          <el-icon><CopyDocument /></el-icon> 复制
        </el-button>
      </div>
      <template #footer>
        <el-button
          type="primary"
          @click="revealVisible = false"
        >
          我已保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { roleApi, type APIToken, type Permission } from '@/api'
import { ElMessage } from 'element-plus'
import { isPermissionSlug } from '@/shared/auth/permissions'
import { useTokenListQuery } from '@/features/tokens/use-token-list-query'
import { useTokenMutations } from '@/features/tokens/use-token-mutations'

const { data: queryData, isLoading: loading } = useTokenListQuery()
const tokens = computed<APIToken[]>(() => queryData.value?.data ?? [])

const { createToken: createTokenMutation, revokeToken } = useTokenMutations()

const dialogVisible = ref(false)
const creating = computed(() => createTokenMutation.isPending.value)
const revealVisible = ref(false)
const createdToken = ref('')
const permissionOptions = ref<Permission[]>([])
const form = reactive<{ name: string; permissions: string[]; fullAccess: boolean; expiresAt: Date | null }>({
  name: '',
  permissions: [],
  fullAccess: false,
  expiresAt: null,
})

function formatTime(t: string) {
  return new Date(t).toLocaleString()
}

function tokenStatus(row: APIToken): { label: string; type: 'success' | 'warning' | 'info' } {
  if (!row.is_active) return { label: '停用', type: 'info' }
  if (row.expires_at && new Date(row.expires_at) < new Date()) return { label: '已过期', type: 'warning' }
  return { label: '有效', type: 'success' }
}

async function fetchPermissionOptions() {
  try {
    permissionOptions.value = (await roleApi.permissions()).data
      .filter(permission => isPermissionSlug(permission.slug))
  } catch {
    // Options are a convenience; the input still allows free-form slugs.
  }
}

function openDialog() {
  Object.assign(form, { name: '', permissions: [], fullAccess: false, expiresAt: null })
  dialogVisible.value = true
}

async function handleCreateToken() {
  if (!form.name.trim()) {
    ElMessage.warning('请填写令牌名称')
    return
  }
  if (!form.fullAccess && !form.permissions.length) {
    ElMessage.warning('请选择至少一个权限，或明确启用全部权限')
    return
  }
  try {
    const res = await createTokenMutation.mutateAsync({
      name: form.name.trim(),
      permissions: form.fullAccess ? ['*'] : form.permissions,
      expires_at: form.expiresAt ? form.expiresAt.toISOString() : '',
    })
    createdToken.value = res.data.token
    dialogVisible.value = false
    revealVisible.value = true
  } finally {
    // Mutation pending state handled by computed.
  }
}

async function copyToken() {
  await navigator.clipboard.writeText(createdToken.value)
  ElMessage.success('已复制到剪贴板')
}

async function removeToken(id: number) {
  await revokeToken.mutateAsync(id)
  ElMessage.success('令牌已删除')
}

onMounted(fetchPermissionOptions)
</script>

<style lang="scss" scoped>
.token-page {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    h2 { margin: 0; }
  }
  .usage-tip { margin-bottom: 16px; }
  .perm-tag { margin-right: 4px; }
  .text-muted { color: var(--el-text-color-secondary); }
  .full-access { display: flex; margin-bottom: 10px; }
  .token-reveal {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 16px;
    padding: 12px;
    background: var(--el-fill-color-light);
    border-radius: 4px;
    code {
      flex: 1;
      word-break: break-all;
      font-size: 13px;
    }
  }
}
</style>
