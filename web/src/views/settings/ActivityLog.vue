<template>
  <div class="activity-page">
    <h2>操作日志</h2>
    <el-card shadow="never">
      <el-form
        :inline="true"
        class="filter-form"
      >
        <el-form-item>
          <el-select
            v-model="filters.entity"
            placeholder="对象"
            clearable
            @change="fetchLogs"
          >
            <el-option
              label="文章"
              value="article"
            /><el-option
              label="用户"
              value="user"
            />
            <el-option
              label="评论"
              value="comment"
            /><el-option
              label="设置"
              value="settings"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-select
            v-model="filters.action"
            placeholder="操作"
            clearable
            @change="fetchLogs"
          >
            <el-option
              label="创建"
              value="create"
            /><el-option
              label="更新"
              value="update"
            />
            <el-option
              label="删除"
              value="delete"
            /><el-option
              label="登录"
              value="login"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            @click="fetchLogs"
          >
            搜索
          </el-button>
        </el-form-item>
      </el-form>

      <el-table
        v-loading="loading"
        :data="logs"
      >
        <el-table-column
          label="时间"
          width="170"
        >
          <template #default="{ row }">
            {{ formatDate(row.created_at, 'YYYY-MM-DD HH:mm:ss') }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="100"
        >
          <template #default="{ row }">
            <el-tag
              :type="actionType(row.action)"
              size="small"
            >
              {{ row.action }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="对象"
          prop="entity"
          width="100"
        />
        <el-table-column
          label="对象 ID"
          prop="entity_id"
          width="80"
        />
        <el-table-column
          label="IP"
          prop="ip"
          width="130"
        />
        <el-table-column
          label="User Agent"
          prop="user_agent"
          min-width="200"
          show-overflow-tooltip
        />
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="page"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="fetchLogs"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { systemApi } from '@/api'
import { formatDate } from '@/utils'

interface ActivityLogEntry {
  id: number
  created_at: string
  action: string
  entity: string
  entity_id: number
  ip: string
  user_agent: string
}

interface ActivityListResponse {
  items: ActivityLogEntry[]
  total: number
}

const logs = ref<ActivityLogEntry[]>([])
const loading = ref(false)
const page = ref(1)
const total = ref(0)
const filters = reactive({ entity: '', action: '' })

async function fetchLogs() {
  loading.value = true
  try {
    const res = await systemApi.activity({ page: page.value, page_size: 50, ...filters }) as ActivityListResponse
    logs.value = res.items; total.value = res.total
  } catch { logs.value = [] }
  finally { loading.value = false }
}

function actionType(a: string) {
  return a === 'create' ? 'success' : a === 'delete' ? 'danger' : a === 'login' ? 'info' : 'warning'
}


onMounted(fetchLogs)
</script>

<style lang="scss" scoped>
.activity-page { h2 { margin-bottom: 16px; } .pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 16px; } }
</style>
