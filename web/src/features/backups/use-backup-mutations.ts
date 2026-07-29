import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { backupApi } from '@/api'
import { backupQueryKeys } from '@/entities/backup/api/query-keys'

export function useBackupMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: backupQueryKeys.all })
  }

  const createBackup = useMutation({
    mutationFn: (type: 'db' | 'media' | 'all' = 'all') => backupApi.create(type),
    onSuccess: invalidate,
  })

  const restoreBackup = useMutation({
    mutationFn: (file: string) => backupApi.restore(file),
    onSuccess: invalidate,
  })

  const downloadBackup = useMutation({
    mutationFn: async (file: string) => {
      const blob = await backupApi.download(file)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = file
      a.click()
      URL.revokeObjectURL(url)
    },
  })

  const deleteBackup = useMutation({
    mutationFn: (file: string) => backupApi.delete(file),
    onSuccess: invalidate,
  })

  return { createBackup, restoreBackup, downloadBackup, deleteBackup }
}
