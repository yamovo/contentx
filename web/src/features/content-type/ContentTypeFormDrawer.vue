<template>
  <el-drawer
    :model-value="modelValue"
    title="新建内容类型"
    size="min(820px, 94vw)"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-alert
      v-if="errors.length"
      type="error"
      :closable="false"
      show-icon
      class="form-errors"
    >
      <ul>
        <li
          v-for="error in errors"
          :key="error"
        >
          {{ error }}
        </li>
      </ul>
    </el-alert>

    <el-form
      :model="form"
      label-position="top"
      @submit.prevent="submit"
    >
      <el-row :gutter="16">
        <el-col
          :xs="24"
          :sm="12"
        >
          <el-form-item
            label="UID"
            required
          >
            <el-input
              v-model="form.uid"
              placeholder="小写字母/下划线，如 product"
            />
          </el-form-item>
        </el-col>
        <el-col
          :xs="24"
          :sm="12"
        >
          <el-form-item
            label="名称"
            required
          >
            <el-input
              v-model="form.name"
              placeholder="显示名称，如产品"
            />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item label="描述">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="2"
        />
      </el-form-item>

      <div class="switches">
        <el-checkbox v-model="form.is_single">
          单例类型
          <span>仅允许一条内容，例如“关于我们”</span>
        </el-checkbox>
        <el-checkbox v-model="form.draft_publish">
          启用草稿/发布
          <span>新条目先保存为草稿，再单独发布</span>
        </el-checkbox>
      </div>

      <el-divider content-position="left">
        字段定义
      </el-divider>

      <div class="fields">
        <div
          v-for="(field, index) in form.fields"
          :key="index"
          class="field-row"
        >
          <el-input
            v-model="field.name"
            placeholder="字段名"
            aria-label="字段名"
          />
          <el-input
            v-model="field.label"
            placeholder="显示标签"
            aria-label="显示标签"
          />
          <el-select
            v-model="field.field_type"
            aria-label="字段类型"
          >
            <el-option
              v-for="type in contentFieldTypes"
              :key="type.value"
              :value="type.value"
              :label="type.label"
            />
          </el-select>
          <el-input
            v-if="field.field_type === 'enum'"
            v-model="field.optionsText"
            placeholder="枚举值，逗号分隔"
            aria-label="枚举值"
          />
          <div class="field-flags">
            <el-checkbox v-model="field.required">
              必填
            </el-checkbox>
            <el-checkbox v-model="field.unique">
              唯一
            </el-checkbox>
          </div>
          <el-button
            text
            type="danger"
            aria-label="删除字段"
            :disabled="form.fields.length <= 1"
            @click="form.fields.splice(index, 1)"
          >
            删除
          </el-button>
        </div>
      </div>
      <el-button
        text
        type="primary"
        @click="form.fields.push(emptyContentField())"
      >
        添加字段
      </el-button>
    </el-form>

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">
        取消
      </el-button>
      <el-button
        type="primary"
        :loading="saving"
        @click="submit"
      >
        创建
      </el-button>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import type { ContentTypeCreatePayload, ContentTypeDraft } from './model'
import {
  contentFieldTypes,
  emptyContentField,
  emptyContentTypeDraft,
  validateContentTypeDraft,
} from './validation'

const props = defineProps<{
  modelValue: boolean
  saving?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  submit: [payload: ContentTypeCreatePayload]
}>()

const form = reactive<ContentTypeDraft>(emptyContentTypeDraft())
const errors = ref<string[]>([])

watch(
  () => props.modelValue,
  visible => {
    if (!visible) return
    Object.assign(form, emptyContentTypeDraft())
    errors.value = []
  },
  { immediate: true },
)

function submit() {
  const result = validateContentTypeDraft(form)
  errors.value = result.errors
  if (!result.payload) return
  emit('submit', result.payload)
}
</script>

<style scoped>
.form-errors {
  margin-bottom: 18px;
}

.form-errors ul {
  margin: 0;
  padding-left: 20px;
}

.switches {
  display: grid;
  gap: 10px;
}

.switches :deep(.el-checkbox) {
  height: auto;
  align-items: flex-start;
}

.switches span {
  display: block;
  color: var(--app-text-muted, #6b7280);
  font-size: 12px;
}

.fields {
  display: grid;
  gap: 12px;
}

.field-row {
  display: grid;
  grid-template-columns: 1fr 1fr 150px minmax(150px, 1fr) auto auto;
  align-items: start;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--app-border-subtle, #e8eaed);
  border-radius: var(--app-radius-md, 8px);
}

.field-flags {
  display: flex;
  flex-direction: column;
}

@media (max-width: 900px) {
  .field-row {
    grid-template-columns: 1fr 1fr;
  }
}
</style>

