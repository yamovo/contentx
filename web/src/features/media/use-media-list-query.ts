import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { mediaApi } from '@/api'
import { mediaQueryKeys } from '@/entities/media/api/query-keys'

export function useMediaListQuery(
  params?: MaybeRefOrGetter<Record<string, unknown>>,
) {
  const resolvedParams = computed(() => params ? { ...toValue(params) } : {})

  return useQuery({
    queryKey: computed(() => mediaQueryKeys.list(resolvedParams.value)),
    queryFn: () => mediaApi.list(resolvedParams.value),
    placeholderData: (previous) => previous,
  })
}

export function useMediaFoldersQuery() {
  return useQuery({
    queryKey: computed(() => mediaQueryKeys.folders()),
    queryFn: () => mediaApi.folders(),
  })
}

export function useMediaStatsQuery() {
  return useQuery({
    queryKey: computed(() => mediaQueryKeys.stats()),
    queryFn: () => mediaApi.stats(),
  })
}
