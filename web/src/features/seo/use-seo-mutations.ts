import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { seoApi, type Redirect } from '@/api'
import { seoQueryKeys } from '@/entities/seo/api/query-keys'

export function useSeoMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: seoQueryKeys.all })
  }

  const createRedirect = useMutation({
    mutationFn: (data: Partial<Redirect>) => seoApi.createRedirect(data),
    onSuccess: invalidate,
  })

  const deleteRedirect = useMutation({
    mutationFn: (id: number) => seoApi.deleteRedirect(id),
    onSuccess: invalidate,
  })

  return { createRedirect, deleteRedirect }
}
