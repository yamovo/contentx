import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createApp, defineComponent } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api', () => ({
  tagApi: {
    create: vi.fn().mockResolvedValue({}),
    update: vi.fn().mockResolvedValue({}),
    delete: vi.fn().mockResolvedValue({}),
    merge: vi.fn().mockResolvedValue({}),
  },
}))

import { useTagMutations } from './use-tag-mutations'
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

describe('useTagMutations', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('creates a tag', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ createTag }] = withSetup(() => useTagMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await createTag.mutateAsync({ name: 'Test' })
    expect(tagApi.create).toHaveBeenCalledWith({ name: 'Test' })
  })

  it('updates a tag', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ updateTag }] = withSetup(() => useTagMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await updateTag.mutateAsync({ id: 1, name: 'Updated' })
    expect(tagApi.update).toHaveBeenCalledWith(1, { name: 'Updated' })
  })

  it('deletes a tag', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ deleteTag }] = withSetup(() => useTagMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await deleteTag.mutateAsync(1)
    expect(tagApi.delete).toHaveBeenCalledWith(1)
  })

  it('merges tags', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ mergeTags }] = withSetup(() => useTagMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    const data = { source_ids: [1, 2], target_id: 3, delete_old: true }
    await mergeTags.mutateAsync(data)
    expect(tagApi.merge).toHaveBeenCalledWith(data)
  })
})
