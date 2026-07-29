import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { pluginApi } from '@/api'
import { pluginQueryKeys } from '@/entities/plugin/api/query-keys'

export function usePluginMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: pluginQueryKeys.all })
  }

  const enablePlugin = useMutation({
    mutationFn: (id: number) => pluginApi.enable(id),
    onSuccess: invalidate,
  })

  const disablePlugin = useMutation({
    mutationFn: (id: number) => pluginApi.disable(id),
    onSuccess: invalidate,
  })

  const updatePluginConfig = useMutation({
    mutationFn: ({ id, config }: { id: number; config: Record<string, unknown> }) =>
      pluginApi.updateConfig(id, config),
    onSuccess: invalidate,
  })

  return { enablePlugin, disablePlugin, updatePluginConfig }
}
