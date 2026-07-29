import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { articleApi } from '@/api'
import { articleQueryKeys } from '@/entities/article/api/query-keys'

export function useArticleDetailQuery(id: MaybeRefOrGetter<number | undefined | null>) {
  const resolvedId = computed(() => toValue(id))

  return useQuery({
    queryKey: computed(() =>
      resolvedId.value ? articleQueryKeys.detail(resolvedId.value) : ['articles', 'detail', 'none'],
    ),
    queryFn: () => articleApi.get(resolvedId.value!),
    enabled: computed(() => !!resolvedId.value),
  })
}
