import { useQuery } from '@tanstack/vue-query'
import { webhookApi } from '@/api'
import { webhookQueryKeys } from '@/entities/webhook/api/query-keys'

export function useWebhookListQuery() {
  return useQuery({
    queryKey: webhookQueryKeys.lists(),
    queryFn: () => webhookApi.list(),
    placeholderData: (previous) => previous,
  })
}

export function useWebhookLogsQuery(webhookId: number) {
  return useQuery({
    queryKey: webhookQueryKeys.logs(webhookId),
    queryFn: () => webhookApi.logs(webhookId),
    enabled: !!webhookId,
  })
}
