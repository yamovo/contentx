<template>
  <div class="plugin-page">
    <div class="page-header">
      <h2>插件管理</h2>
      <el-tag type="info">{{ plugins.length }} 个插件</el-tag>
    </div>
    <el-row :gutter="16">
      <el-col :span="8" v-for="p in plugins" :key="p.id">
        <el-card shadow="hover" class="plugin-card">
          <div class="plugin-header">
            <h3>{{ p.name }} <el-tag size="small">v{{ p.version }}</el-tag></h3>
            <el-switch v-model="p.is_enabled" @change="togglePlugin(p)" />
          </div>
          <p class="plugin-desc">{{ p.description }}</p>
          <p class="plugin-author">作者: {{ p.author }}</p>
        </el-card>
      </el-col>
    </el-row>
    <el-empty v-if="!plugins.length" description="暂无已安装的插件" />
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { pluginApi, type Plugin } from '@/api'
import { ElMessage } from 'element-plus'
const plugins = ref<Plugin[]>([])
async function fetchPlugins() {
  try { plugins.value = (await pluginApi.list()).data } catch {}
}
async function togglePlugin(p: any) {
  try {
    if (p.is_enabled) await pluginApi.enable(p.id)
    else await pluginApi.disable(p.id)
    ElMessage.success(p.is_enabled ? '插件已启用' : '插件已禁用')
  } catch { ElMessage.error('操作失败') }
}
onMounted(fetchPlugins)
</script>
<style lang="scss" scoped>
.plugin-page {
  .page-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; h2 { margin: 0; } }
  .plugin-card { margin-bottom: 16px; .plugin-header { display: flex; justify-content: space-between; align-items: center; h3 { margin: 0; } }
    .plugin-desc { font-size: 13px; color: #606266; margin: 8px 0; } .plugin-author { font-size: 12px; color: #909399; } }
}
</style>
