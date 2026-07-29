export const mediaQueryKeys = {
  all: ['media'] as const,
  lists: () => [...mediaQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...mediaQueryKeys.lists(), params] as const,
  detail: (id: number) => [...mediaQueryKeys.all, 'detail', id] as const,
  folders: () => [...mediaQueryKeys.all, 'folders'] as const,
  stats: () => [...mediaQueryKeys.all, 'stats'] as const,
}
