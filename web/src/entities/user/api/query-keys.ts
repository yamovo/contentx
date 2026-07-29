export const userQueryKeys = {
  all: ['users'] as const,
  lists: () => [...userQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...userQueryKeys.lists(), params] as const,
  detail: (id: number) => [...userQueryKeys.all, 'detail', id] as const,
}
