<template>
  <main class="service-page">
    <section
      class="service-card"
      role="alert"
    >
      <h1>暂时无法连接 ContentX</h1>
      <p>登录状态仍保留。请检查网络连接，然后重试。</p>
      <el-button
        type="primary"
        :loading="retrying"
        @click="retry"
      >
        重新连接
      </el-button>
      <el-button @click="logout">
        退出登录
      </el-button>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const retrying = ref(false)

async function retry() {
  retrying.value = true
  try {
    await authStore.fetchUser()
    if (!authStore.isAuthenticated) {
      await router.replace('/login')
      return
    }
    await router.replace((route.query.redirect as string) || '/admin')
  } finally {
    retrying.value = false
  }
}

async function logout() {
  await authStore.logout()
  await router.replace('/login')
}
</script>

<style scoped>
.service-page {
  display: grid;
  min-height: 100vh;
  place-items: center;
  padding: var(--cx-space-6);
  background: var(--cx-color-bg-canvas);
}

.service-card {
  max-width: 520px;
  padding: var(--cx-space-8);
  border: 1px solid var(--cx-color-border);
  border-radius: var(--cx-radius-lg);
  background: var(--cx-color-bg-surface);
  box-shadow: var(--cx-shadow-md);
  text-align: center;
}

.service-card p {
  color: var(--cx-color-text-secondary);
}
</style>
