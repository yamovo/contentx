export const roleQueryKeys = {
  all: ['roles'] as const,
  lists: () => [...roleQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...roleQueryKeys.lists(), params] as const,
  detail: (id: number) => [...roleQueryKeys.all, 'detail', id] as const,
  permissions: () => [...roleQueryKeys.all, 'permissions'] as const,
}
