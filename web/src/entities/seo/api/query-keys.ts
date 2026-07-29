export const seoQueryKeys = {
  all: ['seo'] as const,
  lists: () => [...seoQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...seoQueryKeys.lists(), params] as const,
  detail: (id: number) => [...seoQueryKeys.all, 'detail', id] as const,
  redirects: () => [...seoQueryKeys.all, 'redirects'] as const,
  sitemap: () => [...seoQueryKeys.all, 'sitemap'] as const,
  robotsTxt: () => [...seoQueryKeys.all, 'robotsTxt'] as const,
}
