import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { analyticsApi } from '@/api'
import { analyticsQueryKeys } from '@/entities/analytics/api/query-keys'

export function useDashboardQuery() {
  return useQuery({
    queryKey: analyticsQueryKeys.dashboard(),
    queryFn: () => analyticsApi.dashboard(),
  })
}

export function useViewsOverTimeQuery(days?: MaybeRefOrGetter<number>) {
  const resolvedDays = computed(() => toValue(days) ?? 30)
  return useQuery({
    queryKey: computed(() => analyticsQueryKeys.viewsOverTime(resolvedDays.value)),
    queryFn: () => analyticsApi.viewsOverTime(resolvedDays.value),
  })
}

export function useTopReferrersQuery() {
  return useQuery({
    queryKey: analyticsQueryKeys.topReferrers(),
    queryFn: () => analyticsApi.topReferrers(),
  })
}

export function useDeviceBreakdownQuery() {
  return useQuery({
    queryKey: analyticsQueryKeys.deviceBreakdown(),
    queryFn: () => analyticsApi.deviceBreakdown(),
  })
}
