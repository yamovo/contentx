<template>
  <div class="backup-page">
    <div class="page-header">
      <h2>备份与恢复</h2>
      <div class="header-actions">
        <el-select
          v-model="backupType"
          style="width: 140px"
        >
          <el-option
            value="all"
            label="数据库 + 媒体"
          />
          <el-option
            value="db"
            label="仅数据库"
          />
          <el-option
            value="media"
            label="仅媒体文件"
          />
        </el-select>
        <el-button
          type="primary"
          :loading="creating"
          @click="handleCreateBackup"
        >
          <el-icon><Plus /></el-icon> 立即备份
        </el-button>
      </div>
    </div>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      class="usage-tip"
    >
      系统每日 03:00 自动备份。恢复数据库会覆盖当前数据；SQLite 恢复后需要重启服务生效。
    </el-alert>

    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="backups"
      >
        <el-table-column
          label="文件名"
          prop="name"
          min-width="280"
        />
        <el-table-column
          label="类型"
          width="100"
        >
          <template #default="{ row }">
            <el-tag
              size="small"
              :type="row.name.startsWith('media-') ? 'warning' : 'success'"
            >
              {{ row.name.startsWith('media-') ? '媒体' : '数据库' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="大小"
          width="120"
        >
          <template #default="{ row }">
            {{ formatSize(row.size) }}
          </template>
        </el-table-column>
        <el-table-column
          label="创建时间"
          width="180"
        >
          <template #default="{ row }">
            {{ new Date(row.created_at).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="220"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              text
              size="small"
              @click="handleDownloadBackup(row.name)"
            >
              下载
            </el-button>
            <el-button
              text
              size="small"
              type="warning"
              @click="confirmRestore(row.name)"
            >
              恢复
            </el-button>
            <el-popconfirm
              title="确认删除该备份文件？"
              @confirm="removeBackup(row.name)"
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { type BackupInfo } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useBackupListQuery } from '@/features/backups/use-backup-list-query'
import { useBackupMutations } from '@/features/backups/use-backup-mutations'

const { data: queryData, isLoading: loading } = useBackupListQuery()
const backups = computed<BackupInfo[]>(() => queryData.value?.data ?? [])

const { createBackup: createBackupMutation, restoreBackup, downloadBackup, deleteBackup } = useBackupMutations()

const creating = computed(() => createBackupMutation.isPending.value)
const backupType = ref<'db' | 'media' | 'all'>('all')

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

async function handleCreateBackup() {
  await createBackupMutation.mutateAsync(backupType.value)
  ElMessage.success('备份已创建')
}

async function handleDownloadBackup(name: string) {
  await downloadBackup.mutateAsync(name)
}

async function confirmRestore(name: string) {
  const isMedia = name.startsWith('media-')
  try {
    await ElMessageBox.confirm(
      isMedia
        ? `将从「${name}」恢复媒体文件，同名文件会被覆盖。继续？`
        : `将从「${name}」恢复数据库，当前数据会被覆盖且不可撤销。继续？`,
      '恢复确认',
      { type: 'warning', confirmButtonText: '恢复', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  const res = await restoreBackup.mutateAsync(name)
  const warning = res.data?.warning as string | undefined
  if (warning) {
    ElMessage.warning(warning)
  } else {
    ElMessage.success('恢复完成')
  }
}

async function removeBackup(name: string) {
  await deleteBackup.mutateAsync(name)
  ElMessage.success('备份已删除')
}
</script>

<style lang="scss" scoped>
.backup-page {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    h2 { margin: 0; }
    .header-actions {
      display: flex;
      gap: 12px;
    }
  }
  .usage-tip { margin-bottom: 16px; }
}
</style>
