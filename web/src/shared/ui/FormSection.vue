<template>
  <section class="form-section">
    <header v-if="title || description">
      <h2 v-if="title">
        {{ title }}
      </h2>
      <p v-if="description">
        {{ description }}
      </p>
    </header>
    <slot />
  </section>
</template>

<script setup lang="ts">
defineProps<{
  title?: string
  description?: string
}>()
</script>

<style scoped>
.form-section {
  padding: var(--cx-space-5);
  border: 1px solid var(--cx-color-border);
  border-radius: var(--cx-radius-md);
  background: var(--cx-color-bg-surface);
}

.form-section h2 {
  margin: 0 0 var(--cx-space-1);
  font-size: var(--cx-font-size-lg);
}

.form-section p {
  margin: 0 0 var(--cx-space-4);
  color: var(--cx-color-text-muted);
}
</style>
