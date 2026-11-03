import type { KernelType, Protocol } from '@/types'
import { readLanguage } from '@/stores/appearance'

export type ProtocolGroup = 'shared' | 'sing-box' | 'mihomo'

export interface ProtocolMeta {
  id: Protocol
  label: string
  glyph: string
  group: ProtocolGroup
  kernels: KernelType[]
  description: string
}

export const protocolCatalog: ProtocolMeta[] = [
  { id: 'shadowsocks', label: 'Shadowsocks', glyph: 'SS', group: 'shared', kernels: ['sing-box', 'mihomo'], description: '加密代理' },
  { id: 'vmess', label: 'VMess', glyph: 'VM', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'V2Ray 兼容代理' },
  { id: 'vless', label: 'VLESS', glyph: 'VL', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'UUID 轻量代理' },
  { id: 'trojan', label: 'Trojan', glyph: 'TR', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'TLS 加密代理' },
  { id: 'hysteria', label: 'Hysteria', glyph: 'H1', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'QUIC 高速代理' },
  { id: 'hysteria2', label: 'Hysteria2', glyph: 'H2', group: 'shared', kernels: ['sing-box', 'mihomo'], description: '第二代 QUIC 高速代理' },
  { id: 'tuic', label: 'TUIC', glyph: 'TU', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'QUIC 多路复用代理' },
  { id: 'anytls', label: 'AnyTLS', glyph: 'AT', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'TLS 多路复用代理' },
  { id: 'snell', label: 'Snell', glyph: 'SN', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'Snell 传输代理' },
  { id: 'socks5', label: 'SOCKS5', glyph: 'S5', group: 'shared', kernels: ['sing-box', 'mihomo'], description: '标准 SOCKS5 代理' },
  { id: 'http', label: 'HTTP', glyph: 'HT', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'HTTP 或 HTTPS 代理' },
  { id: 'ssh', label: 'SSH', glyph: 'SH', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'SSH 传输代理' },
  { id: 'wireguard', label: 'WireGuard', glyph: 'WG', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'WireGuard 隧道端点' },
  { id: 'openvpn', label: 'OpenVPN', glyph: 'OV', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'OpenVPN 隧道端点' },
  { id: 'masque', label: 'MASQUE', glyph: 'MQ', group: 'mihomo', kernels: ['mihomo'], description: 'HTTP/3 MASQUE 覆盖网络' },
  { id: 'tailscale', label: 'Tailscale', glyph: 'TS', group: 'shared', kernels: ['sing-box', 'mihomo'], description: 'Tailscale 覆盖网络端点' },
  { id: 'shadowtls', label: 'ShadowTLS', glyph: 'ST', group: 'sing-box', kernels: ['sing-box'], description: 'ShadowTLS 伪装代理' },
  { id: 'naive', label: 'Naive', glyph: 'NV', group: 'sing-box', kernels: ['sing-box'], description: 'Naive HTTPS 代理' },
  { id: 'tor', label: 'Tor', glyph: 'TO', group: 'sing-box', kernels: ['sing-box'], description: 'Tor SOCKS 端点' },
  { id: 'openconnect', label: 'OpenConnect', glyph: 'OC', group: 'sing-box', kernels: ['sing-box'], description: 'AnyConnect 兼容端点' },
  { id: 'shadowsocksr', label: 'ShadowsocksR', glyph: 'SR', group: 'mihomo', kernels: ['mihomo'], description: 'ShadowsocksR 代理' },
  { id: 'mieru', label: 'Mieru', glyph: 'MI', group: 'mihomo', kernels: ['mihomo'], description: 'Mieru 传输代理' },
  { id: 'sudoku', label: 'Sudoku', glyph: 'SU', group: 'mihomo', kernels: ['mihomo'], description: 'Sudoku 混淆代理' },
  { id: 'shadowquic', label: 'ShadowQUIC', glyph: 'SQ', group: 'mihomo', kernels: ['mihomo'], description: 'ShadowQUIC 代理' },
  { id: 'trusttunnel', label: 'TrustTunnel', glyph: 'TT', group: 'mihomo', kernels: ['mihomo'], description: 'TrustTunnel 代理' },
  { id: 'rematch', label: 'Rematch', glyph: 'RM', group: 'mihomo', kernels: ['mihomo'], description: '路由元数据适配器' },
  { id: 'easytier', label: 'EasyTier', glyph: 'ET', group: 'mihomo', kernels: ['mihomo'], description: '内嵌组网覆盖网络' },
  { id: 'zerotier', label: 'ZeroTier', glyph: 'ZT', group: 'mihomo', kernels: ['mihomo'], description: 'ZeroTier 网络端点' }
]

const byId = new Map<string, ProtocolMeta>(protocolCatalog.map((item) => [item.id, item]))

export function protocolMeta(protocol: Protocol | string): ProtocolMeta | undefined {
  return byId.get(protocol)
}

export function orderedProtocols(protocols: string[]): string[] {
  const available = new Set(protocols)
  const known = protocolCatalog.filter((item) => available.has(item.id)).map((item) => item.id)
  const unknown = protocols.filter((protocol) => !byId.has(protocol)).sort()
  return [...known, ...unknown]
}

export function protocolGroupLabel(group: ProtocolGroup): string {
  if (readLanguage() === 'en') {
    switch (group) {
      case 'shared': return 'Shared protocols (both kernels)'
      case 'sing-box': return 'sing-box only'
      case 'mihomo': return 'mihomo only'
    }
  }
  switch (group) {
    case 'shared':
      return '通用协议（双内核）'
    case 'sing-box':
      return 'sing-box 专用'
    case 'mihomo':
      return 'mihomo 专用'
  }
}
