import { request } from './client'
import type { AppConfig, ApplyConfigRequest, ConfigurationState, ChainPlan, ProxyGroupStatus } from '@/types'

export function getConfig(): Promise<AppConfig> {
  return request<AppConfig>({ url: '/config', method: 'get' })
}

export function updateConfig(config: AppConfig): Promise<AppConfig> {
  return request<AppConfig>({ url: '/config', method: 'put', data: config })
}

export function applyConfig(config?: ApplyConfigRequest): Promise<ConfigurationState> {
  return request<ConfigurationState>({ url: '/config/apply', method: 'post', data: config, timeout: 60000 })
}

export function getCurrentConfig(): Promise<ConfigurationState> {
  return request<ConfigurationState>({ url: '/config/current', method: 'get' })
}

export function getProxyGroupStatus(): Promise<ProxyGroupStatus> {
  return request<ProxyGroupStatus>({ url: '/config/proxy-group', method: 'get' })
}

export function previewChain(nodeIds: string[]): Promise<ChainPlan> {
  return request<ChainPlan>({ url: '/config/chain/preview', method: 'post', data: { node_ids: nodeIds } })
}

export function exportRouting(): Promise<{ text: string }> {
  return request<{ text: string }>({ url: '/routing/export', method: 'get' })
}

export interface RoutingTextResult { rules: AppConfig['routing_rules']; errors?: { line: number; reason: string }[] }

export function validateRoutingText(text: string): Promise<RoutingTextResult> {
  return request<RoutingTextResult>({ url: '/routing/validate', method: 'post', data: { text } })
}

export function importRoutingText(text: string): Promise<RoutingTextResult> {
  return request<RoutingTextResult>({ url: '/routing/import', method: 'post', data: { text } })
}
