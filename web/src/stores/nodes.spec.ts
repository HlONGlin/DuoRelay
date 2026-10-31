import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import * as nodeApi from '@/api/nodes'
import { useNodesStore } from './nodes'
import type { ProxyNode } from '@/types'

vi.mock('@/api/nodes', () => ({
  listNodes: vi.fn(),
  activateNode: vi.fn(),
  testNodeDelay: vi.fn(),
  deleteNode: vi.fn()
}))

function makeNode(overrides: Partial<ProxyNode> = {}): ProxyNode {
  return {
    id: 'node-1',
    name: 'HK-SS-01',
    protocol: 'shadowsocks',
    owner: 3,
    server: { host: 'hk.example.com', port: 8388 },
    source: { type: 'subscription', subscription_id: 'sub-1' },
    status: 'idle',
    last_delay_ms: 0,
    ...overrides
  }
}

describe('nodes store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads nodes and derives protocol options', async () => {
    vi.mocked(nodeApi.listNodes).mockResolvedValue([
      makeNode(),
      makeNode({
        id: 'node-2',
        name: 'SG-VMess',
        protocol: 'vmess',
        source: { type: 'subscription', subscription_id: 'sub-2' }
      })
    ])

    const store = useNodesStore()
    await store.fetchNodes()

    expect(store.items).toHaveLength(2)
    expect(store.protocols).toEqual(['shadowsocks', 'vmess'])
    expect(store.subscriptionIds).toEqual(['sub-1', 'sub-2'])
    expect(store.error).toBeNull()
  })

  it('filters by search, protocol and subscription', async () => {
    vi.mocked(nodeApi.listNodes).mockResolvedValue([
      makeNode(),
      makeNode({
        id: 'node-2',
        name: 'SG-VMess',
        protocol: 'vmess',
        server: { host: 'sg.example.com', port: 443 }
      }),
      makeNode({
        id: 'node-3',
        name: 'JP-Trojan',
        protocol: 'trojan',
        server: { host: 'jp.example.com', port: 443 },
        source: { type: 'subscription', subscription_id: 'sub-2' }
      })
    ])

    const store = useNodesStore()
    await store.fetchNodes()

    store.search = 'hk'
    expect(store.filtered.map((node) => node.id)).toEqual(['node-1'])

    store.search = ''
    store.protocolFilter = 'vmess'
    expect(store.filtered.map((node) => node.id)).toEqual(['node-2'])

    store.protocolFilter = ''
    store.subscriptionFilter = 'sub-2'
    expect(store.filtered.map((node) => node.id)).toEqual(['node-3'])

    store.resetFilters()
    expect(store.filtered).toHaveLength(3)
  })

  it('activates a node and clears the previous active status', async () => {
    vi.mocked(nodeApi.listNodes).mockResolvedValue([
      makeNode({ id: 'node-1', status: 'active' }),
      makeNode({ id: 'node-2' })
    ])
    vi.mocked(nodeApi.activateNode).mockResolvedValue(makeNode({ id: 'node-2' }))

    const store = useNodesStore()
    await store.fetchNodes()
    expect(store.activeNodeId).toBe('node-1')

    await store.activate('node-2')

    expect(store.activeNodeId).toBe('node-2')
    expect(store.items.find((node) => node.id === 'node-2')?.status).toBe('active')
    expect(store.items.find((node) => node.id === 'node-1')?.status).toBe('idle')
    expect(store.activeNode?.id).toBe('node-2')
  })

  it('updates delay after a speed test', async () => {
    vi.mocked(nodeApi.listNodes).mockResolvedValue([makeNode()])
    vi.mocked(nodeApi.testNodeDelay).mockResolvedValue({ delay_ms: 88 })

    const store = useNodesStore()
    await store.fetchNodes()
    const delay = await store.testNode('node-1')

    expect(delay).toBe(88)
    expect(store.items[0].last_delay_ms).toBe(88)
    expect(store.items[0].last_test_at).toBeTruthy()
  })

  it('removes a node from local state', async () => {
    vi.mocked(nodeApi.listNodes).mockResolvedValue([makeNode(), makeNode({ id: 'node-2' })])
    vi.mocked(nodeApi.deleteNode).mockResolvedValue()

    const store = useNodesStore()
    await store.fetchNodes()
    await store.remove('node-1')

    expect(store.items).toHaveLength(1)
    expect(store.items[0].id).toBe('node-2')
  })

  it('records an error when loading fails', async () => {
    vi.mocked(nodeApi.listNodes).mockRejectedValue(new Error('后端不可用'))

    const store = useNodesStore()
    await store.fetchNodes()

    expect(store.error).toBe('后端不可用')
    expect(store.loading).toBe(false)
  })
})
