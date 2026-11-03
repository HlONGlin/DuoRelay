import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import ProxyGroupDialog from './ProxyGroupDialog.vue'
import { applyConfig, previewChain } from '@/api/config'
import { defaultLoadBalance } from '@/stores/config'
import type { ConfigurationState, ProxyGroupMode, ProxyNode } from '@/types'
import { variant } from '@/stores/variant'

vi.mock('@/api/config', () => ({ applyConfig: vi.fn(), previewChain: vi.fn() }))

const nodes: ProxyNode[] = [
  { id: 'a', name: '入口 A', protocol: 'http', owner: 3, server: { host: 'a.example', port: 443 } },
  { id: 'b', name: '出口 B', protocol: 'anytls', owner: 3, server: { host: 'b.example', port: 443 } }
]

describe('multi-node start dialog', () => {
  beforeEach(() => {
    localStorage.removeItem('duorelay-language')
    setActivePinia(createPinia())
    variant.value = 'full'
    vi.clearAllMocks()
    vi.mocked(previewChain).mockImplementation(async ids => ({ supported: true, kernel: 'sing-box', cross_kernel: false, udp: false, warnings: [], hops: ids.map(id => ({ id, name: id, protocol: 'http', kernel: 'sing-box', network: 'tcp', udp: false })) }))
    vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  })
  it('disables lite-exclusive protocols in chain selection and drops preselected unsupported nodes', async () => {
    variant.value = 'lite'
    const unsupported = { ...nodes[0], id: 'naive', protocol: 'naive' as const, owner: 1 }
    const wrapper = await render('chain', ['a', 'naive'], [...nodes, unsupported])
    const option = wrapper.findAllComponents({ name: 'ElOption' }).find(item => item.props('value') === 'naive')!
    expect(option.props('disabled')).toBe(true)
    expect(wrapper.findAll('.group-members li')).toHaveLength(1)
    expect(applyConfig).not.toHaveBeenCalled()
    variant.value = 'full'
    wrapper.unmount()
  })
  async function render(mode: ProxyGroupMode, selectedIds = ['a', 'b'], choices = nodes, defaults = defaultLoadBalance()) {
    const wrapper = mount(ProxyGroupDialog, {
      attachTo: document.body,
      props: { modelValue: false, mode, nodes: choices, selectedIds, defaults },
      // Multiple-select tag measurement requires real layout (covered by the
      // browser check). These tests exercise ordering and start/error handling.
      global: { plugins: [ElementPlus], stubs: { teleport: true, ElSelect: { template: '<div><slot /></div>' }, ElOption: true } }
    })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    return wrapper
  }

  it('submits the edited hop order and closes only after successful start', async () => {
    const wrapper = await render('chain')
    const state: ConfigurationState = { id: 'runtime', mode: 'chain', kernel: 'sing-box', node_ids: ['b', 'a'], applied_at: '' }
    vi.mocked(applyConfig).mockResolvedValue(state)
    await wrapper.get('[aria-label="上移 出口 B"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.group-members li')[0].text()).toContain('出口 B入口节点')
    await wrapper.findAll('button').find(button => button.text() === '启动链式代理')!.trigger('click')
    await flushPromises()
    expect(applyConfig).toHaveBeenCalledWith({ mode: 'chain', node_ids: ['b', 'a'] })
    expect(wrapper.emitted('started')?.[0]).toEqual([state])
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([false])
    wrapper.unmount()
  })

  it('includes saved balance defaults and keeps the dialog open on failure', async () => {
    const wrapper = await render('load_balance')
    vi.mocked(applyConfig).mockRejectedValue(new Error('配置校验失败，原配置已保留'))
    await wrapper.findAll('button').find(button => button.text() === '启动负载均衡')!.trigger('click')
    await flushPromises()
    expect(applyConfig).toHaveBeenCalledWith({ mode: 'load_balance', node_ids: ['a', 'b'], load_balance: defaultLoadBalance() })
    expect(wrapper.find('.el-alert--error').text()).toContain('原配置已保留')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.emitted('busy')).toEqual([[true], [false]])
    wrapper.unmount()
  })

  it('lets fallback members be reordered and submits that priority', async () => {
    const defaults = { ...defaultLoadBalance(), strategy: 'fallback' as const }
    const wrapper = await render('load_balance', ['a', 'b'], nodes, defaults)
    const state: ConfigurationState = { id: 'fallback', mode: 'load_balance', kernel: 'mihomo', node_ids: ['b', 'a'], applied_at: '', load_balance: defaults }
    vi.mocked(applyConfig).mockResolvedValue(state)
    expect(wrapper.text()).toContain('故障切换按下方顺序尝试节点')
    await wrapper.get('[aria-label="上移 出口 B"]').trigger('click')
    await wrapper.findAll('button').find(button => button.text() === '启动负载均衡')!.trigger('click')
    await flushPromises()
    expect(applyConfig).toHaveBeenCalledWith({ mode: 'load_balance', node_ids: ['b', 'a'], load_balance: defaults })
    wrapper.unmount()
  })

  it('allows mixed kernels when the backend can bridge the chain', async () => {
    vi.mocked(previewChain).mockResolvedValue({ supported: true, kernel: 'mihomo', cross_kernel: true, udp: false, hops: [], warnings: [] })
    const wrapper = await render('chain', ['a', 'b'], [{ ...nodes[0], owner: 1 }, { ...nodes[1], owner: 2 }])
    vi.mocked(applyConfig).mockResolvedValue({ id: 'mixed', kernel: 'mihomo', mode: 'chain', node_ids: ['a', 'b'], applied_at: '' })
    expect(wrapper.text()).toContain('双内核桥接')
    await wrapper.findAll('button').find(button => button.text() === '启动链式代理')!.trigger('click')
    await flushPromises()
    expect(applyConfig).toHaveBeenCalledWith({ mode: 'chain', node_ids: ['a', 'b'] })
    wrapper.unmount()
  })

  it('shows a concrete transport limitation and disables start', async () => {
    vi.mocked(previewChain).mockResolvedValue({ supported: false, reason: '第 2 跳需要 UDP，入口 HTTP 无法转发 UDP', cross_kernel: false, udp: false, hops: [], warnings: [] })
    const wrapper = await render('chain')
    expect(wrapper.find('.el-alert--warning').text()).toContain('入口 HTTP 无法转发 UDP')
    expect(wrapper.findAll('button').find(button => button.text() === '启动链式代理')!.attributes('disabled')).toBeDefined()
    expect(applyConfig).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('requires at least two members after removing a hop', async () => {
    const wrapper = await render('chain')
    await wrapper.get('[aria-label="移除 出口 B"]').trigger('click')
    const start = wrapper.findAll('button').find(button => button.text() === '启动链式代理')!
    expect(start.attributes('disabled')).toBeDefined()
    expect(applyConfig).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
