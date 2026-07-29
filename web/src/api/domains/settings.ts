import { get, put } from '../http'

// ─── Types ───────────────────────────────────────────────

export interface SiteSetting {
  id: number
  key: string
  value: string
  type: string
  group: string
  label: string
  help_text: string
}

// ─── API ─────────────────────────────────────────────────

export const settingsApi = {
  // Backend shape: envelope.data = { data: SiteSetting[], grouped } — unwrap.
  list: (group?: string) =>
    get<{ data: { data: SiteSetting[]; grouped: Record<string, SiteSetting[]> } }>('/settings', { group }).then(r => r.data),
  get: (key: string) => get<{ data: SiteSetting }>(`/settings/${key}`),
  update: (data: Record<string, unknown>) => put('/settings', data),
  public: () => get<{ data: Record<string, string> }>('/settings/public'),
}
