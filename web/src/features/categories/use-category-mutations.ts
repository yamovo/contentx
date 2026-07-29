import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { categoryApi, type Category } from '@/api'
import { categoryQueryKeys } from '@/entities/category/api/query-keys'

export function useCategoryMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: categoryQueryKeys.all })
  }

  const createCategory = useMutation({
    mutationFn: (data: Partial<Category>) => categoryApi.create(data),
    onSuccess: invalidate,
  })

  const updateCategory = useMutation({
    mutationFn: ({ id, ...data }: Partial<Category> & { id: number }) =>
      categoryApi.update(id, data),
    onSuccess: invalidate,
  })

  const deleteCategory = useMutation({
    mutationFn: (id: number) => categoryApi.delete(id),
    onSuccess: invalidate,
  })

  const reorderCategories = useMutation({
    mutationFn: (items: { id: number; sort_order: number; parent_id?: number }[]) =>
      categoryApi.reorder(items),
    onSuccess: invalidate,
  })

  return { createCategory, updateCategory, deleteCategory, reorderCategories }
}
