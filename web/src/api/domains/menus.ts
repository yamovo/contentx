import { get, post, put, del } from '../http'

// ─── Types ───────────────────────────────────────────────

export interface Menu {
  id: number
  name: string
  slug: string
  locations: string
  items: MenuItem[]
}

export interface MenuItem {
  id: number
  menu_id: number
  parent_id: number | null
  children?: MenuItem[]
  title: string
  url: string
  target: string
  css_class: string
  icon: string
  sort_order: number
  is_active: boolean
}

// ─── API ─────────────────────────────────────────────────

export const menuApi = {
  list: () => get<{ data: Menu[] }>('/menus'),
  get: (id: number) => get<{ data: Menu }>(`/menus/${id}`),
  create: (data: Partial<Menu>) => post('/menus', data),
  update: (id: number, data: Partial<Menu>) => put(`/menus/${id}`, data),
  delete: (id: number) => del(`/menus/${id}`),
  addItem: (menuId: number, data: Partial<MenuItem>) => post(`/menus/${menuId}/items`, data),
  updateItem: (menuId: number, itemId: number, data: Partial<MenuItem>) =>
    put(`/menus/${menuId}/items/${itemId}`, data),
  deleteItem: (menuId: number, itemId: number) => del(`/menus/${menuId}/items/${itemId}`),
  reorderItems: (menuId: number, items: { id: number; sort_order: number; parent_id?: number }[]) =>
    put(`/menus/${menuId}/items/reorder`, { items }),
}
