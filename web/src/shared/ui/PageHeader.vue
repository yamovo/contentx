<template>
  <header class="page-header">
    <div class="page-header__title">
      <h1>{{ title }}</h1>
      <p v-if="description">
        {{ description }}
      </p>
    </div>
    <div
      v-if="$slots.actions"
      class="page-header__actions"
    >
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--cx-space-4);
  margin-bottom: var(--cx-space-5);
}

.page-header h1 {
  margin: 0;
  color: var(--cx-color-text-primary);
  font-size: var(--cx-font-size-xl);
  line-height: 1.35;
}

.page-header p {
  margin: var(--cx-space-1) 0 0;
  color: var(--cx-color-text-muted);
}

.page-header__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--cx-space-2);
}

@media (max-width: 720px) {
  .page-header {
    flex-direction: column;
  }

  .page-header__actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
