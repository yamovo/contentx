export type { ContentEntry, ContentField, ContentType } from '@/api'

export type ContentEntryValue = string | number | boolean | null | undefined | Record<string, unknown> | unknown[]
export type ContentEntryData = Record<string, ContentEntryValue>

