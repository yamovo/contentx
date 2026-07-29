import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { roleApi, type Role } from '@/api'
import { roleQueryKeys } from '@/entities/role/api/query-keys'

export function useRoleMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: roleQueryKeys.all })
  }

  const createRole = useMutation({
    mutationFn: (data: Partial<Role>) => roleApi.create(data),
    onSuccess: invalidate,
  })

  const updateRole = useMutation({
    mutationFn: ({ id, ...data }: Partial<Role> & { id: number }) =>
      roleApi.update(id, data),
    onSuccess: invalidate,
  })

  const deleteRole = useMutation({
    mutationFn: (id: number) => roleApi.delete(id),
    onSuccess: invalidate,
  })

  return { createRole, updateRole, deleteRole }
}
