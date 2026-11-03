import axios, { type AxiosError, type AxiosInstance, type AxiosRequestConfig } from 'axios'
import type { ApiErrorPayload, ApiResponse } from '@/types'
import { readLanguage } from '@/stores/appearance'
import { translateBackendMessage } from '@/utils/backendMessages'

const englishErrors: Record<string, string> = {
  AUTH_SETUP_REQUIRED: 'Create an administrator account first',
  AUTH_REQUIRED: 'Log in first',
  AUTH_INVALID: 'Incorrect username or password',
  METHOD_NOT_ALLOWED: 'Request method is not supported',
  RATE_LIMITED: 'Too many failed login attempts; try again later',
  INVALID_REQUEST: 'Invalid request parameters',
  NOT_FOUND: 'Resource not found',
  INTERNAL_ERROR: 'Internal server error',
  KERNEL_CHECK_FAILED: 'Configuration validation failed',
  PROXY_GROUP_UNAVAILABLE: 'The current kernel does not provide proxy group status',
  LOG_STREAM_UNAVAILABLE: 'Live log streaming is unavailable',
  UPDATE_UNAVAILABLE: 'Update service is unavailable',
  UPDATE_CHECK_FAILED: 'Could not check for updates',
  UPDATE_FAILED: 'Could not install the update',
  UPDATE_IN_PROGRESS: 'An update is already in progress',
  NODE_ACTIVE: 'Stop the active proxy before deleting these nodes',
  SOCKS_NODE_ASSIGNED: 'Remove the SOCKS5 port binding before deleting this node'
}
const localMessage = (zh: string, en: string) => readLanguage() === 'en' ? en : zh

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
      return new ApiError('UNKNOWN_ERROR', localMessage('请求失败，服务端未返回错误详情', 'Request failed; the server returned no error details'))
    }
    const message = readLanguage() === 'en' ? (englishErrors[payload.code] ?? payload.what) : payload.what
    return new ApiError(payload.code, translateBackendMessage(message) || localMessage('请求失败', 'Request failed'), payload.why ? translateBackendMessage(payload.why) : undefined, payload.fix)
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
      return Promise.reject(new ApiError('TIMEOUT', localMessage('请求超时，请检查后端服务是否运行', 'Request timed out; check that the backend is running')))
    }
    return Promise.reject(
      new ApiError('NETWORK_ERROR', localMessage('无法连接到后端服务', 'Unable to connect to the backend'), error.message)
    )
  }
)

export function request<T>(config: AxiosRequestConfig): Promise<T> {
  return http.request(config) as Promise<T>
}
