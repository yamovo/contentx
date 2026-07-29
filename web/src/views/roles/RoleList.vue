<template>
  <div class="role-page">
    <div class="page-header">
      <div>
        <h2>角色权限</h2>
        <p>角色编辑器只展示当前 Canonical 权限，旧版 view/edit/manage 标识不会再分配。</p>
      </div>
      <PermissionGate :permission="PERMISSIONS.roles.create">
        <el-button
          type="primary"
          @click="openDialog()"
        >
          <el-icon><Plus /></el-icon> 新建角色
        </el-button>
      </PermissionGate>
    </div>

    <el-alert
      v-if="errorMessage"
      type="error"
      :closable="false"
      show-icon
      :title="errorMessage"
      class="error-alert"
    >
      <el-button
        text
        type="primary"
        @click="refetch"
      >
        重试
      </el-button>
    </el-alert>

    <el-row
      v-loading="loading"
      :gutter="16"
    >
      <el-col
        v-for="role in roles"
        :key="role.id"
        :xs="24"
        :lg="12"
      >
        <el-card
          shadow="hover"
          class="role-card"
        >
          <template #header>
            <div class="role-header">
              <div>
                <h3>
                  {{ role.name }} <el-tag
                    v-if="role.is_system"
                    size="small"
                    type="info"
                  >
                    系统
                  </el-tag>
                </h3>
                <p class="role-desc">
                  {{ role.description }}
                </p>
              </div>
              <div class="role-actions">
                <el-button
                  text
                  size="small"
                  :disabled="role.is_system || !authStore.hasPermission(PERMISSIONS.roles.update)"
                  @click="editRole(role)"
                >
                  编辑
                </el-button>
                <el-button
                  text
                  size="small"
                  type="danger"
                  :disabled="role.is_system || !authStore.hasPermission(PERMISSIONS.roles.delete)"
                  @click="deleteRole(role)"
                >
                  删除
                </el-button>
              </div>
            </div>
          </template>
          <div class="role-meta">
            <span>用户数: {{ role.user_count || 0 }}</span>
            <span>权限数: {{ role.permissions?.length || 0 }}</span>
          </div>
          <div class="perm-list">
            <el-tag
              v-for="perm in role.permissions?.slice(0, 8)"
              :key="perm.id"
              size="small"
              class="perm-tag"
            >
              {{ perm.name }}
            </el-tag>
            <el-tag
              v-if="(role.permissions?.length || 0) > 8"
              size="small"
              type="info"
            >
              +{{ (role.permissions?.length || 0) - 8 }}
            </el-tag>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Permission Reference -->
    <el-card
      shadow="never"
      class="section-card"
    >
      <template #header>
        <span>权限列表</span>
      </template>
      <el-collapse>
        <el-collapse-item
          v-for="(perms, module) in groupedPerms"
          :key="module"
          :title="String(module)"
        >
          <el-checkbox
            v-for="p in perms"
            :key="p.id"
            :label="p.name"
            disabled
            class="perm-checkbox"
          >
            {{ p.name }} <span class="perm-slug">{{ p.slug }}</span>
          </el-checkbox>
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <!-- Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑角色' : '新建角色'"
      width="600px"
    >
      <el-form
        :model="form"
        label-width="80px"
      >
        <el-form-item
          label="名称"
          required
        >
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item
          label="标识"
          required
        >
          <el-input v-model="form.slug" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" />
        </el-form-item>
        <el-form-item label="权限">
          <el-checkbox-group v-model="form.permission_ids">
            <div
              v-for="(perms, module) in groupedPerms"
              :key="module"
              class="perm-group"
            >
              <h4>{{ module }}</h4>
              <el-checkbox
                v-for="p in perms"
                :key="p.id"
                :label="p.id"
              >
                {{ p.name }}
              </el-checkbox>
            </div>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          @click="saveRole"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { type Role, type Permission } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getApiError } from '@/utils'
import PermissionGate from '@/shared/ui/PermissionGate.vue'
import { isPermissionSlug, PERMISSIONS } from '@/shared/auth/permissions'
import { useAuthStore } from '@/stores/auth'
import { useRoleListQuery, useRolePermissionsQuery } from '@/features/roles/use-role-list-query'
import { useRoleMutations } from '@/features/roles/use-role-mutations'

const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive({ name: '', slug: '', description: '', permission_ids: [] as number[] })
const authStore = useAuthStore()

const { data: rolesData, isLoading: loading, isError, error, refetch } = useRoleListQuery()
const roles = computed(() => rolesData.value?.data ?? [])
const errorMessage = computed(() => isError.value ? getApiError(error.value, '角色与权限加载失败') : '')

const { data: permsData } = useRolePermissionsQuery()
const groupedPerms = computed(() => {
  const allPerms = permsData.value?.data ?? []
  const canonical = allPerms.filter(permission => isPermissionSlug(permission.slug))
  return canonical.reduce<Record<string, Permission[]>>((grouped, permission) => {
    (grouped[permission.module] ||= []).push(permission)
    return grouped
  }, {})
})

const { createRole, updateRole, deleteRole: deleteRoleMutation } = useRoleMutations()

function openDialog() {
  editingId.value = null
  Object.assign(form, { name: '', slug: '', description: '', permission_ids: [] })
  dialogVisible.value = true
}

function editRole(role: Role) {
  editingId.value = role.id
  Object.assign(form, {
    name: role.name, slug: role.slug, description: role.description,
    permission_ids: role.permissions?.map(p => p.id) || [],
  })
  dialogVisible.value = true
}

async function saveRole() {
  try {
    if (editingId.value) {
      await updateRole.mutateAsync({ id: editingId.value, ...form })
      ElMessage.success('角色已更新')
    } else {
      await createRole.mutateAsync(form)
      ElMessage.success('角色已创建')
    }
    dialogVisible.value = false
  } catch (err) { ElMessage.error(getApiError(err, '保存失败')) }
}

async function deleteRole(role: Role) {
  try {
    await ElMessageBox.confirm(`确认删除角色 "${role.name}"？`, '确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await deleteRoleMutation.mutateAsync(role.id)
    ElMessage.success('角色已删除')
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(getApiError(error, '删除失败'))
  }
}
</script>

<style lang="scss" scoped>
.role-page {
  .page-header {
    display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; margin-bottom: 16px;
    h2 { margin: 0; }
    p { margin: 6px 0 0; color: var(--app-text-muted, #6b7280); font-size: 13px; }
  }
  .error-alert { margin-bottom: 16px; }
  .role-card { margin-bottom: 16px; }
  .role-header { display: flex; justify-content: space-between; align-items: flex-start;
    h3 { margin: 0 0 4px; } .role-desc { color: #909399; font-size: 13px; margin: 0; } }
  .role-meta { display: flex; gap: 16px; font-size: 13px; color: #606266; margin-bottom: 12px; }
  .perm-list { display: flex; flex-wrap: wrap; gap: 4px; }
  .perm-tag { margin: 0; }
  .section-card { margin-top: 16px; }
  .perm-group { margin-bottom: 12px; h4 { margin: 0 0 8px; color: #409eff; } }
  .perm-checkbox { display: block; margin: 4px 0; }
  .perm-slug { font-size: 11px; color: #c0c4cc; margin-left: 4px; }
}
</style>
