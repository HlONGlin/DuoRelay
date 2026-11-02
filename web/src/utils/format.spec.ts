import { describe, expect, it } from 'vitest'
import {
  delayType,
  formatBytes,
  formatDelay,
  formatUptime,
  nodeEndpointLabel,
  ownerKernels,
  ownerLabel,
  protocolDescription,
  protocolGlyph,
  protocolLabel,
  statusType,
  usagePercent
} from './format'

describe('format helpers', () => {
  it('formats delay values', () => {
    expect(formatDelay(0)).toBe('—')
    expect(formatDelay(120)).toBe('120 ms')
    expect(delayType(100)).toBe('success')
    expect(delayType(101)).toBe('warning')
    expect(delayType(250)).toBe('warning')
    expect(delayType(251)).toBe('danger')
    expect(delayType(1200)).toBe('danger')
    expect(delayType(undefined)).toBe('info')
  })

  it('formats byte sizes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(2048)).toBe('2.0 KB')
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB')
  })

  it('formats uptime', () => {
    expect(formatUptime(0)).toBe('—')
    expect(formatUptime(45)).toBe('45 秒')
    expect(formatUptime(125)).toBe('2 分 5 秒')
    expect(formatUptime(7300)).toBe('2 小时 1 分')
    expect(formatUptime(90000)).toBe('1 天 1 小时')
  })

  it('maps kernel ownership', () => {
    expect(ownerKernels(1)).toEqual(['sing-box'])
    expect(ownerKernels(2)).toEqual(['mihomo'])
    expect(ownerKernels(3)).toEqual(['sing-box', 'mihomo'])
    expect(ownerLabel(3)).toBe('双内核')
    expect(ownerLabel(0)).toBe('未知')
  })

  it('maps protocol and status labels', () => {
    expect(protocolLabel('vmess')).toBe('VMess')
    expect(protocolGlyph('wireguard')).toBe('WG')
    expect(protocolDescription('rematch')).toContain('路由')
    expect(protocolLabel('custom')).toBe('custom')
    expect(statusType('active')).toBe('success')
    expect(statusType('error')).toBe('danger')
    expect(statusType('unknown')).toBe('info')
  })

  it('formats regular and overlay node endpoints', () => {
    expect(nodeEndpointLabel({ protocol: 'vmess', server: { host: 'hk.example.com', port: 443 } })).toBe(
      'hk.example.com:443'
    )
    expect(nodeEndpointLabel({ protocol: 'rematch', server: { host: '', port: 0 } })).toBe(
      '路由适配器'
    )
    expect(nodeEndpointLabel({ protocol: 'tailscale' })).toBe('本地覆盖网络')
  })

  it('calculates usage percent safely', () => {
    expect(usagePercent(50, 100)).toBe(50)
    expect(usagePercent(200, 100)).toBe(100)
    expect(usagePercent(0, 0)).toBe(0)
  })
})
