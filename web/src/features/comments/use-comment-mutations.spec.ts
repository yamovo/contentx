import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createApp, defineComponent } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api', () => ({
  commentApi: {
    create: vi.fn().mockResolvedValue({}),
    update: vi.fn().mockResolvedValue({}),
    approve: vi.fn().mockResolvedValue({}),
    spam: vi.fn().mockResolvedValue({}),
    trash: vi.fn().mockResolvedValue({}),
    bulk: vi.fn().mockResolvedValue({}),
  },
}))

import { useCommentMutations } from './use-comment-mutations'
import { commentApi } from '@/api'

function withSetup<T>(composable: () => T, opts?: { plugins?: any[] }): [T, any] {
  let result!: T
  const app = createApp(defineComponent({ setup() { result = composable(); return () => {} } }))
  opts?.plugins?.forEach(p => {
    if (Array.isArray(p)) app.use(p[0], p[1])
    else app.use(p)
  })
  app.mount(document.createElement('div'))
  return [result, app]
}

describe('useCommentMutations', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('approves a comment', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ approveComment }] = withSetup(() => useCommentMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await approveComment.mutateAsync(1)
    expect(commentApi.approve).toHaveBeenCalledWith(1)
  })

  it('marks comment as spam', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ spamComment }] = withSetup(() => useCommentMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await spamComment.mutateAsync(1)
    expect(commentApi.spam).toHaveBeenCalledWith(1)
  })

  it('trashes a comment', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ trashComment }] = withSetup(() => useCommentMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await trashComment.mutateAsync(1)
    expect(commentApi.trash).toHaveBeenCalledWith(1)
  })

  it('performs bulk action', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ bulkAction }] = withSetup(() => useCommentMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    const data = { comment_ids: [1, 2], action: 'approve' }
    await bulkAction.mutateAsync(data)
    expect(commentApi.bulk).toHaveBeenCalledWith(data)
  })
})
