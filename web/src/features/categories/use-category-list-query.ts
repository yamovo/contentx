import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { categoryApi } from '@/api'
import { categoryQueryKeys } from '@/entities/category/api/query-keys'

export function useCategoryListQuery(
  params?: MaybeRefOrGetter<Record<string, unknown>>,
) {
  const resolvedParams = computed(() => params ? { ...toValue(params) } : {})

  return useQuery({
    queryKey: computed(() => categoryQueryKeys.list(resolvedParams.value)),
    queryFn: () => categoryApi.list(resolvedParams.value),
    placeholderData: (previous) => previous,
  })
}
