export const articleQueryKeys = {
  all: ['articles'] as const,
  lists: () => [...articleQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) =>
    [...articleQueryKeys.lists(), params] as const,
  detail: (id: number) => [...articleQueryKeys.all, 'detail', id] as const,
  revisions: (id: number) => [...articleQueryKeys.detail(id), 'revisions'] as const,
}
