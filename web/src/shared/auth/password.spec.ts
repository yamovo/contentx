import { describe, expect, it } from 'vitest'
import {
  MAX_PASSWORD_LENGTH,
  passwordStrengthScore,
  validatePasswordStrength,
} from './password'

describe('password policy', () => {
  it.each([
    ['', '请输入密码'],
    ['Short1', '密码至少 8 位'],
    ['a'.repeat(MAX_PASSWORD_LENGTH + 1), `密码不能超过 ${MAX_PASSWORD_LENGTH} 位`],
    ['lowercase1', '密码需包含至少一个大写字母'],
    ['UPPERCASE1', '密码需包含至少一个小写字母'],
    ['NoDigitsHere', '密码需包含至少一个数字'],
    ['Password1', '密码过于常见，请使用更复杂的密码'],
  ])('rejects invalid password %j', (password, message) => {
    expect(validatePasswordStrength(password)).toEqual({ valid: false, message })
  })

  it('accepts a non-trivial password matching the backend policy', () => {
    expect(validatePasswordStrength('ContentX2026!')).toEqual({ valid: true })
  })

  it.each([
    ['', 0],
    ['abcdefgh', 1],
    ['Abcdefgh', 2],
    ['Abcdefgh1', 3],
    ['Abcdefgh1!', 4],
    ['Abcdefghijkl1!', 4],
  ])('scores %j as %d', (password, score) => {
    expect(passwordStrengthScore(password)).toBe(score)
  })
})
