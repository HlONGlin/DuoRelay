import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import * as kernelApi from '@/api/kernel'
import { useKernelStore } from './kernel'

vi.mock('@/api/kernel', () => ({
  getConnections: vi.fn(),
  getKernelLogSummary: vi.fn(),
  getKernelLogs: vi.fn(),
  getKernelStatus: vi.fn(),
  startKernel: vi.fn(),
  stopKernel: vi.fn()
}))

describe('kernel store logs', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads filtered logs together with the analysis summary', async () => {
    vi.mocked(kernelApi.getKernelLogs).mockResolvedValue([
      { time: '2026-11-01T00:00:00Z', level: 'error', message: 'connection failed' }
    ])
    vi.mocked(kernelApi.getKernelLogSummary).mockResolvedValue({
      kernel: 'sing-box',
      total: 1,
      levels: { debug: 0, info: 0, warn: 0, error: 1 },
      sources: { stderr: 1 },
      clusters: []
    })

    const store = useKernelStore()
    await store.fetchLogs({ level: 'error', source: 'stderr', search: 'failed' })

    expect(kernelApi.getKernelLogs).toHaveBeenCalledWith({
      level: 'error',
      source: 'stderr',
      search: 'failed'
    })
    expect(kernelApi.getKernelLogSummary).toHaveBeenCalledWith({
      level: 'error',
      source: 'stderr',
      search: 'failed'
    })
    expect(store.logs).toHaveLength(1)
    expect(store.logSummary.total).toBe(1)
  })

  it('bounds live history and counts repeated errors without filling the viewport', () => {
    const store = useKernelStore()
    for (let i = 0; i < 150; i++) {
      store.appendLiveLog({ time: String(i), level: 'info', source: 'stdout', message: `message ${i}` })
    }
    expect(store.logs).toHaveLength(100)
    expect(store.logs[0].message).toBe('message 50')
    for (let i = 0; i < 500; i++) {
      store.appendLiveLog({ time: String(i), level: 'error', source: 'stderr', message: `[${i}] connection refused`, repeat_key: 'connection refused', count: 2 })
    }
    expect(store.logs).toHaveLength(100)
    expect(store.logs.filter(item => item.repeat_key === 'connection refused')).toHaveLength(1)
    expect(store.logs[99].count).toBe(1000)
    expect(store.logSummary.levels.error).toBe(1000)
    store.appendLiveLog({ time: '501', level: 'error', source: 'stdout', message: 'connection refused', repeat_key: 'connection refused' })
    expect(store.logs.filter(item => item.repeat_key === 'connection refused')).toHaveLength(2)
    const oldest = store.logs[0]
    store.appendLiveLog({ ...oldest, time: '502' })
    expect(store.logs[99].message).toBe(oldest.message)
    store.appendLiveLog({ time: '503', level: 'info', message: 'newest' })
    expect(store.logs.some(item => item.message === oldest.message)).toBe(true)
  })

  it('bounds initial HTTP history to the latest 100 rows', async () => {
    vi.mocked(kernelApi.getKernelLogs).mockResolvedValue(Array.from({ length: 150 }, (_, i) => ({ time: String(i), level: 'info', message: String(i) })))
    vi.mocked(kernelApi.getKernelLogSummary).mockResolvedValue({ kernel: 'mihomo', total: 150, levels: { debug: 0, info: 150, warn: 0, error: 0 }, sources: {}, clusters: [] })
    const store = useKernelStore()
    await store.fetchLogs()
    expect(store.logs).toHaveLength(100)
    expect(store.logs[0].message).toBe('50')
    expect(store.logs[99].message).toBe('149')
  })
})
