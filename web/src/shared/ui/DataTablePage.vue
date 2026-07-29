<template>
  <section class="data-table-page">
    <div class="data-table-page__scroll">
      <slot />
    </div>
    <footer
      v-if="$slots.pagination"
      class="data-table-page__pagination"
    >
      <slot name="pagination" />
    </footer>
  </section>
</template>

<style scoped>
.data-table-page {
  min-width: 0;
  border: 1px solid var(--cx-color-border);
  border-radius: var(--cx-radius-md);
  background: var(--cx-color-bg-surface);
  box-shadow: var(--cx-shadow-sm);
}

.data-table-page__scroll {
  min-width: 0;
  overflow-x: auto;
}

.data-table-page__pagination {
  display: flex;
  justify-content: flex-end;
  padding: var(--cx-space-4);
  border-top: 1px solid var(--cx-color-border);
}
</style>
