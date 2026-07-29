export interface ApiEnvelope<T> {
  code: 0 | -1
  err_code?: string
  message: string
  data?: T
  meta?: PageMeta
}

export interface PageMeta {
  page: number
  page_size: number
  total: number
  has_next: boolean
  has_prev: boolean
}

export interface PageResult<T> {
  items: T[]
  meta: PageMeta
}

export interface ApiError {
  status: number
  code: string
  message: string
  requestId?: string
}

export class ApiRequestError extends Error implements ApiError {
  readonly status: number
  readonly code: string
  readonly requestId?: string

  constructor(error: ApiError) {
    super(error.message)
    this.name = 'ApiRequestError'
    this.status = error.status
    this.code = error.code
    this.requestId = error.requestId
  }
}

export function isApiError(value: unknown): value is ApiError {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Partial<ApiError>
  return (
    typeof candidate.status === 'number' &&
    typeof candidate.code === 'string' &&
    typeof candidate.message === 'string'
  )
}
