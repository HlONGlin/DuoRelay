import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { localize } from './appearance'
import { getTraffic, getTrafficHistory } from '@/api/traffic'
import type { TrafficSnapshot } from '@/types'

const emptySnapshot: TrafficSnapshot = { up: 0, down: 0, sampled_at: '' }

export const useTrafficStore = defineStore('traffic', () => {
  const snapshot = ref<TrafficSnapshot>({ ...emptySnapshot })
  const history = ref<TrafficSnapshot[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  let timer: ReturnType<typeof setInterval> | undefined

  const peakUp = computed(() => Math.max(0, ...history.value.map((item) => item.up)))
  const peakDown = computed(() => Math.max(0, ...history.value.map((item) => item.down)))
  const averageUp = computed(() => average(history.value.map((item) => item.up)))
  const averageDown = computed(() => average(history.value.map((item) => item.down)))

  async function fetchTraffic() {
    loading.value = true
    try {
      const value = await getTraffic()
      snapshot.value = value
      history.value = [...history.value, value].slice(-30)
      error.value = null
      return value
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('获取流量统计失败', 'Failed to fetch traffic statistics')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchHistory(limit = 60) {
    try {
      const values = await getTrafficHistory(limit)
      history.value = values.slice(-limit)
      const latest = values.at(-1)
      if (latest) snapshot.value = latest
      return values
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('获取流量历史失败', 'Failed to fetch traffic history')
      return []
    }
  }

  function startPolling(intervalMs = 2000) {
    stopPolling()
    void fetchHistory().finally(() => { void fetchTraffic().catch(() => undefined) })
    timer = setInterval(() => void fetchTraffic().catch(() => undefined), intervalMs)
  }

  function stopPolling() {
    if (timer) {
      clearInterval(timer)
      timer = undefined
    }
  }

  return {
    snapshot,
    history,
    loading,
    error,
    peakUp,
    peakDown,
    averageUp,
    averageDown,
    fetchTraffic,
    fetchHistory,
    startPolling,
    stopPolling
  }
})

function average(values: number[]): number {
  if (values.length === 0) return 0
  return values.reduce((total, value) => total + value, 0) / values.length
}
