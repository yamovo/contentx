export const systemQueryKeys = {
  all: ['system'] as const,
  lists: () => [...systemQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...systemQueryKeys.lists(), params] as const,
  detail: (id: number) => [...systemQueryKeys.all, 'detail', id] as const,
  info: () => [...systemQueryKeys.all, 'info'] as const,
  health: () => [...systemQueryKeys.all, 'health'] as const,
  activity: (params: Record<string, unknown>) => [...systemQueryKeys.all, 'activity', params] as const,
}
