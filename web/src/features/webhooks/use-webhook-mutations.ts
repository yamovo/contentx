import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { webhookApi } from '@/api'
import { webhookQueryKeys } from '@/entities/webhook/api/query-keys'

export function useWebhookMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: webhookQueryKeys.all })
  }

  const createWebhook = useMutation({
    mutationFn: (data: { name: string; url: string; events: string[]; headers?: string[]; secret?: string }) =>
      webhookApi.create(data),
    onSuccess: invalidate,
  })

  const deleteWebhook = useMutation({
    mutationFn: (id: number) => webhookApi.delete(id),
    onSuccess: invalidate,
  })

  return { createWebhook, deleteWebhook }
}
