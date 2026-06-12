<template>
  <div class="user-page">
    <div class="page-header">
      <h2>用户管理</h2>
      <el-button type="primary" @click="openDialog()"><el-icon><Plus /></el-icon> 新建用户</el-button>
    </div>

    <el-card shadow="never">
      <el-form :inline="true" class="filter-form">
        <el-form-item>
          <el-input v-model="filters.search" placeholder="搜索用户..." clearable @clear="fetchUsers" />
        </el-form-item>
        <el-form-item>
          <el-select v-model="filters.role" placeholder="角色" clearable @change="fetchUsers">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.slug" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-select v-model="filters.status" placeholder="状态" clearable @change="fetchUsers">
            <el-option label="正常" value="active" />
            <el-option label="禁用" value="inactive" />
            <el-option label="封禁" value="banned" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchUsers">搜索</el-button>
        </el-form-item>
      </el-form>

      <el-table :data="users" v-loading="loading">
        <el-table-column label="用户" min-width="200">
          <template #default="{ row }">
            <div class="user-cell">
              <el-avatar :src="row.avatar" :size="36">{{ (row.display_name || row.username)[0] }}</el-avatar>
              <div>
                <router-link :to="`/admin/users/${row.id}`" class="user-name">{{ row.display_name }}</router-link>
                <div class="user-meta">@{{ row.username }} · {{ row.email }}</div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="角色" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ row.role?.name }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
              {{ { active: '正常', inactive: '禁用', banned: '封禁' }[row.status as string] }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="登录次数" prop="login_count" width="100" align="center" />
        <el-table-column label="注册时间" width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button text size="small" @click="openDialog(row as User)">编辑</el-button>
            <el-button text size="small" @click="resetPassword(row as User)">重置密码</el-button>
            <el-popconfirm title="确认删除？" @confirm="deleteUser(row.id)">
              <template #reference>
                <el-button text size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination v-model:current-page="page" :total="total" layout="total, prev, pager, next"
          @current-change="fetchUsers" />
      </div>
    </el-card>

    <!-- Dialog -->
    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑用户' : '新建用户'" width="500px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="用户名" required>
          <el-input v-model="form.username" :disabled="!!editingId" />
        </el-form-item>
        <el-form-item label="邮箱" required>
          <el-input v-model="form.email" />
        </el-form-item>
        <el-form-item v-if="!editingId" label="密码" required>
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="显示名">
          <el-input v-model="form.display_name" />
        </el-form-item>
        <el-form-item label="角色" required>
          <el-select v-model="form.role_id" style="width: 100%">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="正常" value="active" />
            <el-option label="禁用" value="inactive" />
            <el-option label="封禁" value="banned" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveUser">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { userApi, roleApi, type User, type Role } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'

const users = ref<User[]>([])
const roles = ref<Role[]>([])
const loading = ref(false)
const page = ref(1)
const total = ref(0)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)

const filters = reactive({ search: '', role: '', status: '' })
const form = reactive({
  username: '', email: '', password: '', display_name: '',
  role_id: 0, status: 'active',
})

async function fetchUsers() {
  loading.value = true
  try {
    const res = await userApi.list({ page: page.value, page_size: 20, ...filters })
    users.value = res.items as any
    total.value = res.total
  } catch { users.value = [] }
  finally { loading.value = false }
}

async function fetchRoles() {
  try { roles.value = (await roleApi.list()).data } catch {}
}

function openDialog(user?: User) {
  if (user) {
    editingId.value = user.id
    Object.assign(form, {
      username: user.username, email: user.email, display_name: user.display_name,
      role_id: user.role?.id, status: user.status,
    })
  } else {
    editingId.value = null
    Object.assign(form, { username: '', email: '', password: '', display_name: '', role_id: 4, status: 'active' })
  }
  dialogVisible.value = true
}

async function saveUser() {
  try {
    if (editingId.value) {
      await userApi.update(editingId.value, form)
      ElMessage.success('用户已更新')
    } else {
      await userApi.create(form as any)
      ElMessage.success('用户已创建')
    }
    dialogVisible.value = false
    fetchUsers()
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '保存失败')
  }
}

async function deleteUser(id: number) {
  await userApi.delete(id)
  ElMessage.success('用户已删除')
  fetchUsers()
}

async function resetPassword(user: User) {
  const { value } = await ElMessageBox.prompt('输入新密码', '重置密码', {
    inputValidator: (v) => v.length >= 8 ? true : '密码至少8个字符',
  })
  await userApi.resetPassword(user.id, value)
  ElMessage.success('密码已重置')
}

function formatDate(s: string) { return dayjs(s).format('YYYY-MM-DD HH:mm') }

onMounted(() => { fetchUsers(); fetchRoles() })
</script>

<style lang="scss" scoped>
.user-page {
  .page-header {
    display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;
    h2 { margin: 0; }
  }
  .user-cell {
    display: flex; align-items: center; gap: 10px;
    .user-name { font-weight: 500; color: #303133; text-decoration: none; &:hover { color: #409eff; } }
    .user-meta { font-size: 12px; color: #909399; }
  }
  .pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 16px; }
}
</style>
