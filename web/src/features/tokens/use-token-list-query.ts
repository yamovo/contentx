import { useQuery } from '@tanstack/vue-query'
import { tokenApi } from '@/api'
import { tokenQueryKeys } from '@/entities/token/api/query-keys'

export function useTokenListQuery() {
  return useQuery({
    queryKey: tokenQueryKeys.lists(),
    queryFn: () => tokenApi.list(),
    placeholderData: (previous) => previous,
  })
}
