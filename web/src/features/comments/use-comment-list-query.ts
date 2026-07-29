import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { commentApi } from '@/api'
import { commentQueryKeys } from '@/entities/comment/api/query-keys'

export function useCommentListQuery(
  params?: MaybeRefOrGetter<Record<string, unknown>>,
) {
  const resolvedParams = computed(() => params ? { ...toValue(params) } : {})

  return useQuery({
    queryKey: computed(() => commentQueryKeys.list(resolvedParams.value)),
    queryFn: () => commentApi.list(resolvedParams.value),
    placeholderData: (previous) => previous,
  })
}

export function useCommentStatsQuery() {
  return useQuery({
    queryKey: computed(() => commentQueryKeys.stats()),
    queryFn: () => commentApi.stats(),
  })
}
