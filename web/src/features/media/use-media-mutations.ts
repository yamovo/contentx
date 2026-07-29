import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { mediaApi, type Media } from '@/api'
import { mediaQueryKeys } from '@/entities/media/api/query-keys'

export function useMediaMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: mediaQueryKeys.all })
  }

  const uploadMedia = useMutation({
    mutationFn: (formData: FormData) => mediaApi.upload(formData),
    onSuccess: invalidate,
  })

  const updateMedia = useMutation({
    mutationFn: ({ id, ...data }: Partial<Media> & { id: number }) =>
      mediaApi.update(id, data),
    onSuccess: invalidate,
  })

  const deleteMedia = useMutation({
    mutationFn: (id: number) => mediaApi.delete(id),
    onSuccess: invalidate,
  })

  const bulkDeleteMedia = useMutation({
    mutationFn: (ids: number[]) => mediaApi.bulkDelete(ids),
    onSuccess: invalidate,
  })

  return { uploadMedia, updateMedia, deleteMedia, bulkDeleteMedia }
}
