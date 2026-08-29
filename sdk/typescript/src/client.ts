import type {
  ContentXConfig,
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
  PublicContentEntry,
  Tenant,
  TenantMember,
  Webhook,
  CreateArticleInput,
  UpdateArticleInput,
  CreateEntryInput,
  UpdateEntryInput,
  ListParams,
} from './types.js'

export class ContentX {
  private baseURL: string
  private token: string
  private tenantID?: number
  private timeout: number

  constructor(config: ContentXConfig) {
    this.baseURL = config.baseURL.replace(/\/$/, '')
    this.token = config.token || ''
    this.tenantID = config.tenantID
    this.timeout = config.timeout || 30000
  }

  setToken(token: string) {
    this.token = token
  }

  setTenantID(tenantID: number) {
    this.tenantID = tenantID
  }

  clearTenantID() {
    this.tenantID = undefined
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
      this.post<{ message: string }>('/auth/logout'),
  }

  // ─── Articles ───────────────────────────────────────────────

  articles = {
    list: (params?: ListParams) =>
      this.get<PaginatedResponse<Article>>('/articles', params),

    get: (id: number) =>
      this.get<Article>(`/articles/${id}`),

    getBySlug: (slug: string) =>
      this.get<Article>(`/articles/slug/${slug}`),

    create: (data: CreateArticleInput) =>
      this.post<Article>('/articles', data),

    update: (id: number, data: UpdateArticleInput) =>
      this.put<Article>(`/articles/${id}`, data),

    delete: (id: number) =>
      this.del<{ message: string }>(`/articles/${id}`),

    like: (id: number) =>
      this.post<{ message: string }>(`/articles/${id}/like`),
  }

  // ─── Categories ─────────────────────────────────────────────

  categories = {
    list: (params?: ListParams) =>
      this.get<Category[]>('/categories', params),

    get: (id: number) =>
      this.get<Category>(`/categories/${id}`),

    create: (data: Partial<Category>) =>
      this.post<Category>('/categories', data),

    update: (id: number, data: Partial<Category>) =>
      this.put<{ message: string }>(`/categories/${id}`, data),

    delete: (id: number) =>
      this.del<{ message: string }>(`/categories/${id}`),
  }

  // ─── Tags ───────────────────────────────────────────────────

  tags = {
    list: (params?: ListParams) =>
      this.get<{ data: Tag[]; total: number }>('/tags', params),

    get: (id: number) =>
      this.get<Tag>(`/tags/${id}`),

    create: (data: Partial<Tag>) =>
      this.post<Tag>('/tags', data),

    update: (id: number, data: Partial<Tag>) =>
      this.put<{ message: string }>(`/tags/${id}`, data),

    delete: (id: number) =>
      this.del<{ message: string }>(`/tags/${id}`),

    merge: (sourceIds: number[], targetId: number) =>
      this.post<{ message: string }>('/tags/merge', { source_ids: sourceIds, target_id: targetId }),
  }

  // ─── Comments ───────────────────────────────────────────────

  comments = {
    list: (params?: ListParams) =>
      this.get<PaginatedResponse<Comment>>('/comments', params),

    create: (data: { article_id: number; content: string; parent_id?: number }) =>
      this.post<Comment>('/comments', data),

    approve: (id: number) =>
      this.post<{ message: string }>(`/comments/${id}/approve`),

    spam: (id: number) =>
      this.post<{ message: string }>(`/comments/${id}/spam`),

    trash: (id: number) =>
      this.post<{ message: string }>(`/comments/${id}/trash`),
  }

  // ─── Media ──────────────────────────────────────────────────

  media = {
    list: (params?: ListParams) =>
      this.get<PaginatedResponse<Media>>('/media', params),

    get: (id: number) =>
      this.get<Media>(`/media/${id}`),

    delete: (id: number) =>
      this.del<{ message: string }>(`/media/${id}`),

    upload: async (file: File | Blob, filename?: string) => {
      const formData = new FormData()
      formData.append('file', file, filename)
      return this.rawFetch<Media>('/media/upload', {
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
      this.get<ContentType>(`/content-types/${uid}`),

    create: (data: { uid: string; name: string; fields: any[] }) =>
      this.post<ContentType>('/content-types', data),

    delete: (uid: string) =>
      this.del<{ message: string }>(`/content-types/${uid}`),
  }

  // ─── Dynamic Content ───────────────────────────────────────

  content(uid: string) {
    return {
      list: (params?: ListParams) =>
        this.get<PaginatedResponse<ContentEntry>>(`/content/${uid}`, params),

      get: (documentId: string) =>
        this.get<ContentEntry>(`/content/${uid}/${documentId}`),

      create: (data: CreateEntryInput) =>
        this.post<ContentEntry>(`/content/${uid}`, data),

      update: (documentId: string, data: UpdateEntryInput) =>
        this.put<ContentEntry>(`/content/${uid}/${documentId}`, data),

      delete: (documentId: string) =>
        this.del<{ message: string }>(`/content/${uid}/${documentId}`),

      publish: (documentId: string) =>
        this.post<ContentEntry>(`/content/${uid}/${documentId}/publish`),

      unpublish: (documentId: string) =>
        this.post<ContentEntry>(`/content/${uid}/${documentId}/unpublish`),

      export: () =>
        this.get<{ json: string }>(`/content/${uid}/export`),

      import: (json: string) =>
        this.post<{ imported: number }>(`/content/${uid}/import`, { json }),
    }
  }

  // ─── Tenants (platform, RFC-001 PR-5) ───────────────────────

  /**
   * Platform tenant administration. These endpoints require the
   * tenants.read / tenants.manage platform permissions; tenant membership
   * roles can never reach the identity plane.
   */
  tenants = {
    list: () => this.get<Tenant[]>('/admin/tenants'),

    get: (id: number) => this.get<Tenant>(`/admin/tenants/${id}`),

    create: (data: { name: string; slug: string; max_users?: number }) =>
      this.post<Tenant>('/admin/tenants', data),

    update: (id: number, data: { name?: string; status?: 'active' | 'suspended'; max_users?: number }) =>
      this.put<Tenant>(`/admin/tenants/${id}`, data),

    listMembers: (id: number) => this.get<TenantMember[]>(`/admin/tenants/${id}/members`),

    addMember: (id: number, data: { user_id: number; role_slug: 'admin' | 'editor' | 'member' }) =>
      this.post<TenantMember>(`/admin/tenants/${id}/members`, data),

    updateMemberRole: (id: number, userId: number, role_slug: 'admin' | 'editor' | 'member') =>
      this.put<{ message: string }>(`/admin/tenants/${id}/members/${userId}`, { role_slug }),

    removeMember: (id: number, userId: number) =>
      this.del<{ message: string }>(`/admin/tenants/${id}/members/${userId}`),
  }

  // ─── Public Content (RFC-002) ───────────────────────────────

  /**
   * Public, read-only delivery of published dynamic content (RFC-002).
   * Only content types on the server's CONTENT_DELIVERY_UIDS allowlist are
   * exposed; unknown, unpublished, or non-allowlisted documents return 404.
   * These reads are anonymous on the server: any token or tenant configured
   * on this client is ignored for this surface, and unpublishing takes
   * effect immediately (Cache-Control: no-store).
   */
  publicContent(uid: string) {
    return {
      list: (params?: { page?: number; page_size?: number; locale?: string }) =>
        this.get<PaginatedResponse<PublicContentEntry>>(`/public/content/${uid}`, params),

      get: (documentId: string) =>
        this.get<PublicContentEntry>(`/public/content/${uid}/${documentId}`),
    }
  }

  // ─── Webhooks ───────────────────────────────────────────────

  webhooks = {
    list: () =>
      this.get<Webhook[]>('/webhooks'),

    create: (data: { name: string; url: string; events: string[]; secret?: string }) =>
      this.post<Webhook>('/webhooks', data),

    delete: (id: number) =>
      this.del<{ message: string }>(`/webhooks/${id}`),

    logs: (id: number, limit?: number) =>
      this.get<unknown[]>(`/webhooks/${id}/logs`, { limit }),
  }

  // ─── System ─────────────────────────────────────────────────

  system = {
    health: () =>
      this.get<{ status: string; database: boolean }>('/system/health'),

    info: () =>
      this.get<Record<string, unknown>>('/system/info'),
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

    const opts: RequestInit = { method, headers }
    if (body !== undefined && method !== 'GET') {
      opts.body = JSON.stringify(body)
    }

    return this.rawFetch<T>(url, opts, true)
  }

  private async rawFetch<T>(path: string, opts: RequestInit, absolute = false): Promise<T> {
    const url = absolute ? path : `${this.baseURL}${path}`
    const headers: Record<string, string> = {}
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }
    if (this.tenantID !== undefined) {
      headers['X-Tenant-ID'] = String(this.tenantID)
    }

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const resp = await fetch(url, {
        ...opts,
        headers: { ...headers, ...opts.headers },
        signal: controller.signal,
      })
      const payload = await this.readPayload(resp)
      const envelope = this.asAPIResponse<T>(payload)

      if (!resp.ok || (envelope && envelope.code !== 0)) {
        throw this.toResponseError(resp, payload, envelope)
      }

      if (envelope) {
        return envelope.data as T
      }

      // The health endpoint is intentionally a bare JSON object, while 204
      // and empty successful responses have no payload at all.
      return payload as T
    } catch (error: unknown) {
      if (error instanceof Error && error.name === 'AbortError') {
        throw new ContentXError('Request timeout', 408)
      }
      throw error
    } finally {
      clearTimeout(timeoutId)
    }
  }

  private async readPayload(resp: Response): Promise<unknown> {
    const text = await resp.text()
    if (!text.trim()) return undefined

    try {
      return JSON.parse(text)
    } catch {
      throw new ContentXError('Invalid JSON response', resp.status)
    }
  }

  private asAPIResponse<T>(payload: unknown): APIResponse<T> | undefined {
    if (!payload || typeof payload !== 'object') return undefined

    const value = payload as Record<string, unknown>
    if (typeof value.code !== 'number' || typeof value.message !== 'string') return undefined
    return value as unknown as APIResponse<T>
  }

  private toResponseError(
    resp: Response,
    payload: unknown,
    envelope?: APIResponse<unknown>,
  ): ContentXError {
    if (envelope) {
      return new ContentXError(envelope.message || resp.statusText, resp.status, envelope.err_code)
    }

    const value = payload && typeof payload === 'object'
      ? payload as Record<string, unknown>
      : undefined
    const message = typeof value?.message === 'string'
      ? value.message
      : typeof value?.error === 'string'
        ? value.error
        : resp.statusText || `Request failed with status ${resp.status}`
    const errCode = typeof value?.err_code === 'string' ? value.err_code : undefined
    return new ContentXError(message, resp.status, errCode)
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

export class ContentXError extends Error {
  status: number
  code?: string

  constructor(message: string, status: number, code?: string) {
    super(message)
    this.name = 'ContentXError'
    this.status = status
    this.code = code
  }
}
