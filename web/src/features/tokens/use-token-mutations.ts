import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { tokenApi } from '@/api'
import { tokenQueryKeys } from '@/entities/token/api/query-keys'

export function useTokenMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: tokenQueryKeys.all })
  }

  const createToken = useMutation({
    mutationFn: (data: { name: string; permissions?: string[]; expires_at?: string }) =>
      tokenApi.create(data),
    onSuccess: invalidate,
  })

  const revokeToken = useMutation({
    mutationFn: (id: number) => tokenApi.delete(id),
    onSuccess: invalidate,
  })

  return { createToken, revokeToken }
}
