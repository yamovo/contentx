export const menuQueryKeys = {
  all: ['menus'] as const,
  lists: () => [...menuQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...menuQueryKeys.lists(), params] as const,
  detail: (id: number) => [...menuQueryKeys.all, 'detail', id] as const,
}
