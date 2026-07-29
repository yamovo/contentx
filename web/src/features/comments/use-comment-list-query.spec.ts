import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createApp, defineComponent } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api', () => ({
  commentApi: {
    list: vi.fn().mockResolvedValue({ data: [] }),
    stats: vi.fn().mockResolvedValue({}),
  },
}))

import { useCommentListQuery, useCommentStatsQuery } from './use-comment-list-query'
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

describe('useCommentListQuery', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('fetches comments', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ data }] = withSetup(() => useCommentListQuery(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await vi.waitFor(() => expect(data.value).toBeDefined())
    expect(commentApi.list).toHaveBeenCalled()
  })
})

describe('useCommentStatsQuery', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('fetches comment stats', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ data }] = withSetup(() => useCommentStatsQuery(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await vi.waitFor(() => expect(data.value).toBeDefined())
    expect(commentApi.stats).toHaveBeenCalled()
  })
})
