import http from './http'
import type { PageMeta, PageResult } from '@/shared/api/types'

export interface ListResponse<T> extends PageResult<T> {
  items: T[]
  page: number
  page_size: number
  total: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
}

// The backend wraps every response in an envelope: {code, message, data}.
// Paginated list endpoints put the whole ListResponse inside `data`, so it
// must be unwrapped here — views consume `res.items` / `res.total` directly.
export function getList<T>(
  url: string,
  params?: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<ListResponse<T>> {
  return http
    .get<{ data: Omit<ListResponse<T>, 'meta'> }>(url, { params, signal })
    .then((response) => {
      const result = response.data.data
      const meta: PageMeta = {
        page: result.page,
        page_size: result.page_size,
        total: result.total,
        has_next: result.has_next,
        has_prev: result.has_prev,
      }
      return { ...result, meta }
    })
}
