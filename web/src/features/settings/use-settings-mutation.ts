import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { settingsApi } from '@/api'
import { settingsQueryKeys } from '@/entities/settings/api/query-keys'

export function useSettingsMutation() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: settingsQueryKeys.all })
  }

  const updateSettings = useMutation({
    mutationFn: (data: Record<string, unknown>) => settingsApi.update(data),
    onSuccess: invalidate,
  })

  return { updateSettings }
}
