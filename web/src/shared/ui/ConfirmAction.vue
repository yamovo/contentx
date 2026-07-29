<template>
  <el-popconfirm
    :title="title"
    :confirm-button-text="confirmText"
    cancel-button-text="取消"
    @confirm="$emit('confirm')"
  >
    <template #reference>
      <slot />
    </template>
  </el-popconfirm>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  confirmText?: string
}>(), {
  confirmText: '确认',
})

defineEmits<{
  confirm: []
}>()
</script>
