export const tagQueryKeys = {
  all: ['tags'] as const,
  lists: () => [...tagQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...tagQueryKeys.lists(), params] as const,
  detail: (id: number) => [...tagQueryKeys.all, 'detail', id] as const,
}
