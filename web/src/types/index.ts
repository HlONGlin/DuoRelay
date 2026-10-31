export type KernelType = 'sing-box' | 'mihomo'

export type Protocol =
  | 'shadowsocks'
  | 'vmess'
  | 'vless'
  | 'trojan'
  | 'hysteria'
  | 'hysteria2'
  | 'tuic'
  | 'anytls'
  | 'snell'
  | 'socks5'
  | 'http'
  | 'ssh'
  | 'wireguard'
  | 'openvpn'
  | 'masque'
  | 'tailscale'
  | 'shadowtls'
  | 'naive'
  | 'tor'
  | 'openconnect'
  | 'shadowsocksr'
  | 'mieru'
  | 'sudoku'
  | 'shadowquic'
  | 'trusttunnel'
  | 'rematch'
  | 'easytier'
  | 'zerotier'

export interface ApiErrorPayload {
  code: string
  what: string
  why?: string
  fix?: {
    label: string
    action: string
    target?: string
  }
}

export interface ApiResponse<T> {
  success: boolean
  data?: T
  error?: ApiErrorPayload
}

export interface ProxyServer {
  host: string
  port: number
}

export interface ProxyTransport {
  type: string
  path?: string
  host?: string
  opts?: Record<string, unknown>
}

export interface ProxySecurity {
  type: string
  sni?: string
  alpn?: string[]
  fingerprint?: string
  opts?: Record<string, unknown>
}

export interface NodeSource {
  type: string
  subscription_id?: string
  url?: string
}

export interface ProxyNode {
  id: string
  name: string
  protocol: Protocol | string
  owner: number
  server: ProxyServer
  transport?: ProxyTransport
  security?: ProxySecurity
  proto?: Record<string, unknown>
  extra?: Record<string, unknown>
  source?: NodeSource
  group_id?: string
  tags?: string[]
  favorite?: boolean
  sort_order?: number
  status?: string
  last_delay_ms?: number
  last_test_at?: string
  updated_at?: string
  created_at?: string
}

export interface Subscription {
  id: string
  name: string
  url: string
  user_agent?: string
  interval?: number
  last_update?: string
  last_status?: string
  used?: number
  total?: number
  expire?: string
  created_at?: string
  updated_at?: string
}

export interface SubscriptionInput {
  name: string
  url: string
  user_agent?: string
  interval?: number
}

export interface KernelStatus {
  running: boolean
  active_kernel?: KernelType | ''
  version?: string
  pid?: number
  uptime_seconds?: number
  started_at?: string
  error?: string
}

export interface KernelLog {
  time: string
  level: string
  message: string
}

export interface ConnectionMetadata {
  network?: string
  type?: string
  sourceIP?: string
  destinationIP?: string
  sourcePort?: string
  destinationPort?: string
  host?: string
  dnsMode?: string
  processPath?: string
}

export interface Connection {
  id: string
  metadata: ConnectionMetadata
  upload: number
  download: number
  start: string
  chains?: string[]
  rule?: string
  rulePayload?: string
}

export interface ConnectionsSnapshot {
  download_total: number
  upload_total: number
  connections: Connection[]
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'
export type TunStack = 'system' | 'gvisor' | 'mixed'

export interface AppConfig {
  kernel: KernelType
  inbound_host: string
  inbound_port: number
  mixed_port: number
  allow_lan: boolean
  log_level: LogLevel
  dns_servers: string[]
  dns_fake_ip: boolean
  enable_tun: boolean
  tun_stack: TunStack
  default_outbound: string
}
