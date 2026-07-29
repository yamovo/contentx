import { useQuery } from '@tanstack/vue-query'
import { seoApi } from '@/api'
import { seoQueryKeys } from '@/entities/seo/api/query-keys'

export function useSeoRedirectsQuery() {
  return useQuery({
    queryKey: seoQueryKeys.redirects(),
    queryFn: () => seoApi.listRedirects(),
    placeholderData: (previous) => previous,
  })
}
