import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { tenantsApi, type Tenant } from '@/api/domains/tenants'
import { useAuthStore } from '@/stores/auth'

const STORAGE_KEY = 'current_tenant_id'

/**
 * Tenant context for the admin UI (RFC-001 PR-5).
 *
 * The backend resolves the request tenant from the JWT plus the X-Tenant-ID
 * header; only platform administrators may switch tenants this way, and every
 * switched request re-validates membership and tenant status. The store just
 * carries the selection: inject the header, remember it across reloads, and
 * let the caller wipe data caches after a switch.
 */
export const useTenantStore = defineStore('tenant', () => {
  const stored = localStorage.getItem(STORAGE_KEY)
  const currentTenantID = ref<number | null>(stored ? Number(stored) || null : null)
  const tenants = ref<Tenant[]>([])
  const loading = ref(false)

  const authStore = useAuthStore()

  // Only platform administrators may override the tenant context; everyone
  // else always works inside the tenant bound to their session.
  const canSwitch = computed(() => authStore.isAdmin)

  function setHeaderTenantID(id: number | null) {
    if (id) {
      localStorage.setItem(STORAGE_KEY, String(id))
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
    currentTenantID.value = id
  }

  async function loadTenants() {
    if (!canSwitch.value) {
      tenants.value = []
      return
    }
    loading.value = true
    try {
      const res = await tenantsApi.list()
      tenants.value = res.data
      // The persisted selection may point at a tenant the operator can no
      // longer see; fall back to the default tenant context.
      if (
        currentTenantID.value !== null &&
        !tenants.value.some((t) => t.id === currentTenantID.value)
      ) {
        setHeaderTenantID(null)
      }
    } finally {
      loading.value = false
    }
  }

  /**
   * Switch the admin context to a tenant (null = default tenant). Data caches
   * must be invalidated by the caller after switching — every query needs to
   * refetch under the new tenant scope.
   */
  function switchTo(tenantID: number | null) {
    setHeaderTenantID(tenantID)
  }

  const currentTenant = computed(
    () => tenants.value.find((t) => t.id === currentTenantID.value) || null,
  )

  return {
    currentTenantID,
    currentTenant,
    tenants,
    loading,
    canSwitch,
    loadTenants,
    switchTo,
  }
})
