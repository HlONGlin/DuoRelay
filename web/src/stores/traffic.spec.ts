import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import * as trafficApi from '@/api/traffic'
import { useTrafficStore } from './traffic'

vi.mock('@/api/traffic', () => ({
  getTraffic: vi.fn(),
  getTrafficHistory: vi.fn()
}))

describe('traffic store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('keeps current, peak and average samples', async () => {
    vi.mocked(trafficApi.getTraffic)
      .mockResolvedValueOnce({ up: 100, down: 400, sampled_at: '2026-11-01T00:00:00Z' })
      .mockResolvedValueOnce({ up: 300, down: 200, sampled_at: '2026-11-01T00:00:02Z' })

    const store = useTrafficStore()
    await store.fetchTraffic()
    await store.fetchTraffic()

    expect(store.snapshot.up).toBe(300)
    expect(store.peakUp).toBe(300)
    expect(store.peakDown).toBe(400)
    expect(store.averageUp).toBe(200)
    expect(store.averageDown).toBe(300)
  })

  it('records request failures', async () => {
    vi.mocked(trafficApi.getTraffic).mockRejectedValue(new Error('mock unavailable'))

    const store = useTrafficStore()
    await expect(store.fetchTraffic()).rejects.toThrow('mock unavailable')
    expect(store.error).toBe('mock unavailable')
    expect(store.loading).toBe(false)
  })
})
