import { request } from './client'

export interface AuthStatus {
  configured: boolean
  authenticated: boolean
  username?: string
}

export function getAuthStatus(): Promise<AuthStatus> {
  return request<AuthStatus>({ url: '/auth/status', method: 'get' })
}

export function register(username: string, password: string): Promise<{ registered: boolean }> {
  return request<{ registered: boolean }>({ url: '/auth/register', method: 'post', data: { username, password } })
}

export function login(username: string, password: string): Promise<{ authenticated: boolean; username: string }> {
  return request<{ authenticated: boolean; username: string }>({ url: '/auth/login', method: 'post', data: { username, password } })
}

export function logout(): Promise<{ authenticated: boolean }> {
  return request<{ authenticated: boolean }>({ url: '/auth/logout', method: 'post' })
}
