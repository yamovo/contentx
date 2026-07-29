/**
 * Password policy — kept in sync with the backend (internal/auth/password.go).
 *
 * Backend rules (auth.ValidatePasswordStrength):
 *   - 8 <= len <= 72 (bcrypt limit)
 *   - at least one uppercase letter
 *   - at least one lowercase letter
 *   - at least one digit
 *
 * This module mirrors those rules so the frontend rejects weak passwords
 * before round-tripping to the API, avoiding UX mismatch.
 */

export const MIN_PASSWORD_LENGTH = 8
export const MAX_PASSWORD_LENGTH = 72

export const PASSWORD_RULE_HINT = '至少 8 位，需含大写字母、小写字母和数字'

/** A weak-password blacklist of the most common trivial passwords. */
const COMMON_WEAK_PASSWORDS = new Set([
  'password1', 'Password1', '12345678', 'qwerty123', 'Abc12345',
  'Admin123', 'Admin1234', 'Welcome1', 'Welcome123', 'P@ssw0rd',
])

export interface PasswordValidationResult {
  valid: boolean
  message?: string
}

/**
 * validatePasswordStrength checks a password against the same rules as the
 * backend. Returns { valid: true } on success or { valid: false, message }.
 */
export function validatePasswordStrength(password: string): PasswordValidationResult {
  if (!password) {
    return { valid: false, message: '请输入密码' }
  }
  if (password.length < MIN_PASSWORD_LENGTH) {
    return { valid: false, message: `密码至少 ${MIN_PASSWORD_LENGTH} 位` }
  }
  if (password.length > MAX_PASSWORD_LENGTH) {
    return { valid: false, message: `密码不能超过 ${MAX_PASSWORD_LENGTH} 位` }
  }
  if (!/[A-Z]/.test(password)) {
    return { valid: false, message: '密码需包含至少一个大写字母' }
  }
  if (!/[a-z]/.test(password)) {
    return { valid: false, message: '密码需包含至少一个小写字母' }
  }
  if (!/[0-9]/.test(password)) {
    return { valid: false, message: '密码需包含至少一个数字' }
  }
  if (COMMON_WEAK_PASSWORDS.has(password)) {
    return { valid: false, message: '密码过于常见，请使用更复杂的密码' }
  }
  return { valid: true }
}

/**
 * passwordStrengthScore returns a 0-4 score for UI strength meters.
 * Mirrors the heuristic in auth.PasswordStrengthScore (backend).
 */
export function passwordStrengthScore(password: string): number {
  let score = 0
  if (password.length >= 8) score++
  if (password.length >= 12) score++
  if (/[A-Z]/.test(password) && /[a-z]/.test(password)) score++
  if (/[0-9]/.test(password)) score++
  if (/[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?]/.test(password)) score++
  return Math.min(score, 4)
}
