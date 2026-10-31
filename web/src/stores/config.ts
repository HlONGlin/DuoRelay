import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getConfig, updateConfig } from '@/api/config'
import type { AppConfig } from '@/types'

export function defaultConfig(): AppConfig {
  return {
    kernel: 'sing-box',
    inbound_host: '127.0.0.1',
    inbound_port: 1080,
    mixed_port: 7890,
    allow_lan: false,
    log_level: 'info',
    dns_servers: ['223.5.5.5', '8.8.8.8'],
    dns_fake_ip: false,
    enable_tun: false,
    tun_stack: 'system',
    default_outbound: 'DIRECT'
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
      config.value = await getConfig()
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载配置失败'
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
      error.value = err instanceof Error ? err.message : '保存配置失败'
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
