import { computed, ref } from 'vue'
import { request } from '@/api/client'
import { protocolCatalog } from '@/utils/protocols'
import type { ProxyNode } from '@/types'

export const variant = ref<'full' | 'lite' | 'unknown'>('full')
export const isLite = computed(() => variant.value === 'lite')

// Loaded before mounting: the same compiled panel serves both Go targets.
export async function loadVariant() {
  variant.value = 'unknown'
  try {
    const info = await request<{ variant: 'full' | 'lite' }>({ url: '/auth/variant' })
    if (info.variant !== 'full' && info.variant !== 'lite') throw new Error('未知构建变体')
    variant.value = info.variant
  } catch {
    // Fail closed: no node activation until capabilities can be read.
    variant.value = 'unknown'
  }
}

export function nodeUnavailableReason(node: Pick<ProxyNode, 'protocol' | 'owner'>): string {
  if (variant.value === 'unknown') return '无法读取版本能力，请刷新重试'
  if (!isLite.value) return ''
	if (node.protocol === 'proxy-chain' || node.protocol === 'load-balance') return (node.owner & 2) ? '' : '精简版不支持'
  const protocol = protocolCatalog.find(item => item.id === node.protocol)
  return !(node.owner & 2) || !protocol?.kernels.includes('mihomo') ? '精简版不支持' : ''
}
