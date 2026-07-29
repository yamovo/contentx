export const backupQueryKeys = {
  all: ['backups'] as const,
  lists: () => [...backupQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) => [...backupQueryKeys.lists(), params] as const,
}
