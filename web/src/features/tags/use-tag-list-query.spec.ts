import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createApp, defineComponent } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api', () => ({
  tagApi: { list: vi.fn().mockResolvedValue({ data: [] }) },
}))

import { useTagListQuery } from './use-tag-list-query'
import { tagApi } from '@/api'

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

describe('useTagListQuery', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('fetches tags', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ data }] = withSetup(() => useTagListQuery(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await vi.waitFor(() => expect(data.value).toBeDefined())
    expect(tagApi.list).toHaveBeenCalled()
  })
})
