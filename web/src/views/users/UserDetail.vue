<template>
  <div class="user-detail">
    <el-button
      text
      @click="$router.back()"
    >
      <el-icon><ArrowLeft /></el-icon> 返回
    </el-button>
    <el-card
      v-if="user"
      shadow="never"
      class="user-card"
    >
      <div class="user-header">
        <el-avatar
          :src="user.avatar"
          :size="80"
        >
          {{ (user.display_name || 'U')[0] }}
        </el-avatar>
        <div class="user-info">
          <h2>
            {{ user.display_name }} <el-tag size="small">
              {{ user.role?.name }}
            </el-tag>
          </h2>
          <p>@{{ user.username }} · {{ user.email }}</p>
          <p v-if="user.bio">
            {{ user.bio }}
          </p>
          <p class="meta">
            注册于 {{ formatDate(user.created_at, 'YYYY-MM-DD') }} · 登录 {{ user.login_count }} 次
          </p>
        </div>
      </div>
    </el-card>
    <el-empty
      v-else
      description="加载中..."
    />
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { userApi, type User } from '@/api'
import { formatDate } from '@/utils'
const route = useRoute()
const user = ref<User | null>(null)
onMounted(async () => {
  const res = await userApi.get(Number(route.params.id))
  user.value = res.data
})
</script>
<style lang="scss" scoped>
.user-detail { .user-header { display: flex; gap: 20px; align-items: center; }
  h2 { margin: 0 0 8px; } .meta { color: #909399; font-size: 13px; } }
</style>
