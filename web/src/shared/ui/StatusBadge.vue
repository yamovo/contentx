<template>
  <span
    class="status-badge"
    :data-tone="tone"
  >
    <span
      class="status-badge__dot"
      aria-hidden="true"
    />
    <slot>{{ label }}</slot>
  </span>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  label?: string
  tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info'
}>(), {
  label: '',
  tone: 'neutral',
})
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: var(--cx-space-1);
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, currentColor 10%, transparent);
  color: var(--cx-color-info);
  font-size: var(--cx-font-size-xs);
  font-weight: 600;
}

.status-badge[data-tone='success'] { color: var(--cx-color-success); }
.status-badge[data-tone='warning'] { color: var(--cx-color-warning); }
.status-badge[data-tone='danger'] { color: var(--cx-color-danger); }
.status-badge[data-tone='info'] { color: var(--cx-color-primary); }

.status-badge__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
</style>
