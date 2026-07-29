import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { commentApi, type Comment } from '@/api'
import { commentQueryKeys } from '@/entities/comment/api/query-keys'

export function useCommentMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: commentQueryKeys.all })
  }

  const createComment = useMutation({
    mutationFn: (data: Partial<Comment>) => commentApi.create(data),
    onSuccess: invalidate,
  })

  const updateComment = useMutation({
    mutationFn: ({ id, ...data }: Partial<Comment> & { id: number }) =>
      commentApi.update(id, data),
    onSuccess: invalidate,
  })

  const approveComment = useMutation({
    mutationFn: (id: number) => commentApi.approve(id),
    onSuccess: invalidate,
  })

  const spamComment = useMutation({
    mutationFn: (id: number) => commentApi.spam(id),
    onSuccess: invalidate,
  })

  const trashComment = useMutation({
    mutationFn: (id: number) => commentApi.trash(id),
    onSuccess: invalidate,
  })

  const bulkAction = useMutation({
    mutationFn: (data: { comment_ids: number[]; action: string }) =>
      commentApi.bulk(data),
    onSuccess: invalidate,
  })

  return { createComment, updateComment, approveComment, spamComment, trashComment, bulkAction }
}
