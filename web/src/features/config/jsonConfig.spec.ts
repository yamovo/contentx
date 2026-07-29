import { describe, expect, it } from 'vitest'
import { formatConfig, parseConfig } from './jsonConfig'

describe('JSON config helpers', () => {
  it('accepts object configuration and formats it deterministically', () => {
    const parsed = parseConfig('{"enabled":true,"retries":3}')
    expect(parsed.error).toBeUndefined()
    expect(formatConfig(parsed.config)).toContain('"retries": 3')
  })

  it('rejects arrays, primitives and malformed JSON', () => {
    expect(parseConfig('[]').error).toBeTruthy()
    expect(parseConfig('"value"').error).toBeTruthy()
    expect(parseConfig('{bad').error).toBeTruthy()
  })
})
