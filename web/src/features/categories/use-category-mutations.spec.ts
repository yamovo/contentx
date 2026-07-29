import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createApp, defineComponent } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api', () => ({
  categoryApi: {
    create: vi.fn().mockResolvedValue({}),
    update: vi.fn().mockResolvedValue({}),
    delete: vi.fn().mockResolvedValue({}),
    reorder: vi.fn().mockResolvedValue({}),
  },
}))

import { useCategoryMutations } from './use-category-mutations'
import { categoryApi } from '@/api'

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

describe('useCategoryMutations', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('creates a category', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ createCategory }] = withSetup(() => useCategoryMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await createCategory.mutateAsync({ name: 'Test' })
    expect(categoryApi.create).toHaveBeenCalledWith({ name: 'Test' })
  })

  it('updates a category', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ updateCategory }] = withSetup(() => useCategoryMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await updateCategory.mutateAsync({ id: 1, name: 'Updated' })
    expect(categoryApi.update).toHaveBeenCalledWith(1, { name: 'Updated' })
  })

  it('deletes a category', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ deleteCategory }] = withSetup(() => useCategoryMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    await deleteCategory.mutateAsync(1)
    expect(categoryApi.delete).toHaveBeenCalledWith(1)
  })

  it('reorders categories', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const [{ reorderCategories }] = withSetup(() => useCategoryMutations(), {
      plugins: [[VueQueryPlugin, { queryClient: qc }]],
    })
    const items = [{ id: 1, sort_order: 1 }]
    await reorderCategories.mutateAsync(items)
    expect(categoryApi.reorder).toHaveBeenCalledWith(items)
  })
})
