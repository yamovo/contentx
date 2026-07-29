<template>
  <div class="media-page">
    <div class="page-header">
      <h2>媒体库</h2>
      <div class="header-actions">
        <el-button @click="showStats = true">
          <el-icon><DataAnalysis /></el-icon> 统计
        </el-button>
        <el-upload
          action="/api/v1/media/upload"
          :headers="uploadHeaders"
          :on-success="onUploadSuccess"
          :show-file-list="false"
          multiple
        >
          <el-button type="primary">
            <el-icon><Upload /></el-icon> 上传文件
          </el-button>
        </el-upload>
      </div>
    </div>

    <!-- Filters -->
    <el-card
      shadow="never"
      class="filter-card"
    >
      <el-form :inline="true">
        <el-form-item>
          <el-select
            v-model="filters.type"
            placeholder="文件类型"
            clearable
            @change="applyFilters"
          >
            <el-option
              label="图片"
              value="image"
            />
            <el-option
              label="视频"
              value="video"
            />
            <el-option
              label="音频"
              value="audio"
            />
            <el-option
              label="文档"
              value="application"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-select
            v-model="filters.folder"
            placeholder="文件夹"
            clearable
            @change="applyFilters"
          >
            <el-option
              v-for="f in folders"
              :key="f"
              :label="f"
              :value="f"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="filters.search"
            placeholder="搜索文件..."
            clearable
            @clear="applyFilters"
            @keyup.enter="applyFilters"
          />
        </el-form-item>
        <el-form-item>
          <el-radio-group v-model="viewMode">
            <el-radio-button value="grid">
              网格
            </el-radio-button>
            <el-radio-button value="list">
              列表
            </el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Grid View -->
    <div
      v-if="viewMode === 'grid'"
      v-loading="loading"
      class="media-grid"
    >
      <div
        v-for="item in media"
        :key="item.id"
        class="media-card"
        :class="{ selected: selectedMedia?.id === item.id }"
        @click="selectMedia(item)"
      >
        <div class="media-preview">
          <img
            v-if="item.mime_type.startsWith('image/')"
            :src="item.url"
            :alt="item.alt"
            loading="lazy"
            decoding="async"
          >
          <div
            v-else
            class="file-icon"
          >
            <el-icon :size="48">
              <Document />
            </el-icon>
            <span>{{ item.filename.split('.').pop()?.toUpperCase() }}</span>
          </div>
        </div>
        <div class="media-info">
          <span
            class="media-name"
            :title="item.original_name"
          >{{ item.original_name }}</span>
          <span class="media-size">{{ formatSize(item.file_size) }}</span>
        </div>
      </div>
      <el-empty
        v-if="!media.length && !loading"
        description="暂无文件"
      />
    </div>

    <!-- List View -->
    <el-card
      v-else
      shadow="never"
    >
      <el-table
        v-loading="loading"
        :data="media"
        @row-click="selectMedia"
      >
        <el-table-column width="60">
          <template #default="{ row }">
            <el-image
              v-if="row.mime_type.startsWith('image/')"
              :src="row.url"
              style="width: 40px; height: 40px; border-radius: 4px"
              fit="cover"
            />
            <el-icon
              v-else
              :size="24"
            >
              <Document />
            </el-icon>
          </template>
        </el-table-column>
        <el-table-column
          label="文件名"
          prop="original_name"
          min-width="200"
        />
        <el-table-column
          label="类型"
          prop="mime_type"
          width="120"
        />
        <el-table-column
          label="大小"
          width="100"
        >
          <template #default="{ row }">
            {{ formatSize(row.file_size) }}
          </template>
        </el-table-column>
        <el-table-column
          label="上传时间"
          width="160"
        >
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="120"
        >
          <template #default="{ row }">
            <el-button
              text
              size="small"
              @click.stop="copyUrl(row as Media)"
            >
              复制URL
            </el-button>
            <el-popconfirm
              title="确认删除？"
              @confirm="deleteMedia(row.id)"
            >
              <template #reference>
                <el-button
                  text
                  size="small"
                  type="danger"
                  @click.stop
                >
                  删除
                </el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Pagination (was missing: page/total state existed but users could
         never navigate past page 1) -->
    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        background
        @current-change="refetch"
      />
    </div>

    <!-- Detail Sidebar -->
    <el-drawer
      v-model="detailVisible"
      title="文件详情"
      size="400px"
    >
      <template v-if="selectedMedia">
        <div class="detail-preview">
          <img
            v-if="selectedMedia.mime_type.startsWith('image/')"
            :src="selectedMedia.url"
          >
        </div>
        <el-form
          label-position="top"
          class="detail-form"
        >
          <el-form-item label="文件名">
            <el-input
              v-model="editForm.filename"
              disabled
            />
          </el-form-item>
          <el-form-item label="标题">
            <el-input v-model="editForm.title" />
          </el-form-item>
          <el-form-item label="替代文本">
            <el-input v-model="editForm.alt" />
          </el-form-item>
          <el-form-item label="说明">
            <el-input
              v-model="editForm.caption"
              type="textarea"
            />
          </el-form-item>
          <el-form-item label="URL">
            <el-input
              :model-value="selectedMedia.url"
              readonly
            >
              <template #append>
                <el-button @click="copyUrl(selectedMedia!)">
                  复制
                </el-button>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              @click="updateMedia"
            >
              保存修改
            </el-button>
            <el-button
              type="danger"
              @click="deleteMedia(selectedMedia!.id)"
            >
              删除
            </el-button>
          </el-form-item>
        </el-form>
      </template>
    </el-drawer>

    <!-- Stats Dialog -->
    <el-dialog
      v-model="showStats"
      title="媒体库统计"
      width="400px"
    >
      <div
        v-if="mediaStats"
        class="stats-content"
      >
        <el-statistic
          title="总文件数"
          :value="mediaStats.total_files"
        />
        <el-statistic
          title="总大小"
          :value="mediaStats.total_size"
          :formatter="formatSize"
        />
        <el-statistic
          title="图片"
          :value="mediaStats.images"
        />
        <el-statistic
          title="视频"
          :value="mediaStats.videos"
        />
        <el-statistic
          title="文档"
          :value="mediaStats.documents"
        />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { type Media } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { formatDate, formatSize } from '@/utils'
import { useMediaListQuery, useMediaFoldersQuery, useMediaStatsQuery } from '@/features/media/use-media-list-query'
import { useMediaMutations } from '@/features/media/use-media-mutations'
import { mediaQueryKeys } from '@/entities/media/api/query-keys'

const authStore = useAuthStore()
const queryClient = useQueryClient()
const viewMode = ref('grid')
const selectedMedia = ref<Media | null>(null)
const detailVisible = ref(false)
const showStats = ref(false)
const page = ref(1)
const pageSize = 24

const filters = reactive({ type: '', folder: '', search: '' })

const editForm = reactive({
  filename: '', title: '', alt: '', caption: '',
})

const uploadHeaders = computed(() => ({
  Authorization: `Bearer ${authStore.token}`,
}))

const queryParams = computed(() => ({
  page: page.value,
  page_size: pageSize,
  ...(filters.type ? { type: filters.type } : {}),
  ...(filters.folder ? { folder: filters.folder } : {}),
  ...(filters.search ? { search: filters.search } : {}),
}))

const { data: listData, isLoading: loading, refetch } = useMediaListQuery(queryParams)
const media = computed(() => listData.value?.items ?? [])
const total = computed(() => listData.value?.total ?? 0)

const { data: foldersData } = useMediaFoldersQuery()
const folders = computed(() => foldersData.value?.data ?? [])

const { data: statsData } = useMediaStatsQuery()
const mediaStats = computed(() => statsData.value?.data ?? null)

const { updateMedia: updateMediaMutation, deleteMedia: deleteMediaMutation } = useMediaMutations()

// Filter changes restart from page 1 so stale offsets never show empty pages.
function applyFilters() {
  page.value = 1
  refetch()
}

function selectMedia(item: Media) {
  selectedMedia.value = item
  Object.assign(editForm, {
    filename: item.filename, title: item.title, alt: item.alt, caption: item.caption,
  })
  detailVisible.value = true
}

async function updateMedia() {
  if (!selectedMedia.value) return
  try {
    await updateMediaMutation.mutateAsync({ id: selectedMedia.value.id, ...editForm })
    ElMessage.success('已更新')
  } catch { /* mutation handles invalidation */ }
}

async function deleteMedia(id: number) {
  try {
    await deleteMediaMutation.mutateAsync(id)
    ElMessage.success('已删除')
    detailVisible.value = false
  } catch { /* mutation handles invalidation */ }
}

function copyUrl(item: Media) {
  navigator.clipboard.writeText(window.location.origin + item.url)
  ElMessage.success('URL 已复制')
}

function onUploadSuccess() {
  ElMessage.success('上传成功')
  queryClient.invalidateQueries({ queryKey: mediaQueryKeys.all })
}
</script>

<style lang="scss" scoped>
.media-page {
  .page-header {
    display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;
    h2 { margin: 0; }
    .header-actions { display: flex; gap: 8px; }
  }
  .filter-card { margin-bottom: 16px; }
  .pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 16px; }
  .media-grid {
    display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px;
  }
  .media-card {
    border: 2px solid transparent; border-radius: 8px; overflow: hidden;
    cursor: pointer; transition: all 0.2s;
    background: #fff; box-shadow: 0 1px 3px rgba(0,0,0,0.08);
    &:hover { border-color: #c0c4cc; }
    &.selected { border-color: #409eff; }
    .media-preview {
      height: 140px; display: flex; align-items: center; justify-content: center;
      background: #f5f7fa; overflow: hidden;
      img { width: 100%; height: 100%; object-fit: cover; }
      .file-icon {
        display: flex; flex-direction: column; align-items: center; gap: 4px;
        color: #909399;
        span { font-size: 12px; font-weight: 600; }
      }
    }
    .media-info {
      padding: 8px 10px;
      .media-name {
        display: block; font-size: 12px; white-space: nowrap;
        overflow: hidden; text-overflow: ellipsis;
      }
      .media-size { font-size: 11px; color: #c0c4cc; }
    }
  }
  .detail-preview {
    margin-bottom: 16px;
    img { width: 100%; border-radius: 6px; }
  }
  .stats-content {
    display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; text-align: center;
  }
}
</style>
