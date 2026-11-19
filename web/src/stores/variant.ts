import { computed, ref } from 'vue'
import { request } from '@/api/client'
import { protocolCatalog } from '@/utils/protocols'
import type { KernelType, ProxyNode } from '@/types'

type Variant = 'full' | 'lite' | 'mihomo' | 'singbox' | 'unknown'
export const variant = ref<Variant>('full')
const detected = ref<{ variant: Variant; kernels: KernelType[] }>()
export const kernels = computed<KernelType[]>(() => {
  if (detected.value?.variant === variant.value) return detected.value.kernels
  switch (variant.value) {
    case 'lite': case 'mihomo': return ['mihomo']
    case 'singbox': return ['sing-box']
    case 'full': return ['sing-box', 'mihomo']
    default: return []
  }
})
export const isLite = computed(() => kernels.value.length === 1 && kernels.value[0] === 'mihomo')
export const canSwitchKernel = computed(() => kernels.value.length > 1)
export const hasMihomo = computed(() => kernels.value.includes('mihomo'))

export async function loadVariant() {
  variant.value = 'unknown'
  detected.value = undefined
  try {
    const info = await request<{ variant: Variant; kernels?: KernelType[] }>({ url: '/auth/variant' })
    if (!['full', 'lite', 'mihomo', 'singbox'].includes(info.variant)) return
    if (info.kernels?.length) detected.value = { variant: info.variant, kernels: info.kernels }
    variant.value = info.variant
  } catch {
    variant.value = 'unknown'
  }
}

export function nodeUnavailableReason(node: Pick<ProxyNode, 'protocol' | 'owner'>): string {
  if (variant.value === 'unknown') return '无法读取版本能力，请刷新重试'
  if (canSwitchKernel.value) return ''
  const kernel = kernels.value[0]
  const owner = kernel === 'mihomo' ? 2 : 1
  const protocol = protocolCatalog.find(item => item.id === node.protocol)
  const supported = kernel && (node.owner & owner) && (
    node.protocol === 'proxy-chain' || (node.protocol === 'load-balance' ? kernel === 'mihomo' : protocol?.kernels.includes(kernel))
  )
  return supported ? '' : variant.value === 'lite' ? '精简版不支持' : `${kernel} 版不支持 ${node.protocol}`
}
