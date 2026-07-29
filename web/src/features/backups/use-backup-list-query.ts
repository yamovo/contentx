import { useQuery } from '@tanstack/vue-query'
import { backupApi } from '@/api'
import { backupQueryKeys } from '@/entities/backup/api/query-keys'

export function useBackupListQuery() {
  return useQuery({
    queryKey: backupQueryKeys.lists(),
    queryFn: () => backupApi.list(),
    placeholderData: (previous) => previous,
  })
}
