<template>
  <el-drawer
    :model-value="modelValue"
    :title="`${title}配置`"
    size="min(640px, 92vw)"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <p class="config-help">
      配置将作为 JSON 对象保存。请只修改你了解的键，未知键会原样保留。
    </p>
    <el-alert
      v-if="error"
      type="error"
      :closable="false"
      show-icon
      :title="error"
      class="config-error"
    />
    <el-input
      v-model="source"
      type="textarea"
      :rows="22"
      spellcheck="false"
      aria-label="JSON 配置"
      class="config-editor"
    />
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">
        取消
      </el-button>
      <el-button
        type="primary"
        :loading="saving"
        @click="save"
      >
        保存配置
      </el-button>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { formatConfig, parseConfig } from './jsonConfig'

const props = defineProps<{
  modelValue: boolean
  title: string
  config?: Record<string, unknown>
  saving?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [config: Record<string, unknown>]
}>()

const source = ref('{}')
const error = ref('')

watch(
  () => [props.modelValue, props.config] as const,
  ([visible]) => {
    if (!visible) return
    source.value = formatConfig(props.config)
    error.value = ''
  },
  { immediate: true },
)

function save() {
  const result = parseConfig(source.value)
  error.value = result.error || ''
  if (!result.config) return
  emit('save', result.config)
}
</script>

<style scoped>
.config-help {
  margin: 0 0 16px;
  color: var(--app-text-muted, #6b7280);
  font-size: 13px;
}

.config-error {
  margin-bottom: 12px;
}

.config-editor :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  line-height: 1.6;
}
</style>

