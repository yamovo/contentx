<template>
  <div class="theme-page">
    <div class="page-header">
      <div>
        <h2>主题管理</h2>
        <p>主题仅影响未来公共站点渲染；管理后台外观由界面主题设置控制。</p>
      </div>
      <el-button
        :loading="loading"
        @click="refetch"
      >
        刷新
      </el-button>
    </div>
    <el-alert
      v-if="errorMessage"
      type="error"
      show-icon
      :closable="false"
      :title="errorMessage"
      class="error-alert"
    />
    <el-row :gutter="16">
      <el-col
        v-for="t in themes"
        :key="t.id"
        :xs="24"
        :md="12"
        :xl="8"
      >
        <el-card
          shadow="hover"
          class="theme-card"
          :class="{ active: t.is_active }"
        >
          <div class="theme-preview">
            <img
              v-if="t.screenshot"
              :src="t.screenshot"
              :alt="`${t.name} 主题预览`"
              loading="lazy"
            >
            <div
              v-else
              class="placeholder"
            >
              <el-icon :size="48">
                <Brush />
              </el-icon>
            </div>
            <el-tag
              v-if="t.is_active"
              class="active-badge"
              type="success"
            >
              当前主题
            </el-tag>
          </div>
          <div class="theme-info">
            <h3>
              {{ t.name }} <el-tag size="small">
                v{{ t.version }}
              </el-tag>
            </h3>
            <p>{{ t.description }}</p>
            <el-button
              v-if="!t.is_active"
              type="primary"
              size="small"
              :loading="activatingId === t.id"
              @click="activateTheme(t)"
            >
              启用
            </el-button>
            <el-button
              text
              size="small"
              @click="openConfig(t)"
            >
              编辑配置
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
    <el-empty
      v-if="!themes.length"
      :description="loading ? '正在加载主题' : '暂无已安装的主题'"
    />
    <JsonConfigDrawer
      v-model="configVisible"
      :title="selectedTheme?.name || '主题'"
      :config="selectedTheme?.config"
      :saving="savingConfig"
      @save="saveConfig"
    />
  </div>
</template>
<script setup lang="ts">
import { ref, computed } from 'vue'
import { type Theme } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import JsonConfigDrawer from '@/features/config/JsonConfigDrawer.vue'
import { getApiError } from '@/utils'
import { useThemeListQuery } from '@/features/themes/use-theme-list-query'
import { useThemeMutations } from '@/features/themes/use-theme-mutations'

const { data: queryData, isLoading: loading, isError, refetch } = useThemeListQuery()
const themes = computed<Theme[]>(() => queryData.value?.data ?? [])
const errorMessage = computed(() => isError.value ? '主题列表加载失败' : '')

const { activateTheme: activateThemeMutation, updateThemeConfig } = useThemeMutations()

const activatingId = ref<number | null>(null)
const configVisible = ref(false)
const selectedTheme = ref<Theme | null>(null)
const savingConfig = computed(() => updateThemeConfig.isPending.value)

async function activateTheme(t: Theme) {
  await ElMessageBox.confirm(
    `确认启用主题“${t.name}”？公共站点下次渲染将使用该主题。`,
    '启用主题',
    { type: 'warning', confirmButtonText: '启用', cancelButtonText: '取消' },
  )
  activatingId.value = t.id
  try {
    await activateThemeMutation.mutateAsync(t.id)
    ElMessage.success('主题已激活')
  } catch (error) {
    ElMessage.error(getApiError(error, '主题启用失败'))
  } finally {
    activatingId.value = null
  }
}

function openConfig(theme: Theme) {
  selectedTheme.value = theme
  configVisible.value = true
}

async function saveConfig(config: Record<string, unknown>) {
  if (!selectedTheme.value) return
  try {
    await updateThemeConfig.mutateAsync({ id: selectedTheme.value.id, config })
    selectedTheme.value.config = config
    configVisible.value = false
    ElMessage.success('主题配置已保存')
  } catch (error) {
    ElMessage.error(getApiError(error, '主题配置保存失败'))
  }
}
</script>
<style lang="scss" scoped>
.theme-page {
  .page-header {
    display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 16px;
    h2 { margin: 0; }
    p { margin: 6px 0 0; color: var(--app-text-muted, #6b7280); font-size: 13px; }
  }
  .error-alert { margin-bottom: 16px; }
  .theme-card { margin-bottom: 16px; &.active { border-color: var(--app-color-success, #16a34a); }
    .theme-preview { position: relative; height: 160px; background: var(--app-surface-muted, #f3f4f6); border-radius: 6px; overflow: hidden; margin-bottom: 12px;
      img { width: 100%; height: 100%; object-fit: cover; }
      .placeholder { height: 100%; display: flex; align-items: center; justify-content: center; color: var(--app-text-muted, #6b7280); }
      .active-badge { position: absolute; top: 8px; right: 8px; } }
    .theme-info { h3 { margin: 0 0 6px; } p { font-size: 13px; color: var(--app-text, #374151); margin: 0 0 8px; } } }
}
</style>
