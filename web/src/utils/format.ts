import type { KernelType, Protocol } from '@/types'

const protocolLabels: Record<string, string> = {
  shadowsocks: 'Shadowsocks',
  shadowsocksr: 'ShadowsocksR',
  vmess: 'VMess',
  vless: 'VLESS',
  trojan: 'Trojan',
  hysteria: 'Hysteria',
  hysteria2: 'Hysteria2',
  tuic: 'TUIC',
  anytls: 'AnyTLS',
  snell: 'Snell',
  socks5: 'SOCKS5',
  http: 'HTTP',
  ssh: 'SSH',
  wireguard: 'WireGuard',
  openvpn: 'OpenVPN',
  masque: 'MASQUE',
  tailscale: 'Tailscale',
  shadowtls: 'ShadowTLS',
  naive: 'Naive',
  tor: 'Tor',
  openconnect: 'OpenConnect',
  mieru: 'Mieru',
  sudoku: 'Sudoku',
  shadowquic: 'ShadowQUIC',
  trusttunnel: 'TrustTunnel',
  rematch: 'Rematch',
  easytier: 'EasyTier',
  zerotier: 'ZeroTier'
}

export function protocolLabel(protocol: Protocol | string): string {
  return protocolLabels[protocol] ?? protocol
}

export function ownerKernels(owner: number): KernelType[] {
  const kernels: KernelType[] = []
  if (owner & 1) kernels.push('sing-box')
  if (owner & 2) kernels.push('mihomo')
  return kernels
}

export function ownerLabel(owner: number): string {
  const kernels = ownerKernels(owner)
  if (kernels.length === 0) return '未知'
  if (kernels.length === 2) return '双内核'
  return kernels[0]
}

export function formatDelay(delay?: number): string {
  if (!delay || delay <= 0) return '—'
  return `${delay} ms`
}

export function delayType(delay?: number): 'success' | 'warning' | 'danger' | 'info' {
  if (!delay || delay <= 0) return 'info'
  if (delay < 300) return 'success'
  if (delay < 800) return 'warning'
  return 'danger'
}

export function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return '0 B'
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
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function formatUptime(seconds?: number): string {
  if (!seconds || seconds <= 0) return '—'
  const total = Math.floor(seconds)
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
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
