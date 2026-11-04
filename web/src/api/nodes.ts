import { request } from './client'
import type { ProxyNode } from '@/types'

export interface NodeQuery {
  protocol?: string
  subscription_id?: string
  search?: string
  limit?: number
  offset?: number
}

export function listNodes(params: NodeQuery = {}): Promise<ProxyNode[]> {
  return request<ProxyNode[]>({ url: '/nodes', method: 'get', params })
}

export interface ImportNodesResult {
  nodes: ProxyNode[]
  nodes_total: number
  duplicates?: number
  skipped?: { line: number; reason: string }[]
}

export function updateNode(id: string, data: Record<string, unknown>): Promise<ProxyNode> {
  return request<ProxyNode>({ url: `/nodes/${encodeURIComponent(id)}`, method: 'put', data })
}

export function importNodes(content: string): Promise<ImportNodesResult> {
  return request<ImportNodesResult>({
    url: '/nodes/import',
    method: 'post',
    data: { content }
  })
}

export function activateNode(id: string): Promise<ProxyNode> {
  return request<ProxyNode>({ url: `/nodes/${encodeURIComponent(id)}/activate`, method: 'post' })
}

export interface DelayResult {
  delay_ms: number
  success?: boolean
}

export function testNodeDelay(id: string, timeout = 5000): Promise<DelayResult> {
  return request<DelayResult>({
    url: `/nodes/${encodeURIComponent(id)}/test`,
    method: 'post',
    data: { timeout }
  })
}

export function deleteNode(id: string): Promise<void> {
  return request<void>({ url: `/nodes/${encodeURIComponent(id)}`, method: 'delete' })
}

export function deleteNodes(ids: string[]): Promise<{ deleted: number }> {
  return request<{ deleted: number }>({
    url: '/nodes/batch-delete',
    method: 'post',
    data: { node_ids: ids }
  })
}
