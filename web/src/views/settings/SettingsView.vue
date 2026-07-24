<template>
  <div class="settings-page">
    <h2>系统设置</h2>

    <el-tabs
      v-model="activeGroup"
      @tab-change="fetchSettings"
    >
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
              :label="s.label || s.key"
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
import { ref, onMounted } from 'vue'
import { settingsApi, type SiteSetting } from '@/api'
import { ElMessage } from 'element-plus'

const activeGroup = ref('general')
const settings = ref<Record<string, SiteSetting[]>>({})
const groups = ref<string[]>([])
const saving = ref(false)

const groupLabels: Record<string, string> = {
  general: '常规', content: '内容', reading: '阅读', seo: 'SEO',
  social: '社交媒体', email: '邮件', media: '媒体', cache: '缓存',
}

async function fetchSettings() {
  try {
    const res = await settingsApi.list()
    settings.value = res.grouped
    groups.value = Object.keys(res.grouped)
    if (groups.value.length && !groups.value.includes(activeGroup.value)) {
      activeGroup.value = groups.value[0]
    }
  } catch {}
}

async function saveSettings() {
  saving.value = true
  try {
    const data: Record<string, string> = {}
    for (const group of Object.values(settings.value)) {
      for (const s of group) {
        data[s.key] = String(s.value)
      }
    }
    await settingsApi.update(data)
    ElMessage.success('设置已保存')
  } catch { ElMessage.error('保存失败') }
  finally { saving.value = false }
}

onMounted(fetchSettings)
</script>

<style lang="scss" scoped>
.settings-page {
  h2 { margin-bottom: 16px; }
  .help-text { font-size: 12px; color: #909399; margin-top: 4px; }
  .save-bar { margin-top: 20px; text-align: right; }
}
</style>
