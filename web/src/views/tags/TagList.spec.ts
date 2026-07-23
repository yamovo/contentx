import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

// Mock @/api using the shared helper. vi.mock factories are hoisted, so we
// use a dynamic import to access the helper (vitest supports async factories).
vi.mock('@/api', async () => {
  const { mockApi } = await import('@/test/utils')
  return mockApi()
})

// Mock element-plus services while preserving real component exports (the
// view auto-imports ElInput, ElIcon, etc. from element-plus, so a bare
// service-only mock would break those imports). importOriginal keeps the
// real components available; @vue/test-utils stubs handle the rendering.
vi.mock('element-plus', async (importOriginal) => {
  const actual = await importOriginal<typeof import('element-plus')>()
  const { mockElementPlus } = await import('@/test/utils')
  return { ...actual, ...mockElementPlus() }
})

// Note: @element-plus/icons-vue is NOT mocked — the real SVG icon components
// render fine in jsdom, and the real ElInput (preserved via importOriginal
// above) imports icons like Close that we'd otherwise have to enumerate.

import { mountWithPlugins } from '@/test/utils'
import { tagApi } from '@/api'
import { ElMessage } from 'element-plus'
import TagList from './TagList.vue'

const mockTags = [
  { id: 1, name: 'Vue', slug: 'vue', count: 5, color: '#42b883' },
  { id: 2, name: 'TypeScript', slug: 'ts', count: 3, color: '#3178c6' },
]

describe('TagList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(tagApi.list).mockResolvedValue({ data: mockTags } as any)
  })

  it('calls tagApi.list on mount and renders the tags', async () => {
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    expect(tagApi.list).toHaveBeenCalledWith({ search: '' })
    expect(wrapper.text()).toContain('Vue')
    expect(wrapper.text()).toContain('TypeScript')
  })

  it('opens the create dialog when the new-tag button is clicked', async () => {
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    const createBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('新建标签'))
    expect(createBtn).toBeTruthy()
    await createBtn!.trigger('click')
    await flushPromises()

    // The dialog footer has a 保存 button.
    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    expect(saveBtn).toBeTruthy()
  })

  it('creates a tag via tagApi.create when saving a new tag', async () => {
    vi.mocked(tagApi.create).mockResolvedValueOnce({ data: mockTags[0] } as any)
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    // Open create dialog.
    const createBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('新建标签'))
    await createBtn!.trigger('click')
    await flushPromises()

    // Fill the name input — distinguishable by placeholder.
    const nameInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === '标签名称')
    expect(nameInput).toBeTruthy()
    await nameInput!.setValue('NewTag')
    await flushPromises()

    // Click the 保存 button (footer).
    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(tagApi.create).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'NewTag' }),
    )
    expect(ElMessage.success).toHaveBeenCalledWith('标签已创建')
  })

  it('warns when saving with an empty name', async () => {
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    const createBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('新建标签'))
    await createBtn!.trigger('click')
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(ElMessage.warning).toHaveBeenCalledWith('请输入标签名称')
    expect(tagApi.create).not.toHaveBeenCalled()
  })

  it('edits an existing tag via tagApi.update', async () => {
    vi.mocked(tagApi.update).mockResolvedValueOnce({ data: mockTags[0] } as any)
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    // Click the first 编辑 button.
    const editBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('编辑'))
    expect(editBtn).toBeTruthy()
    await editBtn!.trigger('click')
    await flushPromises()

    // Dialog title should be 编辑标签.
    expect(wrapper.text()).toContain('编辑标签')

    // Update the name.
    const nameInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === '标签名称')
    await nameInput!.setValue('VueUpdated')
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(tagApi.update).toHaveBeenCalledWith(
      1,
      expect.objectContaining({ name: 'VueUpdated' }),
    )
    expect(ElMessage.success).toHaveBeenCalledWith('标签已更新')
  })

  it('deletes a tag via tagApi.delete after popconfirm confirm', async () => {
    vi.mocked(tagApi.delete).mockResolvedValueOnce({} as any)
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    // The el-popconfirm stub renders a .popconfirm-confirm button.
    const confirmBtn = wrapper.find('.popconfirm-confirm')
    expect(confirmBtn.exists()).toBe(true)
    await confirmBtn.trigger('click')
    await flushPromises()

    expect(tagApi.delete).toHaveBeenCalledWith(1)
    expect(ElMessage.success).toHaveBeenCalledWith('标签已删除')
  })

  it('shows ElMessage.error when create rejects', async () => {
    vi.mocked(tagApi.create).mockRejectedValueOnce(new Error('boom'))
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    const createBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('新建标签'))
    await createBtn!.trigger('click')
    await flushPromises()

    const nameInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === '标签名称')
    await nameInput!.setValue('FailTag')
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('保存'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(tagApi.create).toHaveBeenCalled()
    expect(ElMessage.error).toHaveBeenCalledWith('保存失败')
  })

  it('shows ElMessage.error when delete rejects', async () => {
    vi.mocked(tagApi.delete).mockRejectedValueOnce(new Error('boom'))
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    const confirmBtn = wrapper.find('.popconfirm-confirm')
    await confirmBtn.trigger('click')
    await flushPromises()

    expect(tagApi.delete).toHaveBeenCalled()
    expect(ElMessage.error).toHaveBeenCalledWith('删除失败')
  })

  it('renders empty state when list returns no tags', async () => {
    vi.mocked(tagApi.list).mockResolvedValueOnce({ data: [] } as any)
    const wrapper = mountWithPlugins(TagList)
    await flushPromises()

    expect(wrapper.text()).toContain('暂无标签')
  })
})
