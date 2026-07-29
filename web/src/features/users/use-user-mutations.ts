import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { userApi, type User } from '@/api'
import { userQueryKeys } from '@/entities/user/api/query-keys'

export function useUserMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: userQueryKeys.all })
  }

  const createUser = useMutation({
    mutationFn: (data: Partial<User> & { password: string }) => userApi.create(data),
    onSuccess: invalidate,
  })

  const updateUser = useMutation({
    mutationFn: ({ id, ...data }: Partial<User> & { id: number }) =>
      userApi.update(id, data),
    onSuccess: invalidate,
  })

  const deleteUser = useMutation({
    mutationFn: (id: number) => userApi.delete(id),
    onSuccess: invalidate,
  })

  const resetPassword = useMutation({
    mutationFn: ({ id, new_password }: { id: number; new_password: string }) =>
      userApi.resetPassword(id, new_password),
    onSuccess: invalidate,
  })

  return { createUser, updateUser, deleteUser, resetPassword }
}
