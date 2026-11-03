import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import Settings from './Settings.vue'
import { defaultConfig } from '@/stores/config'
import * as configApi from '@/api/config'
import * as nodeApi from '@/api/nodes'

vi.mock('@/api/config', () => ({ getConfig: vi.fn(), updateConfig: vi.fn() }))
vi.mock('@/api/nodes', () => ({ listNodes: vi.fn(), activateNode: vi.fn(), deleteNode: vi.fn(), testNodeDelay: vi.fn() }))

describe('settings workflows', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
    vi.mocked(nodeApi.listNodes).mockResolvedValue([])
    const config = defaultConfig()
    config.transparent_modes = [
      { mode: 'off', supported: true, prerequisites: '', dns_description: '' },
      { mode: 'tun', supported: true, prerequisites: '', dns_description: '' }
    ]
    config.routing_rules.unshift({ match: 'domain_suffix', values: ['example.com'], outbound: 'PROXY' })
    vi.mocked(configApi.getConfig).mockResolvedValue(config)
    vi.mocked(configApi.updateConfig).mockImplementation(async next => next)
    localStorage.removeItem('duorelay-theme')
  })

  async function render() {
    const wrapper = mount(Settings, { global: { plugins: [createPinia(), ElementPlus], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    return wrapper
  }

  it('preserves custom rules while switching the routing preset', async () => {
    const wrapper = await render()
    await wrapper.findAll('.category-nav button').find(button => button.text() === '路由规则')!.trigger('click')
    await wrapper.findAll('.el-radio-button').find(button => button.text() === '全局')!.find('input').setValue(true)
    await wrapper.find('.save-bar button').trigger('click')
    await flushPromises()
    expect(configApi.updateConfig).toHaveBeenCalledWith(expect.objectContaining({
      routing_mode: 'global', default_outbound: 'PROXY',
      routing_rules: [{ id: 'rule-1', source: 'user', enabled: true, match: 'domain_suffix', values: ['example.com'], outbound: 'PROXY' }]
    }))
    wrapper.unmount()
  })

  it('opens the editor when clicking a rule value', async () => {
    const wrapper = await render()
    await wrapper.findAll('.category-nav button').find(button => button.text() === '路由规则')!.trigger('click')
    const ruleButton = wrapper.find('.rule-summary')
    expect(ruleButton.attributes('type')).toBe('button')
    await ruleButton.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('编辑路由规则')
    expect((wrapper.find('.rule-input textarea').element as HTMLTextAreaElement).value).toBe('example.com')
    expect(configApi.updateConfig).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('applies and persists appearance without saving backend configuration', async () => {
    const wrapper = await render()
    await wrapper.findAll('.category-nav button').find(button => button.text() === '外观')!.trigger('click')
    expect(wrapper.findAll('.theme-option').every(button => button.attributes('type') === 'button')).toBe(true)
    await wrapper.findAll('.theme-option')[1].trigger('click')
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(localStorage.getItem('duorelay-theme')).toBe('dark')
    expect(wrapper.find('.save-bar').exists()).toBe(false)
    expect(configApi.updateConfig).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('saves the dedicated SOCKS-only ingress without changing mixed ports', async () => {
    const wrapper = await render()
    await wrapper.findAll('.el-switch')[1].trigger('click')
    expect(wrapper.text()).toContain('独立 SOCKS5 端口')
    await wrapper.find('.save-bar button').trigger('click')
    await flushPromises()
    expect(configApi.updateConfig).toHaveBeenCalledWith(expect.objectContaining({
      socks_only: true, socks_port: 10808, inbound_port: 1080, mixed_port: 7890
    }))
    wrapper.unmount()
  })

  it('keeps TUN takeover and SOCKS-only mode mutually exclusive', async () => {
    const wrapper = await render()
    await wrapper.find('[aria-label="仅 SOCKS5 端口使用代理"]').trigger('click')
    await wrapper.findAll('.category-nav button').find(button => button.text() === '内核')!.trigger('click')
    await wrapper.find('[aria-label="TUN 全流量接管"]').trigger('click')
    await wrapper.find('.save-bar button').trigger('click')
    await flushPromises()
    expect(configApi.updateConfig).toHaveBeenCalledWith(expect.objectContaining({ socks_only: false, enable_tun: true }))
    wrapper.unmount()
  })

  it('provides a separate category for persistent load balance defaults', async () => {
    const wrapper = await render()
    await wrapper.findAll('.category-nav button').find(button => button.text() === '负载均衡')!.trigger('click')
    expect(wrapper.text()).toContain('分配策略')
    expect(wrapper.text()).toContain('健康检查地址')
    await wrapper.find('.save-bar button').trigger('click')
    await flushPromises()
    expect(configApi.updateConfig).toHaveBeenCalledWith(expect.objectContaining({ load_balance: defaultConfig().load_balance }))
    wrapper.unmount()
  })
})
