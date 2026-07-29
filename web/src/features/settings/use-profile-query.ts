import { useQuery } from '@tanstack/vue-query'
import { authApi } from '@/api'

export const profileQueryKeys = {
  all: ['auth', 'profile'] as const,
}

/**
 * Fetches the current user profile via authApi.me().
 * The authStore remains the source of truth for session-level auth state;
 * this query is specifically for the ProfileView data-loading pattern.
 */
export function useProfileQuery() {
  return useQuery({
    queryKey: profileQueryKeys.all,
    queryFn: async () => {
      const res = await authApi.me()
      return res.data
    },
    staleTime: 5 * 60 * 1000, // 5 min – profile rarely changes
  })
}
