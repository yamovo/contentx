export const settingsQueryKeys = {
  all: ['settings'] as const,
  lists: () => [...settingsQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...settingsQueryKeys.lists(), params] as const,
  detail: (id: number) => [...settingsQueryKeys.all, 'detail', id] as const,
  group: (group: string) => [...settingsQueryKeys.all, 'group', group] as const,
  public: () => [...settingsQueryKeys.all, 'public'] as const,
}
