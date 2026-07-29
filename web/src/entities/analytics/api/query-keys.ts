export const analyticsQueryKeys = {
  all: ['analytics'] as const,
  lists: () => [...analyticsQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...analyticsQueryKeys.lists(), params] as const,
  detail: (id: number) => [...analyticsQueryKeys.all, 'detail', id] as const,
  dashboard: () => [...analyticsQueryKeys.all, 'dashboard'] as const,
  viewsOverTime: (days: number) => [...analyticsQueryKeys.all, 'viewsOverTime', days] as const,
  topReferrers: () => [...analyticsQueryKeys.all, 'topReferrers'] as const,
  deviceBreakdown: () => [...analyticsQueryKeys.all, 'deviceBreakdown'] as const,
}
