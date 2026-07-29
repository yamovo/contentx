import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { tagApi } from '@/api'
import { tagQueryKeys } from '@/entities/tag/api/query-keys'

export function useTagListQuery(
  params?: MaybeRefOrGetter<Record<string, unknown>>,
) {
  const resolvedParams = computed(() => params ? { ...toValue(params) } : {})

  return useQuery({
    queryKey: computed(() => tagQueryKeys.list(resolvedParams.value)),
    queryFn: () => tagApi.list(resolvedParams.value),
    placeholderData: (previous) => previous,
  })
}
