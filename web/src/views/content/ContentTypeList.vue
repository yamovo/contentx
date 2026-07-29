<template>
  <div class="ct-page">
    <div class="page-header">
      <h2>内容类型</h2>
      <el-button
        type="primary"
        @click="openDialog"
      >
        <el-icon><Plus /></el-icon> 新建类型
      </el-button>
    </div>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      class="usage-tip"
    >
      自定义内容类型（如产品、活动）由字段定义组成，条目通过 REST / GraphQL / MCP 对外提供。类型创建后字段不可修改，删除类型会同时删除全部条目。
    </el-alert>

    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="types"
      >
        <el-table-column
          label="名称"
          min-width="160"
        >
          <template #default="{ row }">
            <el-link
              type="primary"
              @click="openEntries(row as ContentType)"
            >
              {{ row.name }}
            </el-link>
          </template>
        </el-table-column>
        <el-table-column
          label="UID"
          prop="uid"
          width="160"
        />
        <el-table-column
          label="结构"
          width="100"
        >
          <template #default="{ row }">
            <el-tag
              size="small"
              :type="row.is_single ? 'warning' : 'info'"
            >
              {{ row.is_single ? '单例' : '集合' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="草稿/发布"
          width="100"
        >
          <template #default="{ row }">
            {{ row.draft_publish ? '启用' : '关闭' }}
          </template>
        </el-table-column>
        <el-table-column
          label="字段数"
          width="90"
        >
          <template #default="{ row }">
            {{ row.fields?.length || 0 }}
          </template>
        </el-table-column>
        <el-table-column
          label="条目数"
          width="90"
        >
          <template #default="{ row }">
            {{ row.entry_count ?? '-' }}
          </template>
        </el-table-column>
        <el-table-column
          label="描述"
          prop="description"
          min-width="180"
          show-overflow-tooltip
        />
        <el-table-column
          label="操作"
          width="160"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              text
              size="small"
              @click="openEntries(row as ContentType)"
            >
              条目
            </el-button>
            <el-popconfirm
              title="删除类型会同时删除全部条目，确认？"
              width="240"
              @confirm="removeType(row.uid)"
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

    <ContentTypeFormDrawer
      v-model="dialogVisible"
      :saving="creating"
      @submit="createType"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { contentApi, type ContentType } from '@/api'
import { ElMessage } from 'element-plus'
import ContentTypeFormDrawer from '@/features/content-type/ContentTypeFormDrawer.vue'
import type { ContentTypeCreatePayload } from '@/features/content-type/model'

const router = useRouter()

const types = ref<ContentType[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const creating = ref(false)

async function fetchTypes() {
  loading.value = true
  try {
    types.value = (await contentApi.listTypes()).data || []
  } finally {
    loading.value = false
  }
}

function openDialog() {
  dialogVisible.value = true
}

async function createType(payload: ContentTypeCreatePayload) {
  creating.value = true
  try {
    await contentApi.createType(payload)
    ElMessage.success('内容类型已创建')
    dialogVisible.value = false
    fetchTypes()
  } finally {
    creating.value = false
  }
}

async function removeType(uid: string) {
  await contentApi.deleteType(uid)
  ElMessage.success('内容类型已删除')
  fetchTypes()
}

function openEntries(ct: ContentType) {
  router.push(`/admin/content/${ct.uid}`)
}

onMounted(fetchTypes)
</script>

<style lang="scss" scoped>
.ct-page {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    h2 { margin: 0; }
  }
  .usage-tip { margin-bottom: 16px; }
  .field-hint {
    margin-left: 8px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
  .field-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }
}
</style>
