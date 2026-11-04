import { request } from './client'
import type {
  ConnectionsSnapshot,
  KernelLog,
  KernelLogQuery,
  KernelLogSummary,
  KernelStatus,
  KernelType
} from '@/types'

export function getKernelStatus(): Promise<KernelStatus> {
  return request<KernelStatus>({ url: '/kernel/status', method: 'get' })
}

export function startKernel(kernel?: KernelType): Promise<KernelStatus> {
  return request<KernelStatus>({ url: '/kernel/start', method: 'post', data: { kernel } })
}

export function stopKernel(): Promise<KernelStatus> {
  return request<KernelStatus>({ url: '/kernel/stop', method: 'post' })
}

export function getKernelLogs(query: KernelLogQuery = {}): Promise<KernelLog[]> {
  return request<KernelLog[]>({ url: '/kernel/logs', method: 'get', params: { limit: 100, ...query } })
}

export function getKernelLogSummary(query: KernelLogQuery = {}): Promise<KernelLogSummary> {
  return request<KernelLogSummary>({ url: '/kernel/logs/summary', method: 'get', params: query })
}

export function openKernelLogStream(query: KernelLogQuery = {}): WebSocket {
  const url = new URL('/ws/logs', window.location.href)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== '') url.searchParams.set(key, String(value))
  }
  return new WebSocket(url)
}

export function getConnections(): Promise<ConnectionsSnapshot> {
  return request<ConnectionsSnapshot>({ url: '/kernel/connections', method: 'get' })
}

export interface LogPolicy {
  retention_days: number
  cleanup_hours: number
  last_cleanup?: string
  last_error?: string
}
export function getLogPolicy(): Promise<LogPolicy> {
  return request<LogPolicy>({ url: '/kernel/logs/policy' })
}
export function saveLogPolicy(data: LogPolicy): Promise<LogPolicy> {
  return request<LogPolicy>({ url: '/kernel/logs/policy', method: 'put', data: { retention_days: data.retention_days, cleanup_hours: data.cleanup_hours } })
}
export function clearKernelLogs(kernel?: KernelType): Promise<void> {
  return request<void>({ url: '/kernel/logs', method: 'delete', params: { kernel } })
}
