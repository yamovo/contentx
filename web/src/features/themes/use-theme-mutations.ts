import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { themeApi } from '@/api'

const themeQueryKeys = {
  all: ['themes'] as const,
}

export function useThemeMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: themeQueryKeys.all })
  }

  const activateTheme = useMutation({
    mutationFn: (id: number) => themeApi.activate(id),
    onSuccess: invalidate,
  })

  const updateThemeConfig = useMutation({
    mutationFn: ({ id, config }: { id: number; config: Record<string, unknown> }) =>
      themeApi.updateConfig(id, config),
    onSuccess: invalidate,
  })

  return { activateTheme, updateThemeConfig }
}
