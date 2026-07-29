import { get, post, put } from '../http'

// ─── Types ───────────────────────────────────────────────

export interface Plugin {
  id: number
  name: string
  version: string
  description: string
  author: string
  is_enabled: boolean
  config?: Record<string, unknown>
}

export interface Theme {
  id: number
  name: string
  version: string
  description: string
  screenshot: string
  is_active: boolean
  config?: Record<string, unknown>
}

// ─── API ─────────────────────────────────────────────────

export const pluginApi = {
  list: () => get<{ data: Plugin[] }>('/plugins'),
  enable: (id: number) => post(`/plugins/${id}/enable`),
  disable: (id: number) => post(`/plugins/${id}/disable`),
  updateConfig: (id: number, config: Record<string, unknown>) => put(`/plugins/${id}/config`, config),
}

export const themeApi = {
  list: () => get<{ data: Theme[] }>('/themes'),
  activate: (id: number) => post(`/themes/${id}/activate`),
  updateConfig: (id: number, config: Record<string, unknown>) => put(`/themes/${id}/config`, config),
}
