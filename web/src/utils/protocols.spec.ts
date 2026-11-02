import { describe, expect, it } from 'vitest'
import {
  orderedProtocols,
  protocolCatalog,
  protocolGroupLabel,
  protocolMeta
} from './protocols'

describe('protocol catalog', () => {
  it('contains all 28 supported protocol entries', () => {
    expect(protocolCatalog).toHaveLength(28)
    expect(new Set(protocolCatalog.map((item) => item.id)).size).toBe(28)
  })

  it('keeps kernel-only protocols in the correct filter group', () => {
    expect(protocolMeta('masque')?.group).toBe('mihomo')
    expect(protocolMeta('shadowtls')?.group).toBe('sing-box')
    expect(protocolMeta('vmess')?.group).toBe('shared')
  })

  it('orders known protocols by the catalog and unknown values last', () => {
    expect(orderedProtocols(['custom', 'vmess', 'shadowsocks'])).toEqual([
      'shadowsocks',
      'vmess',
      'custom'
    ])
    expect(protocolGroupLabel('mihomo')).toBe('mihomo 专用')
  })
})
