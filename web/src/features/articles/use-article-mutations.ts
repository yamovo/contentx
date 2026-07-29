import { useMutation, useQueryClient } from '@tanstack/vue-query'
import {
  articleApi,
  type ArticleCreateInput,
  type ArticleUpdateInput,
} from '@/api'
import { articleQueryKeys } from '@/entities/article/api/query-keys'

export function useArticleMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: articleQueryKeys.all })
  }

  const createArticle = useMutation({
    mutationFn: (data: ArticleCreateInput) => articleApi.create(data),
    onSuccess: invalidate,
  })

  const updateArticle = useMutation({
    mutationFn: ({ id, ...data }: ArticleUpdateInput & { id: number }) =>
      articleApi.update(id, data),
    onSuccess: invalidate,
  })

  const deleteArticle = useMutation({
    mutationFn: (id: number) => articleApi.delete(id),
    onSuccess: invalidate,
  })

  const bulkAction = useMutation({
    mutationFn: (data: {
      article_ids: number[]
      action: 'publish' | 'unpublish' | 'draft' | 'trash' | 'delete'
      category_id?: number
    }) => articleApi.bulk(data),
    onSuccess: invalidate,
  })

  const publishArticle = useMutation({
    mutationFn: (id: number) => articleApi.publish(id),
    onSuccess: invalidate,
  })

  const unpublishArticle = useMutation({
    mutationFn: (id: number) => articleApi.unpublish(id),
    onSuccess: invalidate,
  })

  const submitReview = useMutation({
    mutationFn: (id: number) => articleApi.submitReview(id),
    onSuccess: invalidate,
  })

  const approveArticle = useMutation({
    mutationFn: (id: number) => articleApi.approve(id),
    onSuccess: invalidate,
  })

  const scheduleArticle = useMutation({
    mutationFn: ({ id, scheduled_at }: { id: number; scheduled_at: string }) =>
      articleApi.schedule(id, scheduled_at),
    onSuccess: invalidate,
  })

  const archiveArticle = useMutation({
    mutationFn: (id: number) => articleApi.archive(id),
    onSuccess: invalidate,
  })

  const restoreRevision = useMutation({
    mutationFn: ({ articleId, revisionId }: { articleId: number; revisionId: number }) =>
      articleApi.restoreRevision(articleId, revisionId),
    onSuccess: invalidate,
  })

  return {
    createArticle,
    updateArticle,
    deleteArticle,
    bulkAction,
    publishArticle,
    unpublishArticle,
    submitReview,
    approveArticle,
    scheduleArticle,
    archiveArticle,
    restoreRevision,
  }
}
