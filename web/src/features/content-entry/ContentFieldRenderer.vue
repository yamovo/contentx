<template>
  <el-switch
    v-if="field.field_type === 'boolean'"
    :model-value="boolValue"
    @update:model-value="emitValue"
  />
  <el-input-number
    v-else-if="field.field_type === 'integer' || field.field_type === 'float'"
    :model-value="numValue"
    :precision="field.field_type === 'integer' ? 0 : undefined"
    :min="field.min_value ?? undefined"
    :max="field.max_value ?? undefined"
    controls-position="right"
    class="field-control field-control--number"
    @update:model-value="emitValue"
  />
  <el-date-picker
    v-else-if="field.field_type === 'date'"
    :model-value="strValue"
    type="datetime"
    value-format="YYYY-MM-DDTHH:mm:ssZ"
    class="field-control"
    @update:model-value="emitValue"
  />
  <el-select
    v-else-if="field.field_type === 'enum'"
    :model-value="strValue"
    clearable
    class="field-control"
    @update:model-value="emitValue"
  >
    <el-option
      v-for="option in field.options || []"
      :key="option"
      :value="option"
      :label="option"
    />
  </el-select>
  <el-input
    v-else-if="field.field_type === 'rich_text' || field.field_type === 'json'"
    :model-value="strValue"
    type="textarea"
    :rows="field.field_type === 'rich_text' ? 8 : 5"
    :placeholder="field.field_type === 'json' ? '{ &quot;key&quot;: &quot;value&quot; }' : '输入 Markdown 内容'"
    @update:model-value="emitValue"
  />
  <el-input
    v-else
    :model-value="strValue"
    :maxlength="field.max_length ?? undefined"
    :placeholder="placeholder"
    clearable
    @update:model-value="emitValue"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ContentField } from '@/api'
import type { ContentEntryValue } from '@/entities/content/model'

const props = defineProps<{
  field: ContentField
  modelValue: ContentEntryValue
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ContentEntryValue]
}>()

const boolValue = computed(() => props.modelValue as boolean)
const numValue = computed(() => props.modelValue as number | undefined)
const strValue = computed(() => props.modelValue as string | undefined)

const placeholder = computed(() => {
  switch (props.field.field_type) {
    case 'email': return 'name@example.com'
    case 'url': return 'https://example.com'
    case 'slug': return 'my-entry-slug'
    case 'media': return '媒体文件 URL 或 ID'
    case 'relation': return `关联 ${props.field.relation_uid || '条目'} 的 document_id`
    default: return ''
  }
})

function emitValue(value: ContentEntryValue) {
  emit('update:modelValue', value)
}
</script>

<style scoped>
.field-control {
  width: min(100%, 320px);
}

.field-control--number {
  width: 220px;
}
</style>

