<template>
  <section
    class="filter-bar"
    aria-label="筛选条件"
  >
    <slot />
  </section>
</template>

<style scoped>
.filter-bar {
  min-width: 0;
  margin-bottom: var(--cx-space-4);
  padding: var(--cx-space-4);
  border: 1px solid var(--cx-color-border);
  border-radius: var(--cx-radius-md);
  background: var(--cx-color-bg-surface);
  box-shadow: var(--cx-shadow-sm);
}

.filter-bar :deep(.el-form) {
  display: flex;
  flex-wrap: wrap;
  gap: var(--cx-space-2);
}

.filter-bar :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}
</style>
