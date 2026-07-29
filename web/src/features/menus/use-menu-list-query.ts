import { useQuery } from '@tanstack/vue-query'
import { menuApi } from '@/api'
import { menuQueryKeys } from '@/entities/menu/api/query-keys'

export function useMenuListQuery() {
  return useQuery({
    queryKey: menuQueryKeys.lists(),
    queryFn: () => menuApi.list(),
    placeholderData: (previous) => previous,
  })
}
