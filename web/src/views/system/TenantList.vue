<template>
  <div class="tenant-page">
    <div class="page-header">
      <h2>租户管理</h2>
      <el-button
        type="primary"
        @click="openCreate"
      >
        <el-icon><Plus /></el-icon> 新建租户
      </el-button>
    </div>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      class="usage-tip"
    >
      租户是内容与成员的隔离边界。停用租户后其成员无法登录使用；最后一名管理员不可移除，避免租户失去管理入口。
    </el-alert>

    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="tenants"
      >
        <el-table-column
          label="名称"
          prop="name"
          min-width="160"
        />
        <el-table-column
          label="Slug"
          prop="slug"
          min-width="140"
        />
        <el-table-column
          label="状态"
          width="110"
        >
          <template #default="{ row }">
            <el-tag
              :type="row.status === 'active' ? 'success' : 'danger'"
              size="small"
            >
              {{ row.status === 'active' ? '启用' : '已停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="成员上限"
          prop="max_users"
          width="110"
        >
          <template #default="{ row }">
            {{ row.max_users > 0 ? row.max_users : '不限' }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="260"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              size="small"
              @click="openMembers(row as Tenant)"
            >
              成员
            </el-button>
            <el-button
              size="small"
              @click="openEdit(row as Tenant)"
            >
              编辑
            </el-button>
            <el-button
              size="small"
              :type="row.status === 'active' ? 'danger' : 'success'"
              :disabled="row.status === 'active' && row.id === 1"
              @click="toggleStatus(row as Tenant)"
            >
              {{ row.status === 'active' ? '停用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create / edit tenant -->
    <el-dialog
      v-model="editVisible"
      :title="editing ? '编辑租户' : '新建租户'"
      width="440px"
    >
      <el-form
        :model="form"
        label-width="90px"
      >
        <el-form-item label="名称">
          <el-input
            v-model="form.name"
            maxlength="128"
          />
        </el-form-item>
        <el-form-item
          v-if="!editing"
          label="Slug"
        >
          <el-input
            v-model="form.slug"
            placeholder="小写字母、数字与连字符"
          />
        </el-form-item>
        <el-form-item label="成员上限">
          <el-input-number
            v-model="form.max_users"
            :min="0"
          />
          <span class="form-hint">0 表示不限</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="saveTenant"
        >
          保存
        </el-button>
      </template>
    </el-dialog>

    <!-- Members management -->
    <el-dialog
      v-model="membersVisible"
      :title="`成员管理 · ${activeTenant?.name || ''}`"
      width="720px"
    >
      <div class="member-add">
        <el-select
          v-model="newMemberUserID"
          filterable
          placeholder="选择用户"
          class="member-user"
        >
          <el-option
            v-for="user in candidateUsers"
            :key="user.id"
            :label="`${user.display_name || user.username} (${user.username})`"
            :value="user.id"
          />
        </el-select>
        <el-select
          v-model="newMemberRole"
          class="member-role"
        >
          <el-option
            label="管理员"
            value="admin"
          />
          <el-option
            label="编辑"
            value="editor"
          />
          <el-option
            label="成员"
            value="member"
          />
        </el-select>
        <el-button
          type="primary"
          :disabled="!newMemberUserID"
          :loading="addingMember"
          @click="addMember"
        >
          添加
        </el-button>
      </div>

      <el-table
        v-loading="membersLoading"
        :data="members"
        size="small"
      >
        <el-table-column
          label="用户"
          min-width="140"
        >
          <template #default="{ row }">
            {{ row.display_name || row.username }}
          </template>
        </el-table-column>
        <el-table-column
          label="邮箱"
          prop="email"
          min-width="180"
          show-overflow-tooltip
        />
        <el-table-column
          label="角色"
          width="160"
        >
          <template #default="{ row }">
            <el-select
              :model-value="row.role_slug"
              size="small"
              @change="(role: TenantRole) => changeRole(row as TenantMember, role)"
            >
              <el-option
                label="管理员"
                value="admin"
              />
              <el-option
                label="编辑"
                value="editor"
              />
              <el-option
                label="成员"
                value="member"
              />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column
          label="加入时间"
          width="110"
        >
          <template #default="{ row }">
            {{ formatDate(row.joined_at) }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="90"
        >
          <template #default="{ row }">
            <el-button
              size="small"
              type="danger"
              link
              @click="removeMember(row as TenantMember)"
            >
              移除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import {
  tenantsApi,
  type Tenant,
  type TenantMember,
  type TenantRole,
} from '@/api/domains/tenants'
import { userApi } from '@/api/domains/users'
import type { User } from '@/api/domains/auth'

const tenants = ref<Tenant[]>([])
const loading = ref(false)
const saving = ref(false)

const editVisible = ref(false)
const editing = ref<Tenant | null>(null)
const form = ref({ name: '', slug: '', max_users: 0 })

const membersVisible = ref(false)
const activeTenant = ref<Tenant | null>(null)
const members = ref<TenantMember[]>([])
const membersLoading = ref(false)
const allUsers = ref<User[]>([])
const newMemberUserID = ref<number | null>(null)
const newMemberRole = ref<TenantRole>('member')
const addingMember = ref(false)

const candidateUsers = ref<User[]>([])

async function loadTenants() {
  loading.value = true
  try {
    const res = await tenantsApi.list()
    tenants.value = res.data
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  form.value = { name: '', slug: '', max_users: 0 }
  editVisible.value = true
}

function openEdit(tenant: Tenant) {
  editing.value = tenant
  form.value = {
    name: tenant.name,
    slug: tenant.slug,
    max_users: tenant.max_users,
  }
  editVisible.value = true
}

async function saveTenant() {
  saving.value = true
  try {
    if (editing.value) {
      await tenantsApi.update(editing.value.id, {
        name: form.value.name,
        max_users: form.value.max_users,
      })
      ElMessage.success('租户已更新')
    } else {
      await tenantsApi.create({
        name: form.value.name,
        slug: form.value.slug,
        max_users: form.value.max_users,
      })
      ElMessage.success('租户已创建')
    }
    editVisible.value = false
    await loadTenants()
  } finally {
    saving.value = false
  }
}

async function toggleStatus(tenant: Tenant) {
  const next = tenant.status === 'active' ? 'suspended' : 'active'
  const action = next === 'active' ? '启用' : '停用'
  try {
    await ElMessageBox.confirm(
      `确定要${action}租户「${tenant.name}」吗？停用后其成员将无法登录。`,
      '确认',
      { type: 'warning' },
    )
  } catch {
    return
  }
  await tenantsApi.update(tenant.id, { status: next })
  ElMessage.success(`租户已${action}`)
  await loadTenants()
}

async function openMembers(tenant: Tenant) {
  activeTenant.value = tenant
  membersVisible.value = true
  newMemberUserID.value = null
  newMemberRole.value = 'member'
  await Promise.all([loadMembers(tenant.id), loadUsers()])
}

async function loadMembers(tenantID: number) {
  membersLoading.value = true
  try {
    const res = await tenantsApi.listMembers(tenantID)
    members.value = res.data
  } finally {
    membersLoading.value = false
  }
}

async function loadUsers() {
  const res = await userApi.list({ page: 1, page_size: 100 })
  allUsers.value = res.items
  refreshCandidates()
}

function refreshCandidates() {
  const memberIDs = new Set(members.value.map((m) => m.user_id))
  candidateUsers.value = allUsers.value.filter((u: User) => !memberIDs.has(u.id))
}

async function addMember() {
  if (!activeTenant.value || !newMemberUserID.value) {
    return
  }
  addingMember.value = true
  try {
    await tenantsApi.addMember(activeTenant.value.id, {
      user_id: newMemberUserID.value,
      role_slug: newMemberRole.value,
    })
    ElMessage.success('成员已添加')
    newMemberUserID.value = null
    await loadMembers(activeTenant.value.id)
    refreshCandidates()
  } finally {
    addingMember.value = false
  }
}

async function changeRole(member: TenantMember, role: TenantRole) {
  if (!activeTenant.value) {
    return
  }
  try {
    await tenantsApi.updateMemberRole(activeTenant.value.id, member.user_id, role)
    member.role_slug = role
    ElMessage.success('角色已更新')
  } catch {
    // Reload so a failed change (e.g. backend validation) resets the select.
    await loadMembers(activeTenant.value.id)
  }
}

async function removeMember(member: TenantMember) {
  if (!activeTenant.value) {
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定要将「${member.display_name || member.username}」移出该租户吗？`,
      '确认',
      { type: 'warning' },
    )
  } catch {
    return
  }
  await tenantsApi.removeMember(activeTenant.value.id, member.user_id)
  ElMessage.success('成员已移除')
  await loadMembers(activeTenant.value.id)
  refreshCandidates()
}

function formatDate(value?: string) {
  if (!value) {
    return ''
  }
  return value.slice(0, 10)
}

onMounted(() => {
  void loadTenants()
})
</script>

<style scoped>
.tenant-page {
  padding: 0;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.page-header h2 {
  margin: 0;
}

.usage-tip {
  margin-bottom: 16px;
}

.member-add {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.member-user {
  flex: 1;
}

.member-role {
  width: 120px;
}

.form-hint {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
