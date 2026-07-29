import axios, {
  type AxiosError,
  type AxiosInstance,
  type AxiosRequestConfig,
  type AxiosResponse,
} from 'axios'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'
import {
  ApiRequestError,
  type ApiEnvelope,
  type ApiError,
} from '@/shared/api/types'

// _retry 是拦截器的内部标记：置为 true 的请求在 401 时不再走 refresh
// 重试流程（见下方响应拦截器与 authApi.refresh）。
declare module 'axios' {
  export interface AxiosRequestConfig {
    _retry?: boolean
  }
}

const config: AxiosRequestConfig = {
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
}

const http: AxiosInstance = axios.create(config)

// Token refresh state to prevent concurrent refresh attempts.
let isRefreshing = false
let failedQueue: Array<{
  resolve: (token: string) => void
  reject: (error: unknown) => void
}> = []

const processQueue = (error: unknown, token: string | null = null) => {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error)
    } else {
      prom.resolve(token!)
    }
  })
  failedQueue = []
}

const statusMessages: Record<number, string> = {
  400: '请求参数有误',
  401: '登录状态已失效',
  403: '当前账号没有执行此操作的权限',
  404: '请求的资源不存在',
  409: '数据状态已发生变化，请刷新后重试',
  422: '提交的数据未通过校验',
  429: '请求过于频繁，请稍后再试',
  500: '服务器内部错误',
}

export function normalizeApiError(error: unknown): ApiRequestError {
  const axiosError = error as AxiosError<ApiEnvelope<unknown> & { error?: string }>
  const response = axiosError.response
  const status = response?.status ?? 0
  const body = response?.data
  const headerRequestId = response?.headers?.['x-request-id']
  const requestId = typeof headerRequestId === 'string'
    ? headerRequestId
    : undefined

  const normalized: ApiError = {
    status,
    code: body?.err_code || (status ? `HTTP_${status}` : 'NETWORK_ERROR'),
    message: body?.message || body?.error || statusMessages[status] || (
      status ? '请求未能完成' : '网络连接失败，请检查连接后重试'
    ),
    requestId,
  }

  return new ApiRequestError(normalized)
}

function navigateToLogin() {
  if (router.currentRoute.value.path !== '/login') {
    void router.replace({
      path: '/login',
      query: { redirect: router.currentRoute.value.fullPath },
    })
  }
}

// Request interceptor: attach JWT token.
http.interceptors.request.use(
  (cfg) => {
    const authStore = useAuthStore()
    if (authStore.token) {
      cfg.headers.Authorization = `Bearer ${authStore.token}`
    }
    return cfg
  },
  (error) => Promise.reject(error)
)

// Response interceptor: handle errors globally.
http.interceptors.response.use(
  (response: AxiosResponse) => response,
  (error: AxiosError<ApiEnvelope<unknown>>) => {
    const { response } = error

    if (!response) {
      return Promise.reject(normalizeApiError(error))
    }

    const originalRequest = error.config

    // Handle 401 with token refresh queue to prevent concurrent refreshes.
    if (response.status === 401 && originalRequest && !originalRequest._retry) {
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject })
        }).then((token) => {
          originalRequest.headers.Authorization = `Bearer ${token}`
          return http(originalRequest)
        })
      }

      originalRequest._retry = true
      isRefreshing = true

      const authStore = useAuthStore()
      if (!authStore.refreshToken) {
        authStore.clearAuth()
        navigateToLogin()
        isRefreshing = false
        return Promise.reject(normalizeApiError(error))
      }

      return authStore
        .refreshAccessToken()
        .then((token) => {
          processQueue(null, token)
          originalRequest.headers.Authorization = `Bearer ${token}`
          return http(originalRequest)
        })
        .catch((err) => {
          processQueue(err, null)
          authStore.clearAuth()
          navigateToLogin()
          return Promise.reject(err)
        })
        .finally(() => {
          isRefreshing = false
        })
    }

    return Promise.reject(normalizeApiError(error))
  }
)

export default http

// Typed API helpers.
export function get<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  return http.get(url, { params }).then(r => r.data)
}

export function post<T>(url: string, data?: unknown): Promise<T> {
  return http.post(url, data).then(r => r.data)
}

export function put<T>(url: string, data?: unknown): Promise<T> {
  return http.put(url, data).then(r => r.data)
}

export function del<T>(url: string): Promise<T> {
  return http.delete(url).then(r => r.data)
}
