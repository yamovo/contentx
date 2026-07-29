export const webhookQueryKeys = {
  all: ['webhooks'] as const,
  lists: () => [...webhookQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...webhookQueryKeys.lists(), params] as const,
  detail: (id: number) => [...webhookQueryKeys.all, 'detail', id] as const,
  logs: (id: number) => [...webhookQueryKeys.detail(id), 'logs'] as const,
}
