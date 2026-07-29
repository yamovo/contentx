import { useQuery } from '@tanstack/vue-query'
import { themeApi } from '@/api'

const themeQueryKeys = {
  all: ['themes'] as const,
  lists: () => [...themeQueryKeys.all, 'list'] as const,
}

export function useThemeListQuery() {
  return useQuery({
    queryKey: themeQueryKeys.lists(),
    queryFn: () => themeApi.list(),
    placeholderData: (previous) => previous,
  })
}
