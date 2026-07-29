export const categoryQueryKeys = {
  all: ['categories'] as const,
  lists: () => [...categoryQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...categoryQueryKeys.lists(), params] as const,
  detail: (id: number) => [...categoryQueryKeys.all, 'detail', id] as const,
}
