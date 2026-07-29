<template>
  <div
    v-if="loading && !refreshing"
    class="async-state"
    role="status"
    aria-live="polite"
  >
    <el-skeleton
      :rows="rows"
      animated
    />
  </div>
  <div
    v-else-if="forbidden"
    class="async-state async-state--error"
    role="alert"
  >
    <h3>权限不足</h3>
    <p>当前账号没有查看此内容的权限。</p>
  </div>
  <div
    v-else-if="error"
    class="async-state async-state--error"
    role="alert"
  >
    <h3>加载失败</h3>
    <p>{{ errorMessage }}</p>
    <el-button
      v-if="retryable"
      type="primary"
      plain
      @click="$emit('retry')"
    >
      重试
    </el-button>
  </div>
  <EmptyState
    v-else-if="empty"
    :description="emptyText"
  >
    <slot name="empty-action" />
  </EmptyState>
  <div
    v-else
    class="async-state__content"
    :aria-busy="refreshing"
  >
    <div
      v-if="refreshing"
      class="async-state__refresh"
      role="status"
      aria-live="polite"
    >
      正在刷新…
    </div>
    <slot />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { isApiError } from '@/shared/api/types'
import EmptyState from './EmptyState.vue'

const props = withDefaults(defineProps<{
  loading?: boolean
  refreshing?: boolean
  empty?: boolean
  emptyText?: string
  error?: unknown
  rows?: number
  retryable?: boolean
}>(), {
  loading: false,
  refreshing: false,
  empty: false,
  emptyText: '暂无数据',
  error: undefined,
  rows: 4,
  retryable: true,
})

defineEmits<{
  retry: []
}>()

const forbidden = computed(() => isApiError(props.error) && props.error.status === 403)
const errorMessage = computed(() => {
  if (isApiError(props.error)) return props.error.message
  if (props.error instanceof Error) return props.error.message
  return '请求未能完成，请稍后重试。'
})
</script>

<style scoped>
.async-state {
  min-height: 120px;
  padding: var(--cx-space-5);
}

.async-state--error {
  display: grid;
  place-items: center;
  align-content: center;
  gap: var(--cx-space-2);
  color: var(--cx-color-text-secondary);
  text-align: center;
}

.async-state--error h3,
.async-state--error p {
  margin: 0;
}

.async-state__content {
  position: relative;
  min-width: 0;
}

.async-state__refresh {
  position: absolute;
  z-index: 2;
  top: var(--cx-space-2);
  right: var(--cx-space-2);
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--cx-color-bg-subtle);
  color: var(--cx-color-text-muted);
  font-size: var(--cx-font-size-xs);
}
</style>
