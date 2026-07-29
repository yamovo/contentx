import type { ContentField } from '@/api'

export interface ContentFieldDraft {
  name: string
  label: string
  field_type: string
  required: boolean
  unique: boolean
  optionsText: string
}

export interface ContentTypeDraft {
  uid: string
  name: string
  description: string
  is_single: boolean
  draft_publish: boolean
  fields: ContentFieldDraft[]
}

export interface ContentTypeCreatePayload {
  uid: string
  name: string
  description?: string
  is_single: boolean
  draft_publish: boolean
  fields: ContentField[]
}

export interface ContentTypeValidationResult {
  payload?: ContentTypeCreatePayload
  errors: string[]
}

