export interface ContentXConfig {
  baseURL: string
  token?: string
  timeout?: number
}

export interface APIResponse<T = any> {
  code: number
  message: string
  data?: T
  meta?: PaginationMeta
}

export interface PaginationMeta {
  page: number
  page_size: number
  total: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
}

export interface PaginatedResponse<T> {
  items: T[]
  page: number
  page_size: number
  total: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  expires_at: string
  expires_in: number
}

export interface User {
  id: number
  username: string
  email: string
  display_name: string
  avatar: string
  bio: string
  website: string
  role: Role
  status: string
  created_at: string
}

export interface Role {
  id: number
  name: string
  slug: string
  description: string
  permissions?: Permission[]
}

export interface Permission {
  id: number
  name: string
  slug: string
  module: string
}

export interface Article {
  id: number
  title: string
  slug: string
  content?: string
  excerpt: string
  author?: User
  author_id: number
  category?: Category
  category_id?: number
  tags?: Tag[]
  featured_image: string
  status: string
  post_type: string
  view_count: number
  like_count: number
  published_at?: string
  created_at: string
  updated_at: string
}

export interface Category {
  id: number
  name: string
  slug: string
  description: string
  parent_id?: number
  children?: Category[]
  post_count: number
}

export interface Tag {
  id: number
  name: string
  slug: string
  count: number
  color: string
}

export interface Comment {
  id: number
  article_id: number
  content: string
  status: string
  author_name: string
  created_at: string
}

export interface Media {
  id: number
  filename: string
  original_name: string
  url: string
  thumbnail_url: string
  mime_type: string
  file_size: number
  alt: string
  created_at: string
}

export interface ContentType {
  id: number
  uid: string
  name: string
  description: string
  is_single: boolean
  draft_publish: boolean
  fields: ContentField[]
  entry_count: number
}

export interface ContentField {
  id: number
  name: string
  label: string
  field_type: string
  required: boolean
  options?: string[]
}

export interface ContentEntry {
  id: number
  content_type_id: number
  document_id: string
  status: string
  data: Record<string, any>
  created_at: string
  updated_at: string
}

export interface Webhook {
  id: number
  name: string
  url: string
  events: string[]
  is_active: boolean
  created_at: string
}

export interface CreateArticleInput {
  title: string
  slug?: string
  content?: string
  excerpt?: string
  category_id?: number
  tag_ids?: number[]
  status?: string
  featured_image?: string
}

export interface UpdateArticleInput extends Partial<CreateArticleInput> {}

export interface CreateEntryInput {
  data: Record<string, any>
  status?: string
}

export interface UpdateEntryInput {
  data?: Record<string, any>
  status?: string
}

export interface ListParams {
  page?: number
  page_size?: number
  status?: string
  search?: string
  sort?: string
  [key: string]: any
}
