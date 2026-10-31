import { request } from './client'
import type { ConnectionsSnapshot, KernelLog, KernelStatus, KernelType } from '@/types'

export function getKernelStatus(): Promise<KernelStatus> {
  return request<KernelStatus>({ url: '/kernel/status', method: 'get' })
}

export function startKernel(kernel?: KernelType): Promise<KernelStatus> {
  return request<KernelStatus>({ url: '/kernel/start', method: 'post', data: { kernel } })
}

export function stopKernel(): Promise<KernelStatus> {
  return request<KernelStatus>({ url: '/kernel/stop', method: 'post' })
}

export function getKernelLogs(limit = 200): Promise<KernelLog[]> {
  return request<KernelLog[]>({ url: '/kernel/logs', method: 'get', params: { limit } })
}

export function getConnections(): Promise<ConnectionsSnapshot> {
  return request<ConnectionsSnapshot>({ url: '/kernel/connections', method: 'get' })
}
