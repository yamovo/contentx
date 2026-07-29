<template>
  <div class="app-shell">
    <aside
      v-if="$slots.sidebar"
      class="app-shell__sidebar"
    >
      <slot name="sidebar" />
    </aside>
    <div class="app-shell__body">
      <header
        v-if="$slots.header"
        class="app-shell__header"
      >
        <slot name="header" />
      </header>
      <main
        id="main-content"
        class="app-shell__content"
        tabindex="-1"
      >
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  min-width: 0;
  min-height: 100vh;
  background: var(--cx-color-bg-canvas);
}

.app-shell__sidebar {
  flex: 0 0 auto;
}

.app-shell__body {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
}

.app-shell__header {
  position: relative;
  z-index: var(--cx-z-header);
}

.app-shell__content {
  min-width: 0;
  flex: 1;
}
</style>
