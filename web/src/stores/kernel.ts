import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
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
    if (status.value.running) return '运行中'
    return status.value.active_kernel ? '已停止' : '未配置'
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
      error.value = err instanceof Error ? err.message : '获取内核状态失败'
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
      error.value = err instanceof Error ? err.message : '启动内核失败'
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
      error.value = err instanceof Error ? err.message : '停止内核失败'
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
      logs.value = items
      logSummary.value = summary
    } catch (err) {
      error.value = err instanceof Error ? err.message : '获取日志失败'
    }
  }

  function appendLiveLog(item: KernelLog) {
    logs.value = [...logs.value, item].slice(-200)
    const level = item.level as keyof typeof logSummary.value.levels
    if (level in logSummary.value.levels) {
      logSummary.value.levels[level]++
      logSummary.value.total++
    }
  }

  async function fetchConnections() {
    try {
      connections.value = await getConnections()
    } catch (err) {
      error.value = err instanceof Error ? err.message : '获取连接列表失败'
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
