import { contentApi } from '@/api'

export const contentQueryKeys = {
  all: ['content'] as const,
  type: (uid: string) => [...contentQueryKeys.all, 'type', uid] as const,
  entries: (uid: string, params: Record<string, unknown>) =>
    [...contentQueryKeys.all, 'entries', uid, params] as const,
  translations: (uid: string, documentId: string) =>
    [...contentQueryKeys.all, 'translations', uid, documentId] as const,
}

export async function queryContentType(uid: string) {
  return (await contentApi.getType(uid)).data
}

export async function queryContentEntries(uid: string, params: Record<string, unknown>) {
  return contentApi.listEntries(uid, params)
}

export async function queryContentTranslations(uid: string, documentId: string) {
  return (await contentApi.listTranslations(uid, documentId)).data || []
}

