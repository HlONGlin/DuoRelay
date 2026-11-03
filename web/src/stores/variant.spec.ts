import { afterEach, describe, expect, it, vi } from 'vitest'
import { loadVariant, nodeUnavailableReason, variant } from './variant'
import { protocolCatalog } from '@/utils/protocols'
import { request } from '@/api/client'

vi.mock('@/api/client', () => ({ request: vi.fn() }))
afterEach(() => { variant.value = 'full' })

describe('build capabilities', () => {
  it('retains all 28 protocols in full and exactly 24 in lite', () => {
    for (const protocol of protocolCatalog) {
      expect(nodeUnavailableReason({ protocol: protocol.id, owner: 3 })).toBe('')
    }
    variant.value = 'lite'
    const enabled = protocolCatalog.filter(p => !nodeUnavailableReason({ protocol: p.id, owner: 3 }))
    expect(enabled).toHaveLength(24)
    expect(protocolCatalog.filter(p => !enabled.includes(p)).map(p => p.id)).toEqual(['shadowtls', 'naive', 'tor', 'openconnect'])
    for (const protocol of ['shadowtls', 'naive', 'tor', 'openconnect'] as const) {
      expect(nodeUnavailableReason({ protocol, owner: 3 })).toBe('精简版不支持')
    }
    expect(nodeUnavailableReason({ protocol: 'http', owner: 1 })).toBe('精简版不支持')
  })
  it('reads backend capabilities before enabling activation and fails closed', async () => {
    vi.mocked(request).mockResolvedValue({ variant: 'lite' })
    await loadVariant()
    expect(variant.value).toBe('lite')
    vi.mocked(request).mockRejectedValue(new Error('offline'))
    await loadVariant()
    expect(nodeUnavailableReason({ protocol: 'http', owner: 3 })).toContain('无法读取版本能力')
  })
})
