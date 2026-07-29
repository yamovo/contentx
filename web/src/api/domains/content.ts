import { get, post, put, del } from '../http'
import { getList } from '../helpers'

// ─── Types ───────────────────────────────────────────────

export interface ContentField {
  id?: number
  name: string
  label: string
  field_type: string
  required: boolean
  unique: boolean
  default_value?: string
  options?: string[] | null
  relation_type?: string
  relation_uid?: string
  min_length?: number | null
  max_length?: number | null
  min_value?: number | null
  max_value?: number | null
  sort_order?: number
}

export interface ContentType {
  id: number
  uid: string
  name: string
  description: string
  is_single: boolean
  draft_publish: boolean
  fields?: ContentField[]
  entry_count?: number
  created_at: string
  updated_at: string
}

export interface ContentEntry {
  id: number
  content_type_id: number
  document_id: string
  status: 'draft' | 'published'
  data: Record<string, unknown>
  locale: string
  published_at: string | null
  created_at: string
  updated_at: string
}

// ─── API ─────────────────────────────────────────────────

export const contentApi = {
  listTypes: () => get<{ data: ContentType[] }>('/content-types'),
  getType: (uid: string) => get<{ data: ContentType }>(`/content-types/${uid}`),
  createType: (data: {
    uid: string
    name: string
    description?: string
    is_single?: boolean
    draft_publish?: boolean
    fields: ContentField[]
  }) => post<{ data: ContentType }>('/content-types', data),
  deleteType: (uid: string) => del(`/content-types/${uid}`),

  listEntries: (uid: string, params?: Record<string, unknown>) =>
    getList<ContentEntry>(`/content/${uid}`, params),
  getEntry: (uid: string, documentId: string) =>
    get<{ data: ContentEntry }>(`/content/${uid}/${documentId}`),
  createEntry: (uid: string, data: { data: Record<string, unknown>; status?: string; locale?: string }) =>
    post<{ data: ContentEntry }>(`/content/${uid}`, data),
  updateEntry: (uid: string, documentId: string, data: { data?: Record<string, unknown>; status?: string }) =>
    put<{ data: ContentEntry }>(`/content/${uid}/${documentId}`, data),
  deleteEntry: (uid: string, documentId: string) => del(`/content/${uid}/${documentId}`),
  publishEntry: (uid: string, documentId: string) =>
    post<{ data: ContentEntry }>(`/content/${uid}/${documentId}/publish`),
  unpublishEntry: (uid: string, documentId: string) =>
    post<{ data: ContentEntry }>(`/content/${uid}/${documentId}/unpublish`),
  exportEntries: (uid: string) => get<{ data: { json: string } }>(`/content/${uid}/export`),
  importEntries: (uid: string, json: string) =>
    post<{ data: { imported: number } }>(`/content/${uid}/import`, { json }),

  // i18n: sibling translations of an entry (same translation group).
  listTranslations: (uid: string, documentId: string) =>
    get<{ data: ContentEntry[] }>(`/content/${uid}/${documentId}/translations`),
  createTranslation: (uid: string, documentId: string, locale: string, data: { data: Record<string, unknown>; status?: string }) =>
    post<{ data: ContentEntry }>(`/content/${uid}/${documentId}/translations?locale=${encodeURIComponent(locale)}`, data),
}
