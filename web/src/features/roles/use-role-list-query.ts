import { useQuery } from '@tanstack/vue-query'
import { roleApi } from '@/api'
import { roleQueryKeys } from '@/entities/role/api/query-keys'

export function useRoleListQuery() {
  return useQuery({
    queryKey: roleQueryKeys.lists(),
    queryFn: () => roleApi.list(),
    placeholderData: (previous) => previous,
  })
}

export function useRolePermissionsQuery() {
  return useQuery({
    queryKey: roleQueryKeys.permissions(),
    queryFn: () => roleApi.permissions(),
  })
}
