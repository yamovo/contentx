import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { userApi } from '@/api'
import { userQueryKeys } from '@/entities/user/api/query-keys'

export function useUserListQuery(
  params?: MaybeRefOrGetter<Record<string, unknown>>,
) {
  const resolvedParams = computed(() => params ? { ...toValue(params) } : {})

  return useQuery({
    queryKey: computed(() => userQueryKeys.list(resolvedParams.value)),
    queryFn: () => userApi.list(resolvedParams.value),
    placeholderData: (previous) => previous,
  })
}
