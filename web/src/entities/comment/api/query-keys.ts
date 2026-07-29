export const commentQueryKeys = {
  all: ['comments'] as const,
  lists: () => [...commentQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...commentQueryKeys.lists(), params] as const,
  detail: (id: number) => [...commentQueryKeys.all, 'detail', id] as const,
  stats: () => [...commentQueryKeys.all, 'stats'] as const,
  articleComments: (articleId: number) => [...commentQueryKeys.all, 'article', articleId] as const,
}
