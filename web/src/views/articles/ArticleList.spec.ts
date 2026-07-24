import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

vi.mock('@/api', async () => {
  const { mockApi } = await import('@/test/utils')
  return mockApi()
})

vi.mock('element-plus', async (importOriginal) => {
  const actual = await importOriginal<typeof import('element-plus')>()
  const { mockElementPlus } = await import('@/test/utils')
  return { ...actual, ...mockElementPlus() }
})

import { mountWithPlugins } from '@/test/utils'
import { articleApi, categoryApi } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import ArticleList from './ArticleList.vue'

const mockArticles = [
  {
    id: 1,
    title: 'First Post',
    slug: 'first-post',
    status: 'published',
    view_count: 10,
    comment_count: 2,
    author: { display_name: 'Alice' },
    category: { name: 'Tech' },
    tags: [{ id: 1, name: 'Vue' }],
    is_pinned: false,
    is_featured: false,
    created_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 2,
    title: 'Second Post',
    slug: 'second-post',
    status: 'draft',
    view_count: 5,
    comment_count: 0,
    author: { display_name: 'Bob' },
    category: null,
    tags: [],
    is_pinned: true,
    is_featured: false,
    created_at: '2024-01-02T00:00:00Z',
  },
]

const mockCategories = [
  { id: 1, name: 'Tech', slug: 'tech' },
  { id: 2, name: 'Life', slug: 'life' },
]

describe('ArticleList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(articleApi.list).mockResolvedValue({
      items: mockArticles,
      total: 2,
    } as any)
    vi.mocked(categoryApi.list).mockResolvedValue({ data: mockCategories } as any)
  })

  it('calls articleApi.list and categoryApi.list on mount and renders articles', async () => {
    const wrapper = mountWithPlugins(ArticleList)
    await flushPromises()

    expect(articleApi.list).toHaveBeenCalled()
    expect(categoryApi.list).toHaveBeenCalled()
    expect(wrapper.text()).toContain('First Post')
    expect(wrapper.text()).toContain('Second Post')
    expect(wrapper.text()).toContain('2 篇')
  })

  it('renders empty list when articleApi.list rejects', async () => {
    vi.mocked(articleApi.list).mockRejectedValueOnce(new Error('network'))
    const wrapper = mountWithPlugins(ArticleList)
    await flushPromises()

    expect(wrapper.text()).toContain('0 篇')
  })

  it('triggers bulk publish for selected articles', async () => {
    vi.mocked(articleApi.bulk).mockResolvedValue({} as any)
    const wrapper = mountWithPlugins(ArticleList)
    await flushPromises()

    // Simulate selection: directly invoke the handler via component vm.
    const vm = wrapper.vm as any
    vm.selectedIds = [1, 2]
    await wrapper.vm.$nextTick()

    // Find the bulk publish button and click it.
    const bulkBtn = wrapper.findAll('button').find((b) => b.text().includes('发布'))
    expect(bulkBtn).toBeTruthy()
    await bulkBtn!.trigger('click')
    await flushPromises()

    expect(articleApi.bulk).toHaveBeenCalledWith({ article_ids: [1, 2], action: 'publish' })
    expect(ElMessage.success).toHaveBeenCalledWith('操作成功')
  })

  it('shows error message when bulk action fails', async () => {
    vi.mocked(articleApi.bulk).mockRejectedValueOnce(new Error('fail'))
    const wrapper = mountWithPlugins(ArticleList)
    await flushPromises()

    const vm = wrapper.vm as any
    vm.selectedIds = [1]
    await wrapper.vm.$nextTick()

    const bulkBtn = wrapper.findAll('button').find((b) => b.text().includes('转为草稿'))
    await bulkBtn!.trigger('click')
    await flushPromises()

    expect(ElMessage.error).toHaveBeenCalledWith('操作失败')
  })

  it('publishes an article via handleCommand', async () => {
    vi.mocked(articleApi.update).mockResolvedValue({} as any)
    const wrapper = mountWithPlugins(ArticleList)
    await flushPromises()

    const vm = wrapper.vm as any
    await vm.handleCommand('publish', mockArticles[1])
    await flushPromises()

    expect(articleApi.update).toHaveBeenCalledWith(2, { status: 'published' } as any)
    expect(ElMessage.success).toHaveBeenCalledWith('已发布')
  })

  it('toggles pin via handleCommand', async () => {
    vi.mocked(articleApi.update).mockResolvedValue({} as any)
    const wrapper = mountWithPlugins(ArticleList)
    await flushPromises()

    const vm = wrapper.vm as any
    await vm.handleCommand('pin', mockArticles[0])
    await flushPromises()

    expect(articleApi.update).toHaveBeenCalledWith(1, { is_pinned: true } as any)
  })

  it('deletes an article via handleCommand after confirm', async () => {
    vi.mocked(ElMessageBox.confirm).mockResolvedValueOnce('confirm' as any)
    vi.mocked(articleApi.delete).mockResolvedValue({} as any)
    const wrapper = mountWithPlugins(ArticleList)
    await flushPromises()

    const vm = wrapper.vm as any
    await vm.handleCommand('delete', mockArticles[0])
    await flushPromises()

    expect(articleApi.delete).toHaveBeenCalledWith(1)
    expect(ElMessage.success).toHaveBeenCalledWith('已删除')
  })

  it('formats status labels correctly', () => {
    const wrapper = mountWithPlugins(ArticleList)
    const vm = wrapper.vm as any
    expect(vm.statusLabel('published')).toBe('已发布')
    expect(vm.statusLabel('draft')).toBe('草稿')
    expect(vm.statusLabel('pending')).toBe('待审')
    expect(vm.statusLabel('trash')).toBe('回收站')
    expect(vm.statusLabel('unknown')).toBe('unknown')
  })

  it('returns correct status type', () => {
    const wrapper = mountWithPlugins(ArticleList)
    const vm = wrapper.vm as any
    expect(vm.statusType('published')).toBe('success')
    expect(vm.statusType('draft')).toBe('info')
    expect(vm.statusType('pending')).toBe('warning')
    expect(vm.statusType('trash')).toBe('danger')
  })
})
