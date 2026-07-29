import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { menuApi, type Menu, type MenuItem } from '@/api'
import { menuQueryKeys } from '@/entities/menu/api/query-keys'

export function useMenuMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: menuQueryKeys.all })
  }

  const createMenu = useMutation({
    mutationFn: (data: Partial<Menu>) => menuApi.create(data),
    onSuccess: invalidate,
  })

  const updateMenu = useMutation({
    mutationFn: ({ id, ...data }: Partial<Menu> & { id: number }) =>
      menuApi.update(id, data),
    onSuccess: invalidate,
  })

  const deleteMenu = useMutation({
    mutationFn: (id: number) => menuApi.delete(id),
    onSuccess: invalidate,
  })

  const addMenuItem = useMutation({
    mutationFn: ({ menuId, data }: { menuId: number; data: Partial<MenuItem> }) =>
      menuApi.addItem(menuId, data),
    onSuccess: invalidate,
  })

  const updateMenuItem = useMutation({
    mutationFn: ({ menuId, itemId, data }: { menuId: number; itemId: number; data: Partial<MenuItem> }) =>
      menuApi.updateItem(menuId, itemId, data),
    onSuccess: invalidate,
  })

  const deleteMenuItem = useMutation({
    mutationFn: ({ menuId, itemId }: { menuId: number; itemId: number }) =>
      menuApi.deleteItem(menuId, itemId),
    onSuccess: invalidate,
  })

  const reorderMenuItems = useMutation({
    mutationFn: ({ menuId, items }: { menuId: number; items: { id: number; sort_order: number; parent_id?: number }[] }) =>
      menuApi.reorderItems(menuId, items),
    onSuccess: invalidate,
  })

  return { createMenu, updateMenu, deleteMenu, addMenuItem, updateMenuItem, deleteMenuItem, reorderMenuItems }
}
