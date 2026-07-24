<template>
  <div class="editor-topbar">
    <div class="topbar-left">
      <el-button
        text
        @click="$emit('back')"
      >
        <el-icon><ArrowLeft /></el-icon> 返回
      </el-button>
      <span class="editor-title">{{ isEdit ? '编辑文章' : '写文章' }}</span>
    </div>
    <div class="topbar-right">
      <el-button
        :loading="saving"
        @click="$emit('save-draft')"
      >
        保存草稿
      </el-button>
      <el-button
        type="primary"
        :loading="saving"
        @click="$emit('publish')"
      >
        {{ status === 'published' ? '更新' : '发布' }}
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue'

defineProps<{
  isEdit: boolean
  saving: boolean
  status: string
}>()

defineEmits<{
  back: []
  'save-draft': []
  publish: []
}>()
</script>

<style lang="scss" scoped>
.editor-topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding: 12px 0;

  .topbar-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .editor-title {
    font-size: 18px;
    font-weight: 600;
  }
}
</style>
