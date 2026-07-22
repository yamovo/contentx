import { describe, it, expect, beforeEach, vi } from 'vitest'

// Capture the fake axios instance created by http.ts. vi.hoisted is required
// because vi.mock factories are hoisted above regular const declarations.
const { fakeInstance, mockAuthStore } = vi.hoisted(() => ({
  fakeInstance: {
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
  mockAuthStore: {
    token: 'test-access-token',
    refreshToken: '',
    logout: vi.fn(),
    refreshAccessToken: vi.fn(),
  },
}))

vi.mock('axios', () => ({
  default: {
    create: vi.fn(() => fakeInstance),
  },
}))

// Mock auth store used by interceptors.
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => mockAuthStore,
}))

// Mock element-plus message.
vi.mock('element-plus', () => ({
  ElMessage: {
    error: vi.fn(),
    warning: vi.fn(),
  },
}))

// Mock router.
vi.mock('@/router', () => ({
  default: { push: vi.fn() },
}))

import http, { get, post, put, del } from '@/api/http'
import { ElMessage } from 'element-plus'
import axios from 'axios'

// Capture interceptors and create() args registered at module load time.
// NOTE: these must be captured before any clearAllMocks() call wipes the records.
const createCallArgs = vi.mocked(axios.create).mock.calls[0]?.[0]
const requestUseCalls = fakeInstance.interceptors.request.use.mock.calls
const responseUseCalls = fakeInstance.interceptors.response.use.mock.calls
const onRequest = requestUseCalls[0]?.[0] as (cfg: any) => any
const onResponseSuccess = responseUseCalls[0]?.[0] as (resp: any) => any
const onResponseError = responseUseCalls[0]?.[1] as (err: any) => Promise<any>

describe('http client', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAuthStore.token = 'test-access-token'
    mockAuthStore.refreshToken = ''
  })

  it('creates axios instance with correct base config', () => {
    expect(createCallArgs).toMatchObject({
      baseURL: '/api/v1',
      timeout: 30000,
    })
  })

  it('registers request and response interceptors', () => {
    expect(requestUseCalls).toHaveLength(1)
    expect(responseUseCalls).toHaveLength(1)
  })

  describe('request interceptor', () => {
    it('attaches bearer token when available', () => {
      const cfg = { headers: {} as Record<string, string> }
      const result = onRequest(cfg)
      expect(result.headers.Authorization).toBe('Bearer test-access-token')
    })

    it('skips header when no token', () => {
      mockAuthStore.token = ''
      const cfg = { headers: {} as Record<string, string> }
      const result = onRequest(cfg)
      expect(result.headers.Authorization).toBeUndefined()
    })
  })

  describe('response interceptor', () => {
    it('passes through successful responses', () => {
      const resp = { status: 200, data: { code: 0 } }
      expect(onResponseSuccess(resp)).toBe(resp)
    })

    it('shows network error message when no response', async () => {
      const error = { response: undefined, config: {} }
      await expect(onResponseError(error)).rejects.toBe(error)
      expect(ElMessage.error).toHaveBeenCalledWith('网络错误，请检查连接')
    })

    it('shows 403 permission message', async () => {
      const error = { response: { status: 403, data: {} }, config: {} }
      await expect(onResponseError(error)).rejects.toBe(error)
      expect(ElMessage.error).toHaveBeenCalledWith('权限不足')
    })

    it('shows 404 message', async () => {
      const error = { response: { status: 404, data: {} }, config: {} }
      await expect(onResponseError(error)).rejects.toBe(error)
      expect(ElMessage.error).toHaveBeenCalledWith('资源不存在')
    })

    it('shows 429 warning', async () => {
      const error = { response: { status: 429, data: {} }, config: {} }
      await expect(onResponseError(error)).rejects.toBe(error)
      expect(ElMessage.warning).toHaveBeenCalledWith('请求过于频繁，请稍后再试')
    })

    it('shows 500 server error', async () => {
      const error = { response: { status: 500, data: {} }, config: {} }
      await expect(onResponseError(error)).rejects.toBe(error)
      expect(ElMessage.error).toHaveBeenCalledWith('服务器内部错误')
    })

    it('logs out and redirects on 401 without refresh token', async () => {
      const error = { response: { status: 401, data: {} }, config: { headers: {} } }
      await expect(onResponseError(error)).rejects.toBe(error)
      expect(mockAuthStore.logout).toHaveBeenCalled()
    })
  })

  describe('typed helpers', () => {
    it('get/post/put/del unwrap response data', async () => {
      fakeInstance.get.mockResolvedValue({ data: { items: [] } })
      fakeInstance.post.mockResolvedValue({ data: { id: 1 } })
      fakeInstance.put.mockResolvedValue({ data: { ok: true } })
      fakeInstance.delete.mockResolvedValue({ data: { deleted: true } })

      await expect(get('/things')).resolves.toEqual({ items: [] })
      await expect(post('/things', { a: 1 })).resolves.toEqual({ id: 1 })
      await expect(put('/things/1', { a: 2 })).resolves.toEqual({ ok: true })
      await expect(del('/things/1')).resolves.toEqual({ deleted: true })

      expect(fakeInstance.get).toHaveBeenCalledWith('/things', { params: undefined })
    })
  })

  it('exports the axios instance as default', () => {
    expect(http).toBe(fakeInstance)
  })
})
