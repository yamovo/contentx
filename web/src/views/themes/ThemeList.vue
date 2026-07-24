<template>
  <div class="theme-page">
    <div class="page-header">
      <h2>主题管理</h2>
    </div>
    <el-row :gutter="16">
      <el-col
        v-for="t in themes"
        :key="t.id"
        :span="8"
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
              @click="activateTheme(t)"
            >
              启用
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
    <el-empty
      v-if="!themes.length"
      description="暂无已安装的主题"
    />
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { themeApi, type Theme } from '@/api'
import { ElMessage } from 'element-plus'
const themes = ref<Theme[]>([])
async function fetchThemes() { try { themes.value = (await themeApi.list()).data } catch {} }
async function activateTheme(t: Theme) {
  await themeApi.activate(t.id); ElMessage.success('主题已激活'); fetchThemes()
}
onMounted(fetchThemes)
</script>
<style lang="scss" scoped>
.theme-page {
  .page-header { margin-bottom: 16px; h2 { margin: 0; } }
  .theme-card { margin-bottom: 16px; &.active { border-color: #67c23a; }
    .theme-preview { position: relative; height: 160px; background: #f5f7fa; border-radius: 6px; overflow: hidden; margin-bottom: 12px;
      img { width: 100%; height: 100%; object-fit: cover; }
      .placeholder { height: 100%; display: flex; align-items: center; justify-content: center; color: #c0c4cc; }
      .active-badge { position: absolute; top: 8px; right: 8px; } }
    .theme-info { h3 { margin: 0 0 6px; } p { font-size: 13px; color: #606266; margin: 0 0 8px; } } }
}
</style>
