<template>
  <div
    class="empty-state"
    role="status"
  >
    <el-empty
      :description="description"
      :image-size="imageSize"
    >
      <slot />
    </el-empty>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  description?: string
  imageSize?: number
}>(), {
  description: '暂无数据',
  imageSize: 80,
})
</script>

<style scoped>
.empty-state {
  padding: var(--cx-space-6);
  color: var(--cx-color-text-muted);
  text-align: center;
}
</style>
