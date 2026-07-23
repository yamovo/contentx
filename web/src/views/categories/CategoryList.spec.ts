import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

// Mock @/api using the shared helper.
vi.mock('@/api', async () => {
  const { mockApi } = await import('@/test/utils')
  return mockApi()
})

// Mock element-plus services while preserving real component exports.
vi.mock('element-plus', async (importOriginal) => {
  const actual = await importOriginal<typeof import('element-plus')>()
  const { mockElementPlus } = await import('@/test/utils')
  return { ...actual, ...mockElementPlus() }
})

// Note: @element-plus/icons-vue is NOT mocked — real SVG icons render in jsdom.

import { mountWithPlugins } from '@/test/utils'
import { categoryApi } from '@/api'
import { ElMessage } from 'element-plus'
import CategoryList from './CategoryList.vue'

const mockCategories = [
  {
    id: 1,
    name: 'Tech',
    slug: 'tech',
    description: 'Tech stuff',
    parent_id: null,
    image: '',
    color: '#409eff',
    sort_order: 0,
    post_count: 4,
    is_active: true,
  },
  {
    id: 2,
    name: 'News',
    slug: 'news',
    description: '',
    parent_id: null,
    image: '',
    color: '#67c23a',
    sort_order: 1,
    post_count: 2,
    is_active: false,
  },
]

describe('CategoryList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(categoryApi.list).mockResolvedValue({ data: mockCategories } as any)
  })

  it('calls categoryApi.list on mount and renders categories', async () => {
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    expect(categoryApi.list).toHaveBeenCalledWith({ all: 'true' })
    expect(wrapper.text()).toContain('Tech')
    expect(wrapper.text()).toContain('News')
  })

  it('shows the disabled tag for inactive categories', async () => {
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    expect(wrapper.text()).toContain('已禁用')
  })

  it('opens the create dialog when the new-category button is clicked', async () => {
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    const createBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('新建分类'))
    expect(createBtn).toBeTruthy()
    await createBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('新建分类')
    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    expect(saveBtn).toBeTruthy()
  })

  it('warns when saving with an empty name', async () => {
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    const createBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('新建分类'))
    await createBtn!.trigger('click')
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(ElMessage.warning).toHaveBeenCalledWith('请输入分类名称')
    expect(categoryApi.create).not.toHaveBeenCalled()
  })

  it('creates a category via categoryApi.create', async () => {
    vi.mocked(categoryApi.create).mockResolvedValueOnce({ data: mockCategories[0] } as any)
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    const createBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('新建分类'))
    await createBtn!.trigger('click')
    await flushPromises()

    const nameInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === '分类名称')
    expect(nameInput).toBeTruthy()
    await nameInput!.setValue('NewCat')
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(categoryApi.create).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'NewCat' }),
    )
    expect(ElMessage.success).toHaveBeenCalledWith('分类已创建')
  })

  it('edits a category via categoryApi.update', async () => {
    vi.mocked(categoryApi.update).mockResolvedValueOnce({ data: mockCategories[0] } as any)
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    const editBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('编辑'))
    expect(editBtn).toBeTruthy()
    await editBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('编辑分类')

    const nameInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === '分类名称')
    await nameInput!.setValue('TechUpdated')
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(categoryApi.update).toHaveBeenCalledWith(
      1,
      expect.objectContaining({ name: 'TechUpdated' }),
    )
    expect(ElMessage.success).toHaveBeenCalledWith('分类已更新')
  })

  it('deletes a category via categoryApi.delete after popconfirm confirm', async () => {
    vi.mocked(categoryApi.delete).mockResolvedValueOnce({} as any)
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    const confirmBtn = wrapper.find('.popconfirm-confirm')
    expect(confirmBtn.exists()).toBe(true)
    await confirmBtn.trigger('click')
    await flushPromises()

    expect(categoryApi.delete).toHaveBeenCalledWith(1)
    expect(ElMessage.success).toHaveBeenCalledWith('分类已删除')
  })

  it('shows ElMessage.error when create rejects', async () => {
    vi.mocked(categoryApi.create).mockRejectedValueOnce(new Error('boom'))
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    const createBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('新建分类'))
    await createBtn!.trigger('click')
    await flushPromises()

    const nameInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === '分类名称')
    await nameInput!.setValue('FailCat')
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(categoryApi.create).toHaveBeenCalled()
    expect(ElMessage.error).toHaveBeenCalledWith('保存失败')
  })

  it('shows ElMessage.error when delete rejects', async () => {
    vi.mocked(categoryApi.delete).mockRejectedValueOnce(new Error('boom'))
    const wrapper = mountWithPlugins(CategoryList)
    await flushPromises()

    const confirmBtn = wrapper.find('.popconfirm-confirm')
    await confirmBtn.trigger('click')
    await flushPromises()

    expect(categoryApi.delete).toHaveBeenCalled()
    expect(ElMessage.error).toHaveBeenCalledWith('删除失败')
  })
})
