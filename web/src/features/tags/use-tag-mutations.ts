import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { tagApi, type Tag } from '@/api'
import { tagQueryKeys } from '@/entities/tag/api/query-keys'

export function useTagMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: tagQueryKeys.all })
  }

  const createTag = useMutation({
    mutationFn: (data: Partial<Tag>) => tagApi.create(data),
    onSuccess: invalidate,
  })

  const updateTag = useMutation({
    mutationFn: ({ id, ...data }: Partial<Tag> & { id: number }) =>
      tagApi.update(id, data),
    onSuccess: invalidate,
  })

  const deleteTag = useMutation({
    mutationFn: (id: number) => tagApi.delete(id),
    onSuccess: invalidate,
  })

  const mergeTags = useMutation({
    mutationFn: (data: { source_ids: number[]; target_id: number; delete_old: boolean }) =>
      tagApi.merge(data),
    onSuccess: invalidate,
  })

  return { createTag, updateTag, deleteTag, mergeTags }
}
