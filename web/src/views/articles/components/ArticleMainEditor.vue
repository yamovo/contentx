<template>
  <el-card shadow="never">
    <el-form
      label-position="top"
    >
      <!-- Title -->
      <el-form-item>
        <el-input
          :model-value="title"
          placeholder="输入文章标题..."
          class="title-input"
          @update:model-value="$emit('update:title', $event ?? '')"
          @input="$emit('auto-slug')"
        />
      </el-form-item>

      <!-- Slug -->
      <el-form-item>
        <el-input
          :model-value="slug"
          placeholder="URL 别名"
          class="slug-input"
          @update:model-value="$emit('update:slug', $event ?? '')"
        >
          <template #prepend>
            slug:
          </template>
        </el-input>
      </el-form-item>

      <!-- Editor Tabs -->
      <el-form-item>
        <el-tabs
          :model-value="editorMode"
          class="editor-tabs"
          @update:model-value="$emit('update:editor-mode', $event as string)"
        >
          <el-tab-pane
            label="Markdown"
            name="markdown"
          >
            <div class="markdown-editor">
              <el-input
                :model-value="content"
                type="textarea"
                :autosize="{ minRows: 20, maxRows: 50 }"
                placeholder="使用 Markdown 写作..."
                class="md-textarea"
                @update:model-value="$emit('update:content', $event ?? '')"
              />
            </div>
          </el-tab-pane>
          <el-tab-pane
            label="预览"
            name="preview"
          >
            <div
              class="preview-pane"
              v-html="renderedContent"
            />
          </el-tab-pane>
        </el-tabs>
      </el-form-item>

      <!-- Excerpt -->
      <el-form-item label="摘要">
        <el-input
          :model-value="excerpt"
          type="textarea"
          :rows="3"
          placeholder="文章摘要（留空自动生成）"
          @update:model-value="$emit('update:excerpt', $event ?? '')"
        />
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  slug: string
  content: string
  excerpt: string
  editorMode: string
  renderedContent: string
}>()

defineEmits<{
  'update:title': [value: string]
  'update:slug': [value: string]
  'update:content': [value: string]
  'update:excerpt': [value: string]
  'update:editor-mode': [value: string]
  'auto-slug': []
}>()
</script>

<style lang="scss" scoped>
.title-input :deep(.el-input__inner) {
  font-size: 24px;
  font-weight: 600;
  border: none;
  padding: 0;
}

.slug-input :deep(.el-input-group__prepend) {
  background: #f5f7fa;
}

.editor-tabs { width: 100%; }

.markdown-editor .md-textarea :deep(textarea) {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 14px;
  line-height: 1.8;
  padding: 16px;
}

.preview-pane {
  padding: 16px;
  line-height: 1.8;
  font-size: 15px;

  :deep(h1), :deep(h2), :deep(h3) { margin: 20px 0 10px; }
  :deep(img) { max-width: 100%; border-radius: 8px; }
  :deep(code) {
    background: #f5f7fa;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 13px;
  }
  :deep(pre code) { background: none; padding: 0; }
}
</style>
