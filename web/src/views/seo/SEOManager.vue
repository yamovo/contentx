<template>
  <div class="seo-page">
    <div class="page-header">
      <h2>SEO 管理</h2>
      <div>
        <el-button @click="$router.push('/admin/seo/redirects')">URL 重定向</el-button>
        <el-button type="primary" @click="previewSitemap">预览 Sitemap</el-button>
      </div>
    </div>

    <el-row :gutter="16">
      <el-col :span="12">
        <el-card shadow="never">
          <template #header><span>全局 SEO 设置</span></template>
          <el-form label-width="120px" label-position="left">
            <el-form-item label="标题分隔符"><el-input v-model="globalSEO.title_separator" /></el-form-item>
            <el-form-item label="首页标题"><el-input v-model="globalSEO.home_title" /></el-form-item>
            <el-form-item label="首页描述"><el-input v-model="globalSEO.home_description" type="textarea" /></el-form-item>
            <el-form-item label="首页关键词"><el-input v-model="globalSEO.home_keywords" /></el-form-item>
            <el-form-item label="启用 Sitemap"><el-switch v-model="globalSEO.enable_sitemap" /></el-form-item>
            <el-form-item label="启用 Robots"><el-switch v-model="globalSEO.enable_robots" /></el-form-item>
            <el-form-item label="Google 统计"><el-input v-model="globalSEO.google_analytics" placeholder="UA-XXXX" /></el-form-item>
            <el-form-item label="百度统计"><el-input v-model="globalSEO.baidu_analytics" /></el-form-item>
          </el-form>
          <el-button type="primary" @click="saveSEO">保存</el-button>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="never">
          <template #header><span>Robots.txt 预览</span></template>
          <pre class="robots-preview">{{ robotsTxt }}</pre>
        </el-card>
        <el-card shadow="never" style="margin-top: 16px">
          <template #header><span>SEO 检查清单</span></template>
          <div class="checklist">
            <div v-for="item in checklist" :key="item.label" class="check-item">
              <el-icon :color="item.ok ? '#67c23a' : '#f56c6c'">
                <CircleCheck v-if="item.ok" /><CircleClose v-else />
              </el-icon>
              <span>{{ item.label }}</span>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { settingsApi } from '@/api'
import { ElMessage } from 'element-plus'

const globalSEO = ref({
  title_separator: ' - ', home_title: '', home_description: '',
  home_keywords: '', enable_sitemap: true, enable_robots: true,
  google_analytics: '', baidu_analytics: '',
})

const robotsTxt = ref('User-agent: *\nAllow: /\nSitemap: /sitemap.xml')

const checklist = computed(() => [
  { label: 'Sitemap 已启用', ok: globalSEO.value.enable_sitemap },
  { label: 'Robots.txt 已配置', ok: globalSEO.value.enable_robots },
  { label: '首页标题已设置', ok: !!globalSEO.value.home_title },
  { label: '首页描述已设置', ok: !!globalSEO.value.home_description },
  { label: 'Google Analytics 已配置', ok: !!globalSEO.value.google_analytics },
])

async function fetchSEO() {
  try {
    const res = await settingsApi.list('seo')
    for (const s of res.data) {
      const key = s.key.replace('seo_', '')
      if (key in globalSEO.value) {
        (globalSEO.value as any)[key] = s.value
      }
    }
  } catch {}
}

async function saveSEO() {
  try {
    const data: Record<string, string> = {}
    for (const [k, v] of Object.entries(globalSEO.value)) {
      data['seo_' + k] = String(v)
    }
    await settingsApi.update(data)
    ElMessage.success('SEO 设置已保存')
  } catch { ElMessage.error('保存失败') }
}

function previewSitemap() {
  window.open('/api/v1/seo/sitemap', '_blank')
}

onMounted(fetchSEO)
</script>

<style lang="scss" scoped>
.seo-page {
  .page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; h2 { margin: 0; } }
  .robots-preview { background: #1d1e2c; color: #abb2bf; padding: 16px; border-radius: 6px; font-size: 13px; line-height: 1.6; }
  .checklist { display: flex; flex-direction: column; gap: 10px; }
  .check-item { display: flex; align-items: center; gap: 8px; font-size: 14px; }
}
</style>
