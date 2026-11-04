import type { KernelType, Protocol } from '@/types'
import { protocolMeta } from './protocols'
import { readLanguage } from '@/stores/appearance'

export function protocolLabel(protocol: Protocol | string): string {
  return protocolMeta(protocol)?.label ?? protocol
}

export function protocolGlyph(protocol: Protocol | string): string {
  return protocolMeta(protocol)?.glyph ?? protocol.slice(0, 2).toUpperCase()
}

export function protocolDescription(protocol: Protocol | string): string {
  if (readLanguage() === 'en') {
    const descriptions: Record<string, string> = {
      shadowsocks: 'Encrypted proxy', vmess: 'V2Ray-compatible proxy', vless: 'Lightweight UUID proxy',
      trojan: 'TLS-encrypted proxy', hysteria: 'High-speed QUIC proxy', hysteria2: 'Second-generation high-speed QUIC proxy',
      tuic: 'Multiplexed QUIC proxy', anytls: 'Multiplexed TLS proxy', snell: 'Snell transport proxy',
      socks5: 'Standard SOCKS5 proxy', http: 'HTTP or HTTPS proxy', ssh: 'SSH transport proxy',
      wireguard: 'WireGuard tunnel endpoint', openvpn: 'OpenVPN tunnel endpoint', masque: 'HTTP/3 MASQUE overlay network',
      tailscale: 'Tailscale overlay network endpoint', shadowtls: 'ShadowTLS obfuscated proxy', naive: 'Naive HTTPS proxy',
      tor: 'Tor SOCKS endpoint', openconnect: 'AnyConnect-compatible endpoint', shadowsocksr: 'ShadowsocksR proxy',
      mieru: 'Mieru transport proxy', sudoku: 'Sudoku obfuscated proxy', shadowquic: 'ShadowQUIC proxy',
      trusttunnel: 'TrustTunnel proxy', rematch: 'Routing metadata adapter', easytier: 'Embedded overlay network',
      zerotier: 'ZeroTier network endpoint'
    }
    return descriptions[protocol] ?? 'Custom protocol'
  }
  return protocolMeta(protocol)?.description ?? '自定义协议'
}

export function nodeEndpointLabel(node: {
  protocol: Protocol | string
  server?: { host?: string; port?: number }
}): string {
  const host = node.server?.host?.trim()
  const port = node.server?.port
  if (host && port) return `${host}:${port}`
  if (node.protocol === 'rematch') return readLanguage() === 'en' ? 'Routing adapter' : '路由适配器'
  return readLanguage() === 'en' ? 'Local overlay network' : '本地覆盖网络'
}

export function ownerKernels(owner: number): KernelType[] {
  const kernels: KernelType[] = []
  if (owner & 1) kernels.push('sing-box')
  if (owner & 2) kernels.push('mihomo')
  return kernels
}

export function ownerLabel(owner: number): string {
  const kernels = ownerKernels(owner)
  if (kernels.length === 0) return readLanguage() === 'en' ? 'Unknown' : '未知'
  if (kernels.length === 2) return readLanguage() === 'en' ? 'Dual kernel' : '双内核'
  return kernels[0]
}

export function formatDelay(delay?: number): string {
  if (!delay || delay <= 0) return '—'
  return `${delay} ms`
}

export function delayType(delay?: number): 'success' | 'warning' | 'danger' | 'info' {
  if (!delay || delay <= 0) return 'info'
  if (delay <= 100) return 'success'
  if (delay <= 250) return 'warning'
  return 'danger'
}

export function formatBytes(bytes?: number): string {
  if (bytes == null || !Number.isFinite(bytes)) return '—'
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  const digits = value >= 100 || unit === 0 ? 0 : 1
  return `${value.toFixed(digits)} ${units[unit]}`
}

export function formatDate(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getTime() === 0) return '—'
  return date.toLocaleString(readLanguage(), { hour12: false })
}

export function formatUptime(seconds?: number): string {
  if (!seconds || seconds <= 0) return '—'
  const total = Math.floor(seconds)
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
  if (readLanguage() === 'en') {
    if (days > 0) return `${days} d ${hours} h`
    if (hours > 0) return `${hours} h ${minutes} min`
    if (minutes > 0) return `${minutes} min ${secs} s`
    return `${secs} s`
  }
  if (days > 0) return `${days} 天 ${hours} 小时`
  if (hours > 0) return `${hours} 小时 ${minutes} 分`
  if (minutes > 0) return `${minutes} 分 ${secs} 秒`
  return `${secs} 秒`
}

export function usagePercent(used?: number, total?: number): number {
  if (!used || !total || total <= 0) return 0
  return Math.min(100, Math.round((used / total) * 100))
}

export function statusType(status?: string): 'success' | 'warning' | 'danger' | 'info' {
  switch (status) {
    case 'active':
    case 'success':
    case 'ok':
      return 'success'
    case 'testing':
    case 'updating':
      return 'warning'
    case 'error':
    case 'failed':
      return 'danger'
    default:
      return 'info'
  }
}
