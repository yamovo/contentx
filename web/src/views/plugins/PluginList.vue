<template>
  <div class="plugin-page">
    <div class="page-header">
      <div>
        <h2>插件管理</h2>
        <p>管理已安装插件的启用状态和运行配置。</p>
      </div>
      <div class="header-actions">
        <el-tag type="info">
          {{ plugins.length }} 个插件
        </el-tag>
        <el-button
          :loading="loading"
          @click="refetch"
        >
          刷新
        </el-button>
      </div>
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
        v-for="p in plugins"
        :key="p.id"
        :xs="24"
        :md="12"
        :xl="8"
      >
        <el-card
          shadow="hover"
          class="plugin-card"
        >
          <div class="plugin-header">
            <h3>
              {{ p.name }} <el-tag size="small">
                v{{ p.version }}
              </el-tag>
            </h3>
            <el-switch
              :model-value="p.is_enabled"
              :loading="pendingIds.has(p.id)"
              :aria-label="`${p.is_enabled ? '禁用' : '启用'} ${p.name}`"
              @change="togglePlugin(p, Boolean($event))"
            />
          </div>
          <p class="plugin-desc">
            {{ p.description }}
          </p>
          <p class="plugin-author">
            作者: {{ p.author }}
          </p>
          <el-button
            text
            type="primary"
            @click="openConfig(p)"
          >
            编辑配置
          </el-button>
        </el-card>
      </el-col>
    </el-row>
    <el-empty
      v-if="!plugins.length"
      :description="loading ? '正在加载插件' : '暂无已安装的插件'"
    />
    <JsonConfigDrawer
      v-model="configVisible"
      :title="selectedPlugin?.name || '插件'"
      :config="selectedPlugin?.config"
      :saving="savingConfig"
      @save="saveConfig"
    />
  </div>
</template>
<script setup lang="ts">
import { ref, computed } from 'vue'
import { type Plugin } from '@/api'
import { ElMessage } from 'element-plus'
import JsonConfigDrawer from '@/features/config/JsonConfigDrawer.vue'
import { getApiError } from '@/utils'
import { usePluginListQuery } from '@/features/plugins/use-plugin-list-query'
import { usePluginMutations } from '@/features/plugins/use-plugin-mutations'

const { data: queryData, isLoading: loading, isError, refetch } = usePluginListQuery()
const plugins = computed<Plugin[]>(() => queryData.value?.data ?? [])
const errorMessage = computed(() => isError.value ? '插件列表加载失败' : '')

const { enablePlugin, disablePlugin, updatePluginConfig } = usePluginMutations()

const pendingIds = ref(new Set<number>())
const configVisible = ref(false)
const selectedPlugin = ref<Plugin | null>(null)
const savingConfig = computed(() => updatePluginConfig.isPending.value)

async function togglePlugin(plugin: Plugin, enabled: boolean) {
  pendingIds.value = new Set(pendingIds.value).add(plugin.id)
  try {
    if (enabled) await enablePlugin.mutateAsync(plugin.id)
    else await disablePlugin.mutateAsync(plugin.id)
    ElMessage.success(enabled ? '插件已启用' : '插件已禁用')
  } catch (error) {
    ElMessage.error(getApiError(error, '插件状态更新失败'))
  } finally {
    const next = new Set(pendingIds.value)
    next.delete(plugin.id)
    pendingIds.value = next
  }
}

function openConfig(plugin: Plugin) {
  selectedPlugin.value = plugin
  configVisible.value = true
}

async function saveConfig(config: Record<string, unknown>) {
  if (!selectedPlugin.value) return
  try {
    await updatePluginConfig.mutateAsync({ id: selectedPlugin.value.id, config })
    selectedPlugin.value.config = config
    configVisible.value = false
    ElMessage.success('插件配置已保存')
  } catch (error) {
    ElMessage.error(getApiError(error, '插件配置保存失败'))
  }
}
</script>
<style lang="scss" scoped>
.plugin-page {
  .page-header {
    display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 16px;
    h2 { margin: 0; }
    p { margin: 6px 0 0; color: var(--app-text-muted, #6b7280); font-size: 13px; }
  }
  .header-actions { display: flex; align-items: center; gap: 8px; }
  .error-alert { margin-bottom: 16px; }
  .plugin-card { margin-bottom: 16px; .plugin-header { display: flex; justify-content: space-between; align-items: center; h3 { margin: 0; } }
    .plugin-desc { font-size: 13px; color: var(--app-text, #374151); margin: 8px 0; }
    .plugin-author { font-size: 12px; color: var(--app-text-muted, #6b7280); } }
}
</style>
