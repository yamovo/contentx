import { useQuery } from '@tanstack/vue-query'
import { pluginApi } from '@/api'
import { pluginQueryKeys } from '@/entities/plugin/api/query-keys'

export function usePluginListQuery() {
  return useQuery({
    queryKey: pluginQueryKeys.lists(),
    queryFn: () => pluginApi.list(),
    placeholderData: (previous) => previous,
  })
}
