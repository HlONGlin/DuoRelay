import { ref } from 'vue'
import { defineStore } from 'pinia'
import { localize } from './appearance'
import { getConfig, updateConfig } from '@/api/config'
import type { AppConfig } from '@/types'

export function defaultLoadBalance(): AppConfig['load_balance'] {
  return { strategy: 'round-robin', url: 'https://www.gstatic.com/generate_204', interval: 300, timeout: 5000, lazy: true }
}

export function defaultConfig(): AppConfig {
  return {
    kernel: 'sing-box',
    inbound_host: '127.0.0.1',
    inbound_port: 1080,
    mixed_port: 7890,
    socks_only: false,
    socks_port: 10808,
    load_balance: defaultLoadBalance(),
    allow_lan: false,
    log_level: 'info',
    dns_servers: ['223.5.5.5', '8.8.8.8'],
    dns_fake_ip: false,
    enable_tun: false,
    transparent_mode: 'off',
    tun_stack: 'system',
    system_proxy: false,
    system_proxy_supported: false,
    routing_mode: 'bypass_mainland',
    default_outbound: 'PROXY',
    routing_rules: [
      { match: 'domain_suffix', values: ['cn', '中国', 'local'], outbound: 'DIRECT' },
      {
        match: 'ip_cidr',
        values: ['10.0.0.0/8', '100.64.0.0/10', '172.16.0.0/12', '192.168.0.0/16'],
        outbound: 'DIRECT'
      }
    ]
  }
}

export const useConfigStore = defineStore('config', () => {
  const config = ref<AppConfig>(defaultConfig())
  const loading = ref(false)
  const saving = ref(false)
  const error = ref<string | null>(null)
  const savedAt = ref('')

  async function fetchConfig() {
    loading.value = true
    error.value = null
    try {
      const value = await getConfig()
      config.value = { ...defaultConfig(), ...value, load_balance: { ...defaultLoadBalance(), ...value.load_balance } }
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('加载配置失败', 'Failed to load configuration')
    } finally {
      loading.value = false
    }
  }

  async function save(next: AppConfig) {
    saving.value = true
    error.value = null
    try {
      config.value = await updateConfig(next)
      savedAt.value = new Date().toISOString()
      return config.value
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('保存配置失败', 'Failed to save configuration')
      throw err
    } finally {
      saving.value = false
    }
  }

  function resetLocal() {
    config.value = defaultConfig()
    savedAt.value = ''
  }

  return {
    config,
    loading,
    saving,
    error,
    savedAt,
    fetchConfig,
    save,
    resetLocal
  }
})
