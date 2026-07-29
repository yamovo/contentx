import type { ContentField } from '@/api'
import type { ContentEntryData, ContentEntryValue } from '@/entities/content/model'

export interface ContentValidationError {
  field: string
  label: string
  message: string
}

export interface ContentPayloadResult {
  data: Record<string, unknown>
  errors: ContentValidationError[]
}

export function createInitialEntryData(
  fields: ContentField[],
  existing: Record<string, unknown> = {},
): ContentEntryData {
  const data: ContentEntryData = {}

  for (const field of sortedContentFields(fields)) {
    const value = existing[field.name]
    if (field.field_type === 'json' && value !== undefined && typeof value === 'object') {
      data[field.name] = JSON.stringify(value, null, 2)
    } else if (value !== undefined) {
      data[field.name] = value as ContentEntryValue
    } else if (field.field_type === 'boolean') {
      data[field.name] = field.default_value === 'true'
    } else {
      data[field.name] = field.default_value || undefined
    }
  }

  return data
}

export function buildContentPayload(
  fields: ContentField[],
  source: ContentEntryData,
): ContentPayloadResult {
  const data: Record<string, unknown> = {}
  const errors: ContentValidationError[] = []

  for (const field of sortedContentFields(fields)) {
    let value = source[field.name]

    if (field.field_type === 'json' && typeof value === 'string' && value.trim()) {
      try {
        value = JSON.parse(value) as Record<string, unknown>
      } catch {
        errors.push(errorFor(field, '请输入合法的 JSON'))
        continue
      }
    }

    if (field.required && isEmpty(value)) {
      errors.push(errorFor(field, '此字段为必填项'))
      continue
    }

    if (typeof value === 'string') {
      if (field.min_length != null && value.length < field.min_length) {
        errors.push(errorFor(field, `至少输入 ${field.min_length} 个字符`))
        continue
      }
      if (field.max_length != null && value.length > field.max_length) {
        errors.push(errorFor(field, `最多输入 ${field.max_length} 个字符`))
        continue
      }
      if (field.field_type === 'email' && value && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
        errors.push(errorFor(field, '请输入有效的邮箱地址'))
        continue
      }
      if (field.field_type === 'url' && value && !isValidURL(value)) {
        errors.push(errorFor(field, '请输入有效的 http(s) 地址'))
        continue
      }
    }

    if (typeof value === 'number') {
      if (field.min_value != null && value < field.min_value) {
        errors.push(errorFor(field, `不能小于 ${field.min_value}`))
        continue
      }
      if (field.max_value != null && value > field.max_value) {
        errors.push(errorFor(field, `不能大于 ${field.max_value}`))
        continue
      }
    }

    if (value !== undefined) {
      data[field.name] = value
    }
  }

  return { data, errors }
}

export function sortedContentFields(fields: ContentField[]): ContentField[] {
  return [...fields].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
}

function isEmpty(value: ContentEntryValue): boolean {
  return value === undefined || value === null || value === ''
}

function errorFor(field: ContentField, message: string): ContentValidationError {
  return { field: field.name, label: field.label, message }
}

function isValidURL(value: string): boolean {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

