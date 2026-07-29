<template>
  <slot v-if="allowed" />
  <slot
    v-else
    name="fallback"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import type { PermissionSlug } from '@/shared/auth/permissions'

const props = defineProps<{
  permission?: PermissionSlug
  anyOf?: readonly PermissionSlug[]
  allOf?: readonly PermissionSlug[]
}>()

const authStore = useAuthStore()
const allowed = computed(() => {
  if (props.permission) return authStore.hasPermission(props.permission)
  if (props.anyOf?.length) return authStore.hasAnyPermission(props.anyOf)
  if (props.allOf?.length) return props.allOf.every(authStore.hasPermission)
  return true
})
</script>
