import { request } from './client'
import type { AppConfig } from '@/types'

export function getConfig(): Promise<AppConfig> {
  return request<AppConfig>({ url: '/config', method: 'get' })
}

export function updateConfig(config: AppConfig): Promise<AppConfig> {
  return request<AppConfig>({ url: '/config', method: 'put', data: config })
}

export function applyConfig(): Promise<AppConfig> {
  return request<AppConfig>({ url: '/config/apply', method: 'post' })
}
