import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { settingsApi } from '@/api'
import { settingsQueryKeys } from '@/entities/settings/api/query-keys'

export function useSettingsQuery(group?: MaybeRefOrGetter<string | undefined>) {
  const resolvedGroup = computed(() => toValue(group))

  return useQuery({
    queryKey: computed(() =>
      resolvedGroup.value
        ? settingsQueryKeys.group(resolvedGroup.value)
        : settingsQueryKeys.list({}),
    ),
    queryFn: () => settingsApi.list(resolvedGroup.value),
  })
}

export function usePublicSettingsQuery() {
  return useQuery({
    queryKey: settingsQueryKeys.public(),
    queryFn: () => settingsApi.public(),
  })
}
