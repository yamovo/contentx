import { get, post, put, del } from '../http'
import { getList } from '../helpers'
import type { User } from './auth'
import type { Category } from './categories'
import type { Tag } from './tags'

// ─── Types ───────────────────────────────────────────────

export interface Article {
  id: number
  title: string
  slug: string
  content?: string
  excerpt: string
  author: User
  author_id: number
  category: Category | null
  category_id: number | null
  tags: Tag[]
  featured_image: string
  status: 'draft' | 'published' | 'pending' | 'scheduled' | 'trash' | 'archived'
  post_type: 'post' | 'page'
  format: string
  visibility: 'public' | 'private' | 'password'
  is_pinned: boolean
  is_featured: boolean
  allow_comment: boolean
  view_count: number
  like_count: number
  word_count: number
  reading_time: number
  published_at: string | null
  scheduled_at: string | null
  meta_title: string
  meta_desc: string
  meta_keywords: string
  comment_count: number
  created_at: string
  updated_at: string
}

export interface ArticleCreateInput {
  title: string
  slug?: string
  content?: string
  excerpt?: string
  category_id?: number | null
  tag_ids?: number[]
  featured_image?: string
  post_type: Article['post_type']
  visibility?: Article['visibility']
  password?: string
  is_pinned?: boolean
  is_featured?: boolean
  allow_comment?: boolean
  meta_title?: string
  meta_desc?: string
  meta_keywords?: string
}

export type ArticleUpdateInput = Omit<ArticleCreateInput, 'post_type'> & {
  post_type?: never
  status?: never
  published_at?: never
  scheduled_at?: never
}

export interface Revision {
  id: number
  article_id: number
  title: string
  content: string
  excerpt: string
  editor: User
  version: number
  note: string
  created_at: string
}

// ─── API ─────────────────────────────────────────────────

export const articleApi = {
  list: (params?: Record<string, unknown>, signal?: AbortSignal) =>
    getList<Article>('/articles', params, signal),
  get: (id: number) => get<{ data: Article }>(`/articles/${id}`),
  getBySlug: (slug: string) => get<{ data: Article }>(`/articles/slug/${slug}`),
  create: (data: ArticleCreateInput) => post<{ data: Article }>('/articles', data),
  update: (id: number, data: ArticleUpdateInput) =>
    put<{ data: Article }>(`/articles/${id}`, data),
  delete: (id: number) => del(`/articles/${id}`),
  bulk: (data: {
    article_ids: number[]
    action: 'publish' | 'unpublish' | 'draft' | 'trash' | 'delete'
    category_id?: number
  }) =>
    post('/articles/bulk', data),
  revisions: (id: number) => get<{ data: Revision[] }>(`/articles/${id}/revisions`),
  restoreRevision: (id: number, revisionId: number) =>
    post(`/articles/${id}/revisions/${revisionId}/restore`),
  publish: (id: number) => post<{ data: Article }>(`/articles/${id}/publish`),
  unpublish: (id: number) => post<{ data: Article }>(`/articles/${id}/unpublish`),
  submitReview: (id: number) => post<{ data: Article }>(`/articles/${id}/submit-review`),
  approve: (id: number) => post<{ data: Article }>(`/articles/${id}/approve`),
  schedule: (id: number, scheduled_at: string) =>
    post<{ data: Article }>(`/articles/${id}/schedule`, { scheduled_at }),
  archive: (id: number) => post<{ data: Article }>(`/articles/${id}/archive`),
  like: (id: number) => post(`/articles/${id}/like`),
}
