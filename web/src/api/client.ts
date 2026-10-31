import axios, { type AxiosError, type AxiosInstance, type AxiosRequestConfig } from 'axios'
import type { ApiErrorPayload, ApiResponse } from '@/types'

export class ApiError extends Error {
  readonly code: string
  readonly why?: string
  readonly fix?: ApiErrorPayload['fix']

  constructor(code: string, message: string, why?: string, fix?: ApiErrorPayload['fix']) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.why = why
    this.fix = fix
  }

  static fromPayload(payload?: ApiErrorPayload): ApiError {
    if (!payload) {
      return new ApiError('UNKNOWN_ERROR', '请求失败，服务端未返回错误详情')
    }
    return new ApiError(payload.code, payload.what || '请求失败', payload.why, payload.fix)
  }
}

const baseURL = import.meta.env.VITE_API_BASE ?? '/api'

export const http: AxiosInstance = axios.create({
  baseURL,
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json'
  }
})

export function unwrapResponse<T>(body: ApiResponse<T> | T): T {
  if (body && typeof body === 'object' && 'success' in body) {
    const envelope = body as ApiResponse<T>
    if (envelope.success) {
      return envelope.data as T
    }
    throw ApiError.fromPayload(envelope.error)
  }
  return body as T
}

http.interceptors.response.use(
  (response) => unwrapResponse(response.data),
  (error: AxiosError<ApiResponse<unknown>>) => {
    const payload = error.response?.data?.error
    if (payload) {
      return Promise.reject(ApiError.fromPayload(payload))
    }
    if (error.code === 'ECONNABORTED') {
      return Promise.reject(new ApiError('TIMEOUT', '请求超时，请检查后端服务是否运行'))
    }
    return Promise.reject(
      new ApiError('NETWORK_ERROR', '无法连接到后端服务', error.message)
    )
  }
)

export function request<T>(config: AxiosRequestConfig): Promise<T> {
  return http.request(config) as Promise<T>
}
