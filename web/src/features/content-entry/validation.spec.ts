import { describe, expect, it } from 'vitest'
import type { ContentField } from '@/api'
import { buildContentPayload, createInitialEntryData } from './validation'

const fields: ContentField[] = [
  { name: 'title', label: '标题', field_type: 'text', required: true, unique: false, min_length: 2 },
  { name: 'meta', label: '元数据', field_type: 'json', required: false, unique: false },
  { name: 'url', label: '链接', field_type: 'url', required: false, unique: false },
  { name: 'active', label: '启用', field_type: 'boolean', required: false, unique: false, default_value: 'true' },
]

describe('content entry validation', () => {
  it('initializes heterogeneous fields and formats JSON for editing', () => {
    expect(createInitialEntryData(fields, { meta: { source: 'test' } })).toEqual({
      title: undefined,
      meta: '{\n  "source": "test"\n}',
      url: undefined,
      active: true,
    })
  })

  it('returns structured field errors without showing UI side effects', () => {
    const result = buildContentPayload(fields, {
      title: 'x',
      meta: '{broken',
      url: 'javascript:alert(1)',
      active: true,
    })

    expect(result.errors.map(error => error.field)).toEqual(['title', 'meta', 'url'])
  })

  it('parses valid JSON into the request payload', () => {
    const result = buildContentPayload(fields, {
      title: 'Valid title',
      meta: '{"safe":true}',
      url: 'https://example.com/content',
      active: false,
    })

    expect(result.errors).toEqual([])
    expect(result.data).toEqual({
      title: 'Valid title',
      meta: { safe: true },
      url: 'https://example.com/content',
      active: false,
    })
  })
})
