import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia } from 'pinia'
import { afterEach, expect, it, vi } from 'vitest'
import * as nodeApi from '@/api/nodes'
import { useAppearanceStore } from '@/stores/appearance'
import { variant } from '@/stores/variant'
import NodeList from './NodeList.vue'

vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('@/api/config', () => ({ getCurrentConfig: vi.fn().mockResolvedValue(null), getConfig: vi.fn(), applyConfig: vi.fn(), getProxyGroupStatus: vi.fn() }))
vi.mock('@/api/nodes', () => ({ listNodes: vi.fn(), activateNode: vi.fn(), testNodeDelay: vi.fn(), deleteNode: vi.fn(), importNodes: vi.fn() }))
vi.mock('@/api/subscriptions', () => ({ listSubscriptions: vi.fn().mockResolvedValue([]) }))

afterEach(() => { variant.value = 'full'; vi.unstubAllGlobals() })

it('shows node essentials and keeps guarded actions inside narrow screen details', async () => {
  variant.value = 'lite'
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })))
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} })
  const host = 'long-node-address.example.test'
  vi.mocked(nodeApi.listNodes).mockResolvedValue([
    { id: '1', name: 'Bound node', owner: 3, protocol: 'socks5', server: { host, port: 1080 }, status: 'socks', last_delay_ms: 14 },
    { id: '2', name: 'Unavailable node', owner: 1, protocol: 'naive', server: { host, port: 443 } },
    { id: '3', name: 'Chain node', owner: 2, protocol: 'proxy-chain', server: { host, port: 0 }, composition: { mode: 'chain', kernel: 'mihomo', node_ids: ['1', '2'] } }
  ])
  const pinia = createPinia()
  useAppearanceStore(pinia).language = 'en'
  const wrapper = mount(NodeList, { global: { plugins: [pinia, ElementPlus] } })
  try {
    await flushPromises()
    expect(wrapper.find('.node-scroll-narrow').exists()).toBe(true)
    const [bound, unavailable, chain] = wrapper.findAll('.node-row')
    expect(bound.text()).toContain('Bound node')
    expect(bound.get('.neutral-badge').text()).toBe('SOCKS5')
    expect(bound.get('.numeric').text()).toContain('14')
    expect(bound.get('.activate-button').text()).toBe('Select and start')
    expect(bound.get('.endpoint details').text()).toContain(`${host}:1080`)
    const actions = bound.get<HTMLDetailsElement>('.secondary-actions')
    expect(actions.element.open).toBe(false)
    expect(actions.get('summary').text()).toBe('More actions')
    expect(actions.findAll('button').map(button => button.text())).toEqual(['Test', 'Edit', 'Delete'])
    expect(actions.findAll('button')[2].attributes('disabled')).toBeDefined()
    expect(unavailable.get('.activate-button').attributes('disabled')).toBeDefined()
    expect(chain.get('.secondary-actions button').attributes('disabled')).toBeDefined()
  } finally {
    wrapper.unmount()
  }
})
