import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { buildTree } from '@/utils'

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
import { articleApi, tagApi, categoryApi } from '@/api'
import { ElMessage } from 'element-plus'
import ArticleEditor from './ArticleEditor.vue'

const mockTags = [
  { id: 1, name: 'Vue', slug: 'vue', count: 0, color: '' },
  { id: 2, name: 'TS', slug: 'ts', count: 0, color: '' },
]

const mockCategories = [
  { id: 1, name: 'Tech', slug: 'tech', parent_id: null },
  { id: 2, name: 'Life', slug: 'life', parent_id: null },
]

describe('ArticleEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(tagApi.list).mockResolvedValue({ data: mockTags } as any)
    vi.mocked(categoryApi.list).mockResolvedValue({ data: mockCategories } as any)
  })

  function mountEditor() {
    // mountWithPlugins installs a memory router with a single `/` route; in
    // create mode `route.params.id` is undefined so `isEdit` is false. The
    // component reads `route.meta.postType` which is undefined here and
    // defaults to 'post' via `|| 'post'`.
    return mountWithPlugins(ArticleEditor)
  }

  it('loads tags and categories on mount in create mode', async () => {
    const wrapper = mountEditor()
    await flushPromises()

    expect(tagApi.list).toHaveBeenCalled()
    expect(categoryApi.list).toHaveBeenCalled()
    // Edit mode path is not used — isEdit computed is false, so articleApi.get is NOT called.
    expect(articleApi.get).not.toHaveBeenCalled()
    // Default status is draft and form title is empty.
    const vm = wrapper.vm as any
    expect(vm.form.status).toBe('draft')
    expect(vm.form.title).toBe('')
  })

  it('autoSlug derives slug from title in create mode', async () => {
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.title = 'Hello World 2026'
    vm.autoSlug()
    expect(vm.form.slug).toBe('hello-world-2026')
  })

  it('autoSlug keeps CJK characters and replaces spaces with dashes', async () => {
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.title = '你好 World!'
    vm.autoSlug()
    expect(vm.form.slug).toBe('你好-world')
  })

  it('save() warns when title is empty and does not call articleApi', async () => {
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    await vm.save()
    await flushPromises()

    expect(ElMessage.warning).toHaveBeenCalledWith('请输入文章标题')
    expect(articleApi.create).not.toHaveBeenCalled()
    expect(articleApi.update).not.toHaveBeenCalled()
  })

  it('save() creates an article and navigates to the edit route', async () => {
    vi.mocked(articleApi.create).mockResolvedValueOnce({
      data: { id: 42, title: 'T', slug: 't' } as any,
    } as any)
    const wrapper = mountEditor()
    await flushPromises()

    // The component uses useRouter() (composition API), which returns the
    // actual installed router — not a global mock. Spy on its replace method.
    const replaceSpy = vi.spyOn(wrapper.vm.$router, 'replace')

    const vm = wrapper.vm as any
    vm.form.title = 'New Title'
    vm.form.content = 'body'
    await vm.save()
    await flushPromises()

    expect(articleApi.create).toHaveBeenCalledWith(expect.objectContaining({ title: 'New Title', content: 'body' }))
    expect(ElMessage.success).toHaveBeenCalledWith('文章已保存')
    expect(replaceSpy).toHaveBeenCalledWith('/admin/articles/42/edit')
  })

  it('save() shows ElMessage.error when articleApi.create rejects with response.error', async () => {
    const err: any = new Error('boom')
    err.response = { data: { error: '标题重复' } }
    vi.mocked(articleApi.create).mockRejectedValueOnce(err)

    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.title = 'Whatever'
    await vm.save()
    await flushPromises()

    expect(ElMessage.error).toHaveBeenCalledWith('标题重复')
  })

  it('save() falls back to generic error when response is missing', async () => {
    vi.mocked(articleApi.create).mockRejectedValueOnce(new Error('network'))
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.title = 'Whatever'
    await vm.save()
    await flushPromises()

    expect(ElMessage.error).toHaveBeenCalledWith('保存失败')
  })

  it('saveDraft() sets status to draft and calls save', async () => {
    vi.mocked(articleApi.create).mockResolvedValueOnce({
      data: { id: 7, title: 'd', slug: 'd' } as any,
    } as any)
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.title = 'Draft'
    await vm.saveDraft()
    await flushPromises()

    expect(vm.form.status).toBe('draft')
    expect(articleApi.create).toHaveBeenCalled()
  })

  it('publish() promotes draft status to published before saving', async () => {
    vi.mocked(articleApi.create).mockResolvedValueOnce({
      data: { id: 8, title: 'p', slug: 'p' } as any,
    } as any)
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.title = 'Pub'
    await vm.publish()
    await flushPromises()

    expect(vm.form.status).toBe('published')
    expect(articleApi.create).toHaveBeenCalled()
  })

  it('publish() does not downgrade an already-published article', async () => {
    vi.mocked(articleApi.create).mockResolvedValueOnce({
      data: { id: 9, title: 'p2', slug: 'p2' } as any,
    } as any)
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.title = 'Pub'
    vm.form.status = 'published'
    await vm.publish()
    await flushPromises()

    expect(vm.form.status).toBe('published')
  })

  it('createTag() calls tagApi.create, appends to allTags and selects it', async () => {
    vi.mocked(tagApi.create).mockResolvedValueOnce({
      data: { id: 5, name: 'NewTag', slug: 'newtag', count: 0, color: '' } as any,
    } as any)
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    await vm.createTag('NewTag')
    await flushPromises()

    expect(tagApi.create).toHaveBeenCalledWith({ name: 'NewTag' })
    expect(vm.allTags.some((t: any) => t.id === 5)).toBe(true)
    expect(vm.form.tag_ids).toContain(5)
  })

  it('createTag() shows ElMessage.error when tagApi.create rejects', async () => {
    vi.mocked(tagApi.create).mockRejectedValueOnce(new Error('boom'))
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    await vm.createTag('Fail')
    await flushPromises()

    expect(ElMessage.error).toHaveBeenCalledWith('创建标签失败')
  })

  it('handleImageUpload() sets form.featured_image from the response url', async () => {
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.handleImageUpload({ data: { url: '/uploads/x.png' } })
    expect(vm.form.featured_image).toBe('/uploads/x.png')
  })

  it('renderedContent renders markdown bold via marked', async () => {
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.content = '**hello**'
    await wrapper.vm.$nextTick()
    expect(vm.renderedContent).toContain('<strong>hello</strong>')
  })

  it('renderedContent strips <script> tags via DOMPurify', async () => {
    const wrapper = mountEditor()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.form.content = '<script>alert(1)</script>'
    await wrapper.vm.$nextTick()
    expect(vm.renderedContent).not.toContain('<script')
  })

  it('buildTree builds a nested tree from a flat category list', () => {
    const flat = [
      { id: 1, name: 'A', parent_id: null },
      { id: 2, name: 'B', parent_id: 1 },
      { id: 3, name: 'C', parent_id: 1 },
      { id: 4, name: 'D', parent_id: null },
    ]
    const tree = buildTree(flat)
    expect(tree).toHaveLength(2)
    expect(tree[0].children).toHaveLength(2)
    expect(tree[1].children).toHaveLength(0)
  })
})
