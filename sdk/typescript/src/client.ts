import type {
  VortexConfig,
  APIResponse,
  PaginatedResponse,
  TokenPair,
  User,
  Article,
  Category,
  Tag,
  Comment,
  Media,
  ContentType,
  ContentEntry,
  Webhook,
  CreateArticleInput,
  UpdateArticleInput,
  CreateEntryInput,
  UpdateEntryInput,
  ListParams,
} from './types'

export class VortexCMS {
  private baseURL: string
  private token: string
  private timeout: number

  constructor(config: VortexConfig) {
    this.baseURL = config.baseURL.replace(/\/$/, '')
    this.token = config.token || ''
    this.timeout = config.timeout || 30000
  }

  setToken(token: string) {
    this.token = token
  }

  // ─── Auth ───────────────────────────────────────────────────

  auth = {
    login: (username: string, password: string) =>
      this.post<{ token: TokenPair; user: User }>('/auth/login', { username, password }),

    register: (data: { username: string; email: string; password: string; display_name?: string }) =>
      this.post<{ token: TokenPair; user: User }>('/auth/register', data),

    me: () =>
      this.get<{ user: User; permissions: string[] }>('/auth/me'),

    refresh: (refreshToken: string) =>
      this.post<TokenPair>('/auth/refresh', { refresh_token: refreshToken }),

    logout: () =>
      this.post('/auth/logout'),
  }

  // ─── Articles ───────────────────────────────────────────────

  articles = {
    list: (params?: ListParams) =>
      this.get<PaginatedResponse<Article>>('/articles', params),

    get: (id: number) =>
      this.get<{ data: Article }>(`/articles/${id}`),

    getBySlug: (slug: string) =>
      this.get<{ data: Article }>(`/articles/slug/${slug}`),

    create: (data: CreateArticleInput) =>
      this.post<{ data: Article }>('/articles', data),

    update: (id: number, data: UpdateArticleInput) =>
      this.put<{ data: Article }>(`/articles/${id}`, data),

    delete: (id: number) =>
      this.del(`/articles/${id}`),

    like: (id: number) =>
      this.post(`/articles/${id}/like`),
  }

  // ─── Categories ─────────────────────────────────────────────

  categories = {
    list: (params?: ListParams) =>
      this.get<PaginatedResponse<Category>>('/categories', params),

    get: (id: number) =>
      this.get<{ data: Category }>(`/categories/${id}`),

    create: (data: Partial<Category>) =>
      this.post<{ data: Category }>('/categories', data),

    update: (id: number, data: Partial<Category>) =>
      this.put<{ data: Category }>(`/categories/${id}`, data),

    delete: (id: number) =>
      this.del(`/categories/${id}`),
  }

  // ─── Tags ───────────────────────────────────────────────────

  tags = {
    list: (params?: ListParams) =>
      this.get<PaginatedResponse<Tag>>('/tags', params),

    get: (id: number) =>
      this.get<{ data: Tag }>(`/tags/${id}`),

    create: (data: Partial<Tag>) =>
      this.post<{ data: Tag }>('/tags', data),

    update: (id: number, data: Partial<Tag>) =>
      this.put<{ data: Tag }>(`/tags/${id}`, data),

    delete: (id: number) =>
      this.del(`/tags/${id}`),

    merge: (sourceIds: number[], targetId: number) =>
      this.post('/tags/merge', { source_ids: sourceIds, target_id: targetId }),
  }

  // ─── Comments ───────────────────────────────────────────────

  comments = {
    list: (params?: ListParams) =>
      this.get<PaginatedResponse<Comment>>('/comments', params),

    create: (data: { article_id: number; content: string; parent_id?: number }) =>
      this.post<{ data: Comment }>('/comments', data),

    approve: (id: number) =>
      this.post(`/comments/${id}/approve`),

    spam: (id: number) =>
      this.post(`/comments/${id}/spam`),

    trash: (id: number) =>
      this.post(`/comments/${id}/trash`),
  }

  // ─── Media ──────────────────────────────────────────────────

  media = {
    list: (params?: ListParams) =>
      this.get<PaginatedResponse<Media>>('/media', params),

    get: (id: number) =>
      this.get<{ data: Media }>(`/media/${id}`),

    delete: (id: number) =>
      this.del(`/media/${id}`),

    upload: async (file: File | Blob, filename?: string) => {
      const formData = new FormData()
      formData.append('file', file, filename)
      return this.rawFetch('/media/upload', {
        method: 'POST',
        body: formData,
        headers: {}, // let browser set Content-Type
      })
    },
  }

  // ─── Content Types ─────────────────────────────────────────

  contentTypes = {
    list: () =>
      this.get<ContentType[]>('/content-types'),

    get: (uid: string) =>
      this.get<{ data: ContentType }>(`/content-types/${uid}`),

    create: (data: { uid: string; name: string; fields: any[] }) =>
      this.post<{ data: ContentType }>('/content-types', data),

    delete: (uid: string) =>
      this.del(`/content-types/${uid}`),
  }

  // ─── Dynamic Content ───────────────────────────────────────

  content(uid: string) {
    return {
      list: (params?: ListParams) =>
        this.get<PaginatedResponse<ContentEntry>>(`/content/${uid}`, params),

      get: (documentId: string) =>
        this.get<{ data: ContentEntry }>(`/content/${uid}/${documentId}`),

      create: (data: CreateEntryInput) =>
        this.post<{ data: ContentEntry }>(`/content/${uid}`, data),

      update: (documentId: string, data: UpdateEntryInput) =>
        this.put<{ data: ContentEntry }>(`/content/${uid}/${documentId}`, data),

      delete: (documentId: string) =>
        this.del(`/content/${uid}/${documentId}`),

      publish: (documentId: string) =>
        this.post(`/content/${uid}/${documentId}/publish`),

      unpublish: (documentId: string) =>
        this.post(`/content/${uid}/${documentId}/unpublish`),

      export: () =>
        this.get(`/content/${uid}/export`),

      import: (json: string) =>
        this.post(`/content/${uid}/import`, { json }),
    }
  }

  // ─── Webhooks ───────────────────────────────────────────────

  webhooks = {
    list: () =>
      this.get<Webhook[]>('/webhooks'),

    create: (data: { name: string; url: string; events: string[]; secret?: string }) =>
      this.post<{ data: Webhook }>('/webhooks', data),

    delete: (id: number) =>
      this.del(`/webhooks/${id}`),

    logs: (id: number, limit?: number) =>
      this.get(`/webhooks/${id}/logs`, { limit }),
  }

  // ─── System ─────────────────────────────────────────────────

  system = {
    health: () =>
      this.get<{ status: string; database: boolean }>('/system/health'),

    info: () =>
      this.get('/system/info'),
  }

  // ─── HTTP helpers ───────────────────────────────────────────

  private async request<T>(method: string, path: string, body?: any, params?: Record<string, any>): Promise<T> {
    let url = `${this.baseURL}${path}`

    if (params) {
      const searchParams = new URLSearchParams()
      for (const [key, value] of Object.entries(params)) {
        if (value !== undefined && value !== null) {
          searchParams.set(key, String(value))
        }
      }
      const qs = searchParams.toString()
      if (qs) url += `?${qs}`
    }

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const opts: RequestInit = { method, headers }
    if (body && method !== 'GET') {
      opts.body = JSON.stringify(body)
    }

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)
    opts.signal = controller.signal

    try {
      const resp = await fetch(url, opts)
      clearTimeout(timeoutId)

      if (!resp.ok) {
        const err = await resp.json().catch(() => ({ error: resp.statusText }))
        throw new VortexError(err.error || resp.statusText, resp.status, err.code)
      }

      return resp.json()
    } catch (e: any) {
      clearTimeout(timeoutId)
      if (e.name === 'AbortError') {
        throw new VortexError('Request timeout', 408)
      }
      throw e
    }
  }

  private async rawFetch(path: string, opts: RequestInit): Promise<any> {
    const url = `${this.baseURL}${path}`
    const headers: Record<string, string> = {}
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }
    const resp = await fetch(url, { ...opts, headers: { ...headers, ...opts.headers } })
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({ error: resp.statusText }))
      throw new VortexError(err.error || resp.statusText, resp.status)
    }
    return resp.json()
  }

  private get<T>(path: string, params?: Record<string, any>): Promise<T> {
    return this.request('GET', path, undefined, params)
  }

  private post<T>(path: string, body?: any): Promise<T> {
    return this.request('POST', path, body)
  }

  private put<T>(path: string, body?: any): Promise<T> {
    return this.request('PUT', path, body)
  }

  private del<T>(path: string): Promise<T> {
    return this.request('DELETE', path)
  }
}

export class VortexError extends Error {
  status: number
  code?: string

  constructor(message: string, status: number, code?: string) {
    super(message)
    this.name = 'VortexError'
    this.status = status
    this.code = code
  }
}
