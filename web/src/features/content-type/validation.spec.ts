import { describe, expect, it } from 'vitest'
import { emptyContentTypeDraft, validateContentTypeDraft } from './validation'

describe('content type validation', () => {
  it('rejects malformed and duplicate field names', () => {
    const draft = emptyContentTypeDraft()
    draft.uid = 'Product-Bad'
    draft.name = '产品'
    draft.fields = [
      { name: 'title', label: '标题', field_type: 'text', required: true, unique: false, optionsText: '' },
      { name: 'title', label: '副标题', field_type: 'text', required: false, unique: false, optionsText: '' },
    ]

    const result = validateContentTypeDraft(draft)
    expect(result.payload).toBeUndefined()
    expect(result.errors).toContain('字段名 title 重复')
  })

  it('normalizes fields and unique enum options', () => {
    const draft = emptyContentTypeDraft()
    draft.uid = 'product'
    draft.name = ' 产品 '
    draft.fields = [{
      name: 'status',
      label: '状态',
      field_type: 'enum',
      required: true,
      unique: false,
      optionsText: 'active, draft, active',
    }]

    const result = validateContentTypeDraft(draft)
    expect(result.errors).toEqual([])
    expect(result.payload?.fields[0]).toMatchObject({
      name: 'status',
      options: ['active', 'draft'],
      sort_order: 0,
    })
  })
})
