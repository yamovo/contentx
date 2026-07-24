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
import { mediaApi } from '@/api'
import { ElMessage } from 'element-plus'
import MediaLibrary from './MediaLibrary.vue'

const mockMedia = [
  {
    id: 1,
    filename: 'pic.jpg',
    original_name: 'pic.jpg',
    url: '/uploads/pic.jpg',
    thumbnail_url: '/uploads/pic.jpg',
    mime_type: 'image/jpeg',
    file_size: 2048,
    alt: '',
    title: '',
    caption: '',
    folder: 'default',
    created_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 2,
    filename: 'doc.pdf',
    original_name: 'doc.pdf',
    url: '/uploads/doc.pdf',
    thumbnail_url: '',
    mime_type: 'application/pdf',
    file_size: 5242880,
    alt: '',
    title: '',
    caption: '',
    folder: 'docs',
    created_at: '2024-01-02T00:00:00Z',
  },
]

const mockFolders = ['default', 'docs']

const mockStats = {
  total_files: 10,
  total_size: 10485760,
  images: 7,
  videos: 1,
  documents: 2,
}

describe('MediaLibrary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(mediaApi.list).mockResolvedValue({
      items: mockMedia,
      total: 2,
    } as any)
    vi.mocked(mediaApi.folders).mockResolvedValue({ data: mockFolders } as any)
    vi.mocked(mediaApi.stats).mockResolvedValue({ data: mockStats } as any)

    // jsdom doesn't ship navigator.clipboard — install a stub.
    Object.defineProperty(globalThis.navigator, 'clipboard', {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true,
    })
  })

  it('calls mediaApi.list/folders/stats on mount and renders media items', async () => {
    const wrapper = mountWithPlugins(MediaLibrary)
    await flushPromises()

    expect(mediaApi.list).toHaveBeenCalledWith(
      expect.objectContaining({ page: 1, page_size: 50 }),
    )
    expect(mediaApi.folders).toHaveBeenCalled()
    expect(mediaApi.stats).toHaveBeenCalled()
    expect(wrapper.text()).toContain('pic.jpg')
    expect(wrapper.text()).toContain('doc.pdf')
  })

  it('falls back to empty list when mediaApi.list rejects', async () => {
    vi.mocked(mediaApi.list).mockRejectedValueOnce(new Error('boom'))
    const wrapper = mountWithPlugins(MediaLibrary)
    await flushPromises()

    expect(wrapper.text()).toContain('暂无文件')
  })

  it('opens the detail drawer when a media card is selected', async () => {
    const wrapper = mountWithPlugins(MediaLibrary)
    await flushPromises()

    const vm = wrapper.vm as any
    vm.selectMedia(mockMedia[0])
    await wrapper.vm.$nextTick()

    expect(vm.selectedMedia).toEqual(mockMedia[0])
    expect(vm.detailVisible).toBe(true)
    expect(vm.editForm.filename).toBe('pic.jpg')
  })

  it('updates media via mediaApi.update and shows success', async () => {
    vi.mocked(mediaApi.update).mockResolvedValueOnce({} as any)
    const wrapper = mountWithPlugins(MediaLibrary)
    await flushPromises()

    const vm = wrapper.vm as any
    vm.selectedMedia = mockMedia[0]
    vm.editForm.title = 'New Title'
    await vm.updateMedia()
    await flushPromises()

    expect(mediaApi.update).toHaveBeenCalledWith(1, expect.objectContaining({ title: 'New Title' }))
    expect(ElMessage.success).toHaveBeenCalledWith('已更新')
  })

  it('deletes media via mediaApi.delete, shows success and closes drawer', async () => {
    vi.mocked(mediaApi.delete).mockResolvedValueOnce({} as any)
    const wrapper = mountWithPlugins(MediaLibrary)
    await flushPromises()

    const vm = wrapper.vm as any
    vm.selectedMedia = mockMedia[0]
    vm.detailVisible = true
    await vm.deleteMedia(1)
    await flushPromises()

    expect(mediaApi.delete).toHaveBeenCalledWith(1)
    expect(ElMessage.success).toHaveBeenCalledWith('已删除')
    expect(vm.detailVisible).toBe(false)
  })

  it('copyUrl writes the absolute URL to clipboard and notifies', async () => {
    const wrapper = mountWithPlugins(MediaLibrary)
    await flushPromises()

    const vm = wrapper.vm as any
    await vm.copyUrl(mockMedia[0])

    expect(globalThis.navigator.clipboard.writeText).toHaveBeenCalledWith(
      expect.stringContaining('/uploads/pic.jpg'),
    )
    expect(ElMessage.success).toHaveBeenCalledWith('URL 已复制')
  })

  it('onUploadSuccess shows success and refetches media', async () => {
    const wrapper = mountWithPlugins(MediaLibrary)
    await flushPromises()

    vi.mocked(mediaApi.list).mockClear()
    vi.mocked(mediaApi.list).mockResolvedValue({ items: mockMedia, total: 2 } as any)

    const vm = wrapper.vm as any
    vm.onUploadSuccess({ data: { url: '/uploads/new.png' } })
    await flushPromises()

    expect(ElMessage.success).toHaveBeenCalledWith('上传成功')
    expect(mediaApi.list).toHaveBeenCalled()
  })

  it('formats sizes correctly across unit boundaries', () => {
    const wrapper = mountWithPlugins(MediaLibrary)
    const vm = wrapper.vm as any
    expect(vm.formatSize(512)).toBe('512 B')
    expect(vm.formatSize(2048)).toBe('2.0 KB')
    expect(vm.formatSize(5242880)).toBe('5.0 MB')
    expect(vm.formatSize(1073741824)).toBe('1.0 GB')
  })

  it('formats dates as YYYY-MM-DD HH:mm', () => {
    const wrapper = mountWithPlugins(MediaLibrary)
    const vm = wrapper.vm as any
    expect(vm.formatDate('2024-01-02T03:04:05Z')).toMatch(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}/)
  })

  it('refetches media when filters.type changes', async () => {
    const wrapper = mountWithPlugins(MediaLibrary)
    await flushPromises()

    vi.mocked(mediaApi.list).mockClear()
    vi.mocked(mediaApi.list).mockResolvedValue({ items: [], total: 0 } as any)

    const vm = wrapper.vm as any
    vm.filters.type = 'image'
    await vm.fetchMedia()
    await flushPromises()

    expect(mediaApi.list).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'image' }),
    )
  })
})
