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
})
