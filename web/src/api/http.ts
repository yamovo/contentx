import axios, { type AxiosInstance, type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import router from '@/router'

const config: AxiosRequestConfig = {
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
}

const http: AxiosInstance = axios.create(config)

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
  (error) => {
    const { response } = error

    if (!response) {
      ElMessage.error('网络错误，请检查连接')
      return Promise.reject(error)
    }

    switch (response.status) {
      case 401:
        // Try refresh token first.
        const authStore = useAuthStore()
        if (authStore.refreshToken && !error.config._retry) {
          error.config._retry = true
          return authStore.refreshAccessToken().then(() => {
            error.config.headers.Authorization = `Bearer ${authStore.token}`
            return http(error.config)
          }).catch(() => {
            authStore.logout()
            router.push('/login')
          })
        }
        authStore.logout()
        router.push('/login')
        break
      case 403:
        ElMessage.error('权限不足')
        break
      case 404:
        ElMessage.error('资源不存在')
        break
      case 422:
        ElMessage.error(response.data?.error || '数据验证失败')
        break
      case 429:
        ElMessage.warning('请求过于频繁，请稍后再试')
        break
      case 500:
        ElMessage.error('服务器内部错误')
        break
      default:
        ElMessage.error(response.data?.error || '请求失败')
    }

    return Promise.reject(error)
  }
)

export default http

// Typed API helpers.
export function get<T>(url: string, params?: Record<string, any>): Promise<T> {
  return http.get(url, { params }).then(r => r.data)
}

export function post<T>(url: string, data?: any): Promise<T> {
  return http.post(url, data).then(r => r.data)
}

export function put<T>(url: string, data?: any): Promise<T> {
  return http.put(url, data).then(r => r.data)
}

export function del<T>(url: string): Promise<T> {
  return http.delete(url).then(r => r.data)
}
