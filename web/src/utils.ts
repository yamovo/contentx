import dayjs from 'dayjs'

/**
 * Format an ISO date string using dayjs.
 * @param s ISO date string
 * @param fmt dayjs format token (default: 'YYYY-MM-DD HH:mm')
 */
export function formatDate(s: string, fmt = 'YYYY-MM-DD HH:mm'): string {
  return dayjs(s).format(fmt)
}

/**
 * Format a byte count as a human-readable string (B / KB / MB / GB).
 */
export function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(1) + ' GB'
}

/**
 * Build a tree from a flat list of nodes with parent_id references.
 * @param items flat node list
 * @param parentId root parent id (default: null = top-level)
 */
export function buildTree<T extends { id: number; parent_id: number | null }>(
  items: T[],
  parentId: number | null = null,
): (T & { children: ReturnType<typeof buildTree<T>> })[] {
  return items
    .filter(c => c.parent_id === parentId)
    .map(c => ({ ...c, children: buildTree(items, c.id) }))
}

/**
 * Shape of an axios/HTTP error carrying a backend error message.
 * Replaces ad-hoc `catch (err: any)` patterns across views.
 */
interface HttpError {
  response?: { data?: { error?: string } }
}

/**
 * Extract a user-facing error message from a caught unknown value.
 * Falls back to `fallback` when the error doesn't carry a backend message.
 */
export function getApiError(err: unknown, fallback = '操作失败'): string {
  const e = err as HttpError
  return e?.response?.data?.error || fallback
}
