import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { authApi, type User } from '@/api'
import { profileQueryKeys } from './use-profile-query'
import { useAuthStore } from '@/stores/auth'

/**
 * Mutations for profile updates and password changes.
 * After a successful profile update the authStore user is patched so the
 * header dropdown stays in sync, and the profile query cache is invalidated.
 */
export function useProfileMutation() {
  const queryClient = useQueryClient()
  const authStore = useAuthStore()

  const updateProfile = useMutation({
    mutationFn: (data: Partial<User>) => authApi.updateProfile(data),
    onSuccess: (res) => {
      // Keep authStore in sync so the header dropdown reflects changes.
      if (authStore.user) {
        authStore.user = { ...authStore.user, ...res.data }
      }
      queryClient.invalidateQueries({ queryKey: profileQueryKeys.all })
    },
  })

  const changePassword = useMutation({
    mutationFn: (data: { old_password: string; new_password: string }) =>
      authApi.changePassword(data),
  })

  return { updateProfile, changePassword }
}
