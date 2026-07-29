import { get, post } from '../http'
import type { Article } from './articles'
import type { Comment } from './comments'

// ─── Types ───────────────────────────────────────────────

export interface DashboardStats {
  total_articles: number
  published_articles: number
  total_comments: number
  pending_comments: number
  total_users: number
  total_media: number
  views_today: number
  views_this_week: number
  views_this_month: number
  total_views: number
}

export interface DeviceBreakdownItem {
  name: string
  count: number
}

export interface DeviceBreakdownResponse {
  devices: DeviceBreakdownItem[]
  browsers: DeviceBreakdownItem[]
  os: DeviceBreakdownItem[]
}

// ─── API ─────────────────────────────────────────────────

export const analyticsApi = {
  dashboard: () => get<{ data: { stats: DashboardStats; recent_articles: Article[]; recent_comments: Comment[]; popular_articles: Article[] } }>('/analytics/dashboard'),
  viewsOverTime: (days?: number) => get<{ data: { date: string; views: number }[] }>('/analytics/views', { days }),
  topReferrers: () => get<{ data: { referrer: string; count: number }[] }>('/analytics/referrers'),
  deviceBreakdown: () => get<{ data: DeviceBreakdownResponse }>('/analytics/devices'),
  recordView: (data: { article_id?: number; path: string; duration?: number }) =>
    post('/analytics/record', data),
}
