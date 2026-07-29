import { get, post, put, del } from '../http'
import { getList } from '../helpers'

// ─── Types ───────────────────────────────────────────────

export interface Media {
  id: number
  filename: string
  original_name: string
  url: string
  thumbnail_url: string
  mime_type: string
  file_size: number
  width?: number
  height?: number
  alt: string
  title: string
  caption: string
  folder: string
  created_at: string
}

// ─── API ─────────────────────────────────────────────────

export const mediaApi = {
  list: (params?: Record<string, unknown>) => getList<Media>('/media', params),
  get: (id: number) => get<{ data: Media }>(`/media/${id}`),
  upload: (formData: FormData) =>
    post<{ data: Media }>('/media/upload', formData),
  update: (id: number, data: Partial<Media>) => put(`/media/${id}`, data),
  delete: (id: number) => del(`/media/${id}`),
  bulkDelete: (ids: number[]) => post('/media/bulk-delete', { ids }),
  folders: () => get<{ data: string[] }>('/media/folders'),
  stats: () => get<{ data: { total_files: number; total_size: number; images: number; videos: number; documents: number } }>('/media/stats'),
}
