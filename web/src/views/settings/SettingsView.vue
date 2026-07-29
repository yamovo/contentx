<template>
  <div class="settings-page">
    <h2>系统设置</h2>

    <el-tabs v-model="activeGroup">
      <el-tab-pane
        v-for="g in groups"
        :key="g"
        :label="groupLabels[g] || g"
        :name="g"
      >
        <el-card shadow="never">
          <el-form
            label-width="140px"
            label-position="left"
          >
            <el-form-item
              v-for="s in settings[g]"
              :key="s.key"
              :label="s.label || settingLabels[s.key] || s.key"
            >
              <el-input
                v-if="s.type === 'string'"
                v-model="s.value"
              />
              <el-input
                v-else-if="s.type === 'text'"
                v-model="s.value"
                type="textarea"
                :rows="3"
              />
              <el-input-number
                v-else-if="s.type === 'int'"
                :model-value="Number(s.value)"
                @update:model-value="s.value = String($event)"
              />
              <el-switch
                v-else-if="s.type === 'bool'"
                v-model="s.value"
                active-value="true"
                inactive-value="false"
              />
              <el-input
                v-else
                v-model="s.value"
              />
              <div
                v-if="s.help_text"
                class="help-text"
              >
                {{ s.help_text }}
              </div>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <div class="save-bar">
      <el-button
        type="primary"
        :loading="saving"
        @click="saveSettings"
      >
        保存设置
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { type SiteSetting } from '@/api'
import { ElMessage } from 'element-plus'
import { useSettingsQuery } from '@/features/settings/use-settings-query'
import { useSettingsMutation } from '@/features/settings/use-settings-mutation'

const activeGroup = ref('general')

const groupLabels: Record<string, string> = {
  general: '常规', content: '内容', reading: '阅读', writing: '写作',
  discussion: '评论讨论', users: '用户', seo: 'SEO',
  social: '社交媒体', email: '邮件', media: '媒体', cache: '缓存',
}

// 后端 seed 未写入 label，数据库里的设置项只有英文键名；
// 这里做一层中文映射兑底，后端 label 优先。
const settingLabels: Record<string, string> = {
  site_name: '站点名称',
  site_description: '站点描述',
  site_url: '站点地址',
  site_logo: '站点 Logo',
  site_favicon: '站点图标',
  site_language: '站点语言',
  site_timezone: '时区',
  posts_per_page: '每页文章数',
  default_category: '默认分类',
  enable_comments: '开启评论',
  moderate_comments: '评论先审后发',
  allow_registration: '允许注册',
  default_role: '新用户默认角色',
}

const { data: queryData } = useSettingsQuery()
const settings = computed<Record<string, SiteSetting[]>>(() => queryData.value?.grouped ?? {})
const groups = computed(() => Object.keys(settings.value))

if (groups.value.length && !groups.value.includes(activeGroup.value)) {
  activeGroup.value = groups.value[0]
}

const { updateSettings } = useSettingsMutation()
const saving = computed(() => updateSettings.isPending.value)

async function saveSettings() {
  try {
    const data: Record<string, string> = {}
    for (const group of Object.values(settings.value)) {
      for (const s of group) {
        data[s.key] = String(s.value)
      }
    }
    await updateSettings.mutateAsync(data)
    ElMessage.success('设置已保存')
  } catch { ElMessage.error('保存失败') }
}
</script>

<style lang="scss" scoped>
.settings-page {
  h2 { margin-bottom: 16px; }
  .help-text { font-size: 12px; color: #909399; margin-top: 4px; }
  .save-bar { margin-top: 20px; text-align: right; }
}
</style>
