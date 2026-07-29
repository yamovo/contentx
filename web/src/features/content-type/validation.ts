import type {
  ContentFieldDraft,
  ContentTypeDraft,
  ContentTypeValidationResult,
} from './model'

export const contentFieldTypes = [
  { value: 'text', label: '文本' },
  { value: 'rich_text', label: 'Markdown' },
  { value: 'integer', label: '整数' },
  { value: 'float', label: '小数' },
  { value: 'boolean', label: '布尔' },
  { value: 'date', label: '日期' },
  { value: 'enum', label: '枚举' },
  { value: 'email', label: '邮箱' },
  { value: 'url', label: 'URL' },
  { value: 'slug', label: 'Slug' },
  { value: 'json', label: 'JSON' },
  { value: 'media', label: '媒体' },
  { value: 'relation', label: '关联' },
] as const

export function emptyContentField(): ContentFieldDraft {
  return {
    name: '',
    label: '',
    field_type: 'text',
    required: false,
    unique: false,
    optionsText: '',
  }
}

export function emptyContentTypeDraft(): ContentTypeDraft {
  return {
    uid: '',
    name: '',
    description: '',
    is_single: false,
    draft_publish: true,
    fields: [emptyContentField()],
  }
}

export function validateContentTypeDraft(draft: ContentTypeDraft): ContentTypeValidationResult {
  const errors: string[] = []
  const uid = draft.uid.trim()
  const name = draft.name.trim()

  if (!uid) errors.push('请填写 UID')
  if (uid && !/^[a-z][a-z0-9_]*$/.test(uid)) {
    errors.push('UID 只能包含小写字母、数字和下划线，且以字母开头')
  }
  if (!name) errors.push('请填写名称')

  const completed = draft.fields.filter(field => field.name.trim() || field.label.trim())
  if (!completed.length) errors.push('至少定义一个字段')

  const names = new Set<string>()
  for (const [index, field] of completed.entries()) {
    const fieldName = field.name.trim()
    const label = field.label.trim()
    const prefix = `第 ${index + 1} 个字段`
    if (!fieldName || !label) {
      errors.push(`${prefix}需同时填写字段名和标签`)
      continue
    }
    if (!/^[a-z][a-z0-9_]*$/.test(fieldName)) {
      errors.push(`${prefix}的字段名格式无效`)
    }
    if (names.has(fieldName)) {
      errors.push(`字段名 ${fieldName} 重复`)
    }
    names.add(fieldName)
    if (field.field_type === 'enum' && !enumOptions(field).length) {
      errors.push(`${prefix}的枚举值不能为空`)
    }
  }

  if (errors.length) return { errors }

  return {
    errors: [],
    payload: {
      uid,
      name,
      description: draft.description.trim() || undefined,
      is_single: draft.is_single,
      draft_publish: draft.draft_publish,
      fields: completed.map((field, index) => ({
        name: field.name.trim(),
        label: field.label.trim(),
        field_type: field.field_type,
        required: field.required,
        unique: field.unique,
        options: field.field_type === 'enum' ? enumOptions(field) : undefined,
        sort_order: index,
      })),
    },
  }
}

function enumOptions(field: ContentFieldDraft): string[] {
  return [...new Set(field.optionsText
    .split(/[,，]/)
    .map(value => value.trim())
    .filter(Boolean))]
}

