<template>
  <el-card
    shadow="never"
    class="section-card"
  >
    <template #header>
      <span>SEO 设置</span>
    </template>
    <el-form
      label-position="top"
    >
      <el-form-item label="Meta 标题">
        <el-input
          :model-value="title"
          placeholder="SEO 标题（留空使用文章标题）"
          @update:model-value="$emit('update:title', $event ?? '')"
        />
      </el-form-item>
      <el-form-item label="Meta 描述">
        <el-input
          :model-value="desc"
          type="textarea"
          :rows="2"
          placeholder="SEO 描述"
          @update:model-value="$emit('update:desc', $event ?? '')"
        />
      </el-form-item>
      <el-form-item label="Meta 关键词">
        <el-input
          :model-value="keywords"
          placeholder="关键词，用逗号分隔"
          @update:model-value="$emit('update:keywords', $event ?? '')"
        />
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  desc: string
  keywords: string
}>()

defineEmits<{
  'update:title': [value: string]
  'update:desc': [value: string]
  'update:keywords': [value: string]
}>()
</script>

<style lang="scss" scoped>
.section-card {
  margin-bottom: 16px;
}
</style>
