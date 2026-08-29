<template>
  <el-select
    v-if="tenantStore.canSwitch"
    :model-value="tenantStore.currentTenantID ?? 0"
    class="tenant-switcher"
    size="default"
    :loading="tenantStore.loading"
    placeholder="默认租户"
    @change="onSwitch"
  >
    <el-option
      label="默认租户"
      :value="0"
    />
    <el-option
      v-for="tenant in tenantStore.tenants"
      :key="tenant.id"
      :label="tenant.name"
      :value="tenant.id"
    >
      <span class="tenant-option">
        {{ tenant.name }}
        <el-tag
          v-if="tenant.status === 'suspended'"
          size="small"
          type="danger"
        >
          已停用
        </el-tag>
      </span>
    </el-option>
  </el-select>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { queryClient } from '@/app/providers/vue-query'
import { useTenantStore } from '@/stores/tenant'

const tenantStore = useTenantStore()
const router = useRouter()

onMounted(() => {
  void tenantStore.loadTenants()
})

// Switching the context changes the meaning of every cached response, so the
// whole query cache is dropped and the current view refetches under the new
// tenant scope. Non-admin selections are rejected server-side, and every
// switched request re-validates membership and tenant state.
async function onSwitch(value: number) {
  // 0 is the "default tenant" sentinel: el-option values cannot be null.
  const tenantID = value === 0 ? null : value
  if (tenantID === tenantStore.currentTenantID) {
    return
  }
  tenantStore.switchTo(tenantID)
  queryClient.clear()
  ElMessage.success(tenantID === null ? '已切回默认租户' : '已切换租户，数据已刷新')
  await router.push({ path: '/admin' })
}
</script>

<style scoped>
.tenant-switcher {
  width: 160px;
  margin-right: 12px;
}

.tenant-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
</style>
