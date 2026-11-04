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
  | 'proxy-chain'
  | 'load-balance'

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

export interface UpdateInfo {
  current_version: string
  latest_version: string
  release_name?: string
  release_url?: string
  published_at?: string
  release_notes?: string
  asset_name?: string
  update_available: boolean
  can_update: boolean
  reason?: string
}

export interface UpdateResult {
  latest_version: string
  asset_name: string
  restarting: boolean
  message: string
}

export interface UpdateProgress {
  state: 'idle' | 'downloading' | 'restarting' | 'failed' | string
  message?: string
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
	composition?: NodeComposition
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

export interface NodeComposition {
  mode: ProxyGroupMode
  kernel: KernelType
  node_ids: string[]
  load_balance?: LoadBalanceOptions
}

export interface Subscription {
	last_error?: string
	skipped?: { line: number; reason: string }[]
	diff?: { duplicates?: number }
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
	count?: number
	repeat_key?: string
  time: string
  level: string
  message: string
  timestamp?: string
  source?: string
  line?: string
}

export type KernelLogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface KernelLogQuery {
  kernel?: KernelType
  limit?: number
  level?: KernelLogLevel | 'all'
  source?: 'stdout' | 'stderr'
  search?: string
}

export interface KernelLogCluster {
  message: string
  level: KernelLogLevel
  count: number
  last_seen: string
}

export interface KernelLogSummary {
  kernel: KernelType
  total: number
  levels: Record<KernelLogLevel, number>
  sources: Record<string, number>
  clusters: KernelLogCluster[]
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

export interface TrafficSnapshot {
  up: number
  down: number
  sampled_at: string
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'
export type TunStack = 'system' | 'gvisor' | 'mixed'
export type TransparentMode = 'off' | 'redirect' | 'tproxy' | 'tun'
export type RoutingMode = 'bypass_mainland' | 'global' | 'blacklist'

export type RoutingMatch =
  | 'domain'
  | 'domain_suffix'
  | 'domain_keyword'
  | 'ip_cidr'
  | 'source_ip_cidr'
  | 'port'
  | 'network'
  | 'protocol'
  | 'geosite'
  | 'geoip'

export interface RoutingRule {
  id?: string
  source?: 'internal' | 'bypass' | 'user' | string
  enabled?: boolean
  match: RoutingMatch
  values: string[]
  outbound: string
}

export interface AppConfig {
  kernel: KernelType
  inbound_host: string
  inbound_port: number
  mixed_port: number
  socks_only: boolean
  socks_port: number
  socks_listeners: SOCKSListener[]
  load_balance: LoadBalanceOptions
  allow_lan: boolean
  log_level: LogLevel
  dns_servers: string[]
  dns_bootstrap?: string
  dns_detour?: string
  dns_fake_ip: boolean
  enable_tun: boolean
  transparent_mode: TransparentMode
  transparent_modes?: TransparentModeInfo[]
  transparent_active?: TransparentMode
  transparent_reason?: string
  tun_stack: TunStack
  system_proxy: boolean
  system_proxy_supported: boolean
  routing_mode: RoutingMode
  default_outbound: string
  routing_rules: RoutingRule[]
}

export interface SOCKSListener {
  port: number
  node_id: string
}

export interface TransparentModeInfo {
  mode: TransparentMode
  supported: boolean
  reason?: string
  prerequisites: string
  dns_description: string
}

export type ProxyGroupMode = 'load_balance' | 'chain'
export interface LoadBalanceOptions {
  strategy: 'round-robin' | 'consistent-hashing' | 'sticky-sessions' | 'url-test' | 'fallback'
  url: string
  interval: number
  timeout: number
  lazy: boolean
}
export interface ApplyConfigRequest {
  kernel?: KernelType
  node_ids: string[]
  mode: ProxyGroupMode
  load_balance?: LoadBalanceOptions
}
export interface ConfigurationState {
  id: string
  kernel: KernelType
  node_ids: string[]
  socks_listeners?: SOCKSListener[]
  mode: ProxyGroupMode | 'single' | 'select'
	selected_node_id?: string
  load_balance?: LoadBalanceOptions
  applied_at: string
  chain?: ChainPlan
}
export interface ProxyGroupMemberStatus {
  name: string
  alive: boolean
  delay_ms?: number
}
export interface ProxyGroupStatus {
  active: boolean
  now?: string
  members?: ProxyGroupMemberStatus[]
}

export interface ChainPlan {
  supported: boolean
  reason?: string
  kernel?: KernelType
  cross_kernel: boolean
  udp: boolean
  hops: { id: string; name: string; protocol: string; kernel: KernelType; network: string; udp: boolean }[]
  warnings: string[]
}
