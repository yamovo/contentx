export const tokenQueryKeys = {
  all: ['tokens'] as const,
  lists: () => [...tokenQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...tokenQueryKeys.lists(), params] as const,
}
