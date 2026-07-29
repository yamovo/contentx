import { get, post, put } from '../http'
import { getList } from '../helpers'
import type { Article } from './articles'
import type { User } from './auth'

// ─── Types ───────────────────────────────────────────────

export interface Comment {
  id: number
  article_id: number
  article?: Article
  user_id: number | null
  user?: User
  parent_id: number | null
  children?: Comment[]
  author_name: string
  author_email: string
  author_url: string
  content: string
  status: 'pending' | 'approved' | 'spam' | 'trash'
  depth: number
  like_count: number
  is_sticky: boolean
  created_at: string
}

// ─── API ─────────────────────────────────────────────────

export const commentApi = {
  list: (params?: Record<string, unknown>) => getList<Comment>('/comments', params),
  get: (id: number) => get<{ data: Comment }>(`/comments/${id}`),
  create: (data: Partial<Comment>) => post<{ data: Comment }>('/comments', data),
  update: (id: number, data: Partial<Comment>) => put(`/comments/${id}`, data),
  approve: (id: number) => post(`/comments/${id}/approve`),
  spam: (id: number) => post(`/comments/${id}/spam`),
  trash: (id: number) => post(`/comments/${id}/trash`),
  bulk: (data: { comment_ids: number[]; action: string }) => post('/comments/bulk', data),
  stats: () => get<{ data: { total: number; pending: number; approved: number; spam: number; today: number } }>('/comments/stats'),
  articleComments: (articleId: number) => get<{ data: Comment[] }>(`/articles/${articleId}/comments`),
}
