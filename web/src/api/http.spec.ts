import { describe, it, expect, beforeEach, vi } from 'vitest'

// Capture the fake axios instance created by http.ts. vi.hoisted is required
// because vi.mock factories are hoisted above regular const declarations.
// The instance must be callable (vi.fn()) because http.ts invokes it as
// `http(originalRequest)` when retrying after a token refresh.
const { fakeInstance, mockAuthStore } = vi.hoisted(() => {
  const instance = Object.assign(vi.fn(), {
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  })
  return {
    fakeInstance: instance,
    mockAuthStore: {
      token: 'test-access-token',
      refreshToken: '',
      logout: vi.fn(),
      clearAuth: vi.fn(),
      refreshAccessToken: vi.fn(),
    },
  }
})

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
import router from '@/router'

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

    it('clears auth and redirects on 401 without refresh token', async () => {
      const error = { response: { status: 401, data: {} }, config: { headers: {} } }
      await expect(onResponseError(error)).rejects.toBe(error)
      expect(mockAuthStore.clearAuth).toHaveBeenCalled()
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

  describe('401 token refresh queue', () => {
    let refreshResolve!: (token: string) => void
    let refreshReject!: (err: any) => void
    let refreshPromise: Promise<string>

    beforeEach(() => {
      vi.clearAllMocks()
      mockAuthStore.token = 'expired-token'
      mockAuthStore.refreshToken = 'valid-refresh-token'

      // Controllable refresh promise so tests can resolve/reject it on demand.
      refreshPromise = new Promise((resolve, reject) => {
        refreshResolve = resolve
        refreshReject = reject
      })
      mockAuthStore.refreshAccessToken.mockReturnValue(refreshPromise)

      // By default, retry succeeds with a fresh response.
      fakeInstance.mockResolvedValue({ data: { ok: true } })
    })

    it('refreshes the token and retries the original request on 401', async () => {
      const error = {
        response: { status: 401, data: {} },
        config: { headers: {} as Record<string, string> },
      }

      const pending = onResponseError(error)
      // While refresh is in flight, isRefreshing should be true.
      expect(mockAuthStore.refreshAccessToken).toHaveBeenCalledTimes(1)

      refreshResolve('new-access-token')
      const result = await pending

      expect(result).toEqual({ data: { ok: true } })
      expect(error.config.headers.Authorization).toBe('Bearer new-access-token')
      expect(fakeInstance).toHaveBeenCalledWith(error.config)
    })

    it('clears auth and redirects to /login when refresh fails', async () => {
      const error = {
        response: { status: 401, data: {} },
        config: { headers: {} as Record<string, string> },
      }

      const pending = onResponseError(error)
      refreshReject(new Error('refresh expired'))

      await expect(pending).rejects.toThrow('refresh expired')
      expect(mockAuthStore.clearAuth).toHaveBeenCalled()
      expect(router.push).toHaveBeenCalledWith('/login')
    })

    it('queues concurrent 401s and replays them with the new token', async () => {
      const error1 = {
        response: { status: 401, data: {} },
        config: { headers: {} as Record<string, string> },
      }
      const error2 = {
        response: { status: 401, data: {} },
        config: { headers: {} as Record<string, string> },
      }

      // First 401 kicks off the refresh; isRefreshing becomes true.
      const pending1 = onResponseError(error1)
      expect(mockAuthStore.refreshAccessToken).toHaveBeenCalledTimes(1)

      // Second 401 arrives while refresh is in flight — it should queue,
      // NOT trigger a second refresh.
      const pending2 = onResponseError(error2)
      expect(mockAuthStore.refreshAccessToken).toHaveBeenCalledTimes(1)

      // Resolve the refresh — both queued requests should replay.
      refreshResolve('shared-new-token')
      const [r1, r2] = await Promise.all([pending1, pending2])

      expect(r1).toEqual({ data: { ok: true } })
      expect(r2).toEqual({ data: { ok: true } })
      expect(error1.config.headers.Authorization).toBe('Bearer shared-new-token')
      expect(error2.config.headers.Authorization).toBe('Bearer shared-new-token')
      expect(fakeInstance).toHaveBeenCalledTimes(2)
    })

    it('rejects queued requests when refresh fails', async () => {
      const error1 = {
        response: { status: 401, data: {} },
        config: { headers: {} as Record<string, string> },
      }
      const error2 = {
        response: { status: 401, data: {} },
        config: { headers: {} as Record<string, string> },
      }

      const pending1 = onResponseError(error1)
      const pending2 = onResponseError(error2)
      expect(mockAuthStore.refreshAccessToken).toHaveBeenCalledTimes(1)

      refreshReject(new Error('refresh failed'))

      await expect(pending1).rejects.toThrow('refresh failed')
      await expect(pending2).rejects.toThrow('refresh failed')
      expect(mockAuthStore.clearAuth).toHaveBeenCalled()
      expect(router.push).toHaveBeenCalledWith('/login')
    })

    it('does not refresh when _retry is already set on the request', async () => {
      const error = {
        response: { status: 401, data: {} },
        config: { _retry: true, headers: {} as Record<string, string> },
      }

      await expect(onResponseError(error)).rejects.toBe(error)
      expect(mockAuthStore.refreshAccessToken).not.toHaveBeenCalled()
    })
  })
})
