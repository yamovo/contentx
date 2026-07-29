import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { articleApi } from '@/api'
import { articleQueryKeys } from '@/entities/article/api/query-keys'

export function useArticleListQuery(
  params: MaybeRefOrGetter<Record<string, unknown>>,
) {
  const resolvedParams = computed(() => ({ ...toValue(params) }))

  return useQuery({
    queryKey: computed(() => articleQueryKeys.list(resolvedParams.value)),
    queryFn: ({ signal }) => articleApi.list(resolvedParams.value, signal),
    placeholderData: (previous) => previous,
  })
}
