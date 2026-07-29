<template>
  <el-drawer
    :model-value="modelValue"
    :title="entry ? '编辑条目' : '新建条目'"
    size="min(680px, 92vw)"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-alert
      v-if="errors.length"
      type="error"
      :closable="false"
      show-icon
      class="editor-errors"
    >
      <template #title>
        请修正 {{ errors.length }} 个字段
      </template>
      <ul>
        <li
          v-for="error in errors"
          :key="error.field"
        >
          {{ error.label }}：{{ error.message }}
        </li>
      </ul>
    </el-alert>

    <el-form
      label-position="top"
      :model="entryData"
      @submit.prevent="submit"
    >
      <el-form-item
        v-for="field in sortedFields"
        :key="field.name"
        :label="field.label"
        :required="field.required"
        :error="errorFor(field.name)"
      >
        <ContentFieldRenderer
          v-model="entryData[field.name]"
          :field="field"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="drawer-footer">
        <span class="draft-hint">新条目始终先保存为草稿，可在列表中单独发布。</span>
        <el-button @click="emit('update:modelValue', false)">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="submit"
        >
          保存草稿
        </el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { ContentEntry, ContentField } from '@/api'
import type { ContentEntryData } from '@/entities/content/model'
import ContentFieldRenderer from './ContentFieldRenderer.vue'
import {
  buildContentPayload,
  createInitialEntryData,
  sortedContentFields,
  type ContentValidationError,
} from './validation'

const props = defineProps<{
  modelValue: boolean
  fields: ContentField[]
  entry: ContentEntry | null
  saving?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  submit: [data: Record<string, unknown>]
}>()

const entryData = ref<ContentEntryData>({})
const errors = ref<ContentValidationError[]>([])
const sortedFields = computed(() => sortedContentFields(props.fields))

watch(
  () => [props.modelValue, props.entry, props.fields] as const,
  ([visible]) => {
    if (!visible) return
    entryData.value = createInitialEntryData(props.fields, props.entry?.data || {})
    errors.value = []
  },
  { immediate: true },
)

function errorFor(field: string): string {
  return errors.value.find(error => error.field === field)?.message || ''
}

function submit() {
  const result = buildContentPayload(props.fields, entryData.value)
  errors.value = result.errors
  if (errors.value.length) return
  emit('submit', result.data)
}
</script>

<style scoped>
.editor-errors {
  margin-bottom: 20px;
}

.editor-errors ul {
  margin: 8px 0 0;
  padding-left: 20px;
}

.drawer-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
}

.draft-hint {
  margin-right: auto;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>

