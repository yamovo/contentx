import { vi } from 'vitest'

Object.defineProperty(globalThis, 'scrollTo', {
  value: vi.fn(),
  configurable: true,
  writable: true,
})

if (typeof window !== 'undefined') {
  Object.defineProperty(window, 'scrollTo', {
    value: vi.fn(),
    configurable: true,
    writable: true,
  })
}
