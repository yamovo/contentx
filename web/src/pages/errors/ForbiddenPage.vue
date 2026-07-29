<template>
  <main class="forbidden-page">
    <section
      class="forbidden-card"
      role="alert"
    >
      <span class="forbidden-code">403</span>
      <h1>没有管理后台权限</h1>
      <p>
        当前账号无法进入管理工作台。如需访问，请联系管理员调整角色权限。
      </p>
      <el-button
        v-if="authStore.canAccessAdmin"
        type="primary"
        @click="$router.push('/admin')"
      >
        返回后台
      </el-button>
      <el-button @click="logout">
        退出登录
      </el-button>
    </section>
  </main>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

async function logout() {
  await authStore.logout()
  await router.replace('/login')
}
</script>

<style scoped>
.forbidden-page {
  display: grid;
  min-height: 100vh;
  place-items: center;
  padding: var(--cx-space-6);
  background: var(--cx-color-bg-canvas);
}

.forbidden-card {
  max-width: 520px;
  padding: var(--cx-space-8);
  border: 1px solid var(--cx-color-border);
  border-radius: var(--cx-radius-lg);
  background: var(--cx-color-bg-surface);
  box-shadow: var(--cx-shadow-md);
  text-align: center;
}

.forbidden-code {
  color: var(--cx-color-warning);
  font-size: 48px;
  font-weight: 700;
}

.forbidden-card h1 {
  margin: var(--cx-space-2) 0;
}

.forbidden-card p {
  margin: 0 0 var(--cx-space-5);
  color: var(--cx-color-text-secondary);
}
</style>
