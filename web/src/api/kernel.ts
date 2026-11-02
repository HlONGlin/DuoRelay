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
  return request<KernelLog[]>({ url: '/kernel/logs', method: 'get', params: { limit: 200, ...query } })
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
