import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { localize } from './appearance'
import {
  getConnections,
  getKernelLogSummary,
  getKernelLogs,
  getKernelStatus,
  startKernel,
  stopKernel
} from '@/api/kernel'
import type {
  ConnectionsSnapshot,
  KernelLog,
  KernelLogQuery,
  KernelLogSummary,
  KernelStatus,
  KernelType
} from '@/types'

export const useKernelStore = defineStore('kernel', () => {
  const status = ref<KernelStatus>({ running: false, active_kernel: '' })
  const logs = ref<KernelLog[]>([])
  const logSummary = ref<KernelLogSummary>({
    kernel: 'sing-box',
    total: 0,
    levels: { debug: 0, info: 0, warn: 0, error: 0 },
    sources: {},
    clusters: []
  })
  const connections = ref<ConnectionsSnapshot>({
    download_total: 0,
    upload_total: 0,
    connections: []
  })
  const loading = ref(false)
  const busy = ref(false)
  const error = ref<string | null>(null)

  const displayStatus = computed(() => {
    if (status.value.running) return localize('运行中', 'Running')
    return status.value.active_kernel ? localize('已停止', 'Stopped') : localize('未配置', 'Not configured')
  })

  const statusType = computed<'success' | 'info' | 'danger'>(() => {
    if (status.value.running) return 'success'
    return status.value.error ? 'danger' : 'info'
  })

  async function fetchStatus() {
    loading.value = true
    try {
      status.value = await getKernelStatus()
      error.value = null
    } catch (err) {
      status.value = { running: false, active_kernel: '' }
      error.value = err instanceof Error ? err.message : localize('获取内核状态失败', 'Failed to fetch kernel status')
    } finally {
      loading.value = false
    }
  }

  async function start(kernel?: KernelType) {
    busy.value = true
    error.value = null
    try {
      status.value = await startKernel(kernel)
      return status.value
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('启动内核失败', 'Failed to start kernel')
      throw err
    } finally {
      busy.value = false
    }
  }

  async function stop() {
    busy.value = true
    error.value = null
    try {
      status.value = await stopKernel()
      return status.value
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('停止内核失败', 'Failed to stop kernel')
      throw err
    } finally {
      busy.value = false
    }
  }

  async function fetchLogs(query: KernelLogQuery = {}) {
    try {
      const [items, summary] = await Promise.all([
        getKernelLogs(query),
        getKernelLogSummary(query)
      ])
      logs.value = items.slice(-100)
      logSummary.value = summary
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('获取日志失败', 'Failed to fetch logs')
    }
  }

  function appendLiveLog(item: KernelLog) {
    const index = logs.value.findIndex(log => log.source === item.source && (log.repeat_key ?? log.message) === (item.repeat_key ?? item.message))
    if (index >= 0) {
      const previous = logs.value[index]
      logs.value = [...logs.value.filter((_, i) => i !== index), { ...previous, ...item, count: (previous.count ?? 1) + (item.count ?? 1) }]
    } else logs.value = [...logs.value, item].slice(-100)
    const level = item.level as keyof typeof logSummary.value.levels
    if (level in logSummary.value.levels) {
      logSummary.value.levels[level] += item.count ?? 1
      logSummary.value.total += item.count ?? 1
    }
  }

  async function fetchConnections() {
    try {
      connections.value = await getConnections()
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('获取连接列表失败', 'Failed to fetch connections')
    }
  }

  return {
    status,
    logs,
    logSummary,
    connections,
    loading,
    busy,
    error,
    displayStatus,
    statusType,
    fetchStatus,
    start,
    stop,
    fetchLogs,
    appendLiveLog,
    fetchConnections
  }
})
