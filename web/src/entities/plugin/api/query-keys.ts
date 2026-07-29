export const pluginQueryKeys = {
  all: ['plugins'] as const,
  lists: () => [...pluginQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...pluginQueryKeys.lists(), params] as const,
}
