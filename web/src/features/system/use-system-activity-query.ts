import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { systemApi } from '@/api'
import { systemQueryKeys } from '@/entities/system/api/query-keys'

export function useSystemActivityQuery(
  params?: MaybeRefOrGetter<Record<string, unknown>>,
) {
  const resolvedParams = computed(() => params ? { ...toValue(params) } : {})

  return useQuery({
    queryKey: computed(() => systemQueryKeys.activity(resolvedParams.value)),
    queryFn: () => systemApi.activity(resolvedParams.value),
    placeholderData: (previous) => previous,
  })
}
