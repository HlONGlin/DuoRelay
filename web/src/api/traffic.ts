import { request } from './client'
import type { TrafficSnapshot } from '@/types'

export function getTraffic(): Promise<TrafficSnapshot> {
  return request<TrafficSnapshot>({ url: '/kernel/traffic', method: 'get' })
}

export function getTrafficHistory(limit = 60): Promise<TrafficSnapshot[]> {
  return request<TrafficSnapshot[]>({ url: '/kernel/traffic/history', method: 'get', params: { limit } })
}
