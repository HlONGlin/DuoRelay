import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ElementPlus from 'element-plus'
import AppShell from './AppShell.vue'
import * as kernelApi from '@/api/kernel'

vi.mock('@/api/kernel', () => ({
  getKernelStatus: vi.fn(),
  startKernel: vi.fn(),
  stopKernel: vi.fn(),
  getKernelLogs: vi.fn(),
  getConnections: vi.fn()
}))

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div>home-view</div>' } },
      { path: '/nodes', component: { template: '<div>nodes-view</div>' } },
      { path: '/subscriptions', component: { template: '<div>subs-view</div>' } },
      { path: '/kernel', component: { template: '<div>kernel-view</div>' } },
      { path: '/settings', component: { template: '<div>settings-view</div>' } },
      { path: '/login', component: { template: '<div>login-view</div>' }, meta: { public: true } }
    ]
  })
}

describe('AppShell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    localStorage.removeItem('duorelay-theme')

    vi.stubGlobal('ResizeObserver', ResizeObserverMock)
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn()
      }))
    })
  })

  it('renders navigation and kernel status', async () => {
    vi.mocked(kernelApi.getKernelStatus).mockResolvedValue({
      running: true,
      active_kernel: 'sing-box',
      version: '1.14.4'
    })

    const router = makeRouter()
    await router.push('/')
    await router.isReady()

    const wrapper = mount(AppShell, {
      global: { plugins: [createPinia(), router, ElementPlus] }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('仪表盘')
    expect(wrapper.text()).toContain('节点管理')
    expect(wrapper.text()).toContain('内核运行中')
    expect(wrapper.text()).toContain('sing-box 1.14.4')
    expect(wrapper.text()).toContain('home-view')

    wrapper.unmount()
  })

  it('hides application navigation on public authentication pages', async () => {
    const router = makeRouter()
    await router.push('/login')
    await router.isReady()

    const wrapper = mount(AppShell, {
      global: { plugins: [createPinia(), router, ElementPlus] }
    })
    await flushPromises()

    expect(wrapper.classes()).toContain('auth-page')
    expect(wrapper.find('.sidebar').exists()).toBe(false)
    expect(wrapper.find('.topbar').exists()).toBe(false)
    expect(wrapper.find('.bottom-nav').exists()).toBe(false)
    expect(wrapper.text()).toContain('login-view')
    expect(kernelApi.getKernelStatus).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('navigates through the sidebar menu', async () => {
    vi.mocked(kernelApi.getKernelStatus).mockResolvedValue({
      running: false,
      active_kernel: ''
    })

    const router = makeRouter()
    await router.push('/')
    await router.isReady()

    const wrapper = mount(AppShell, {
      global: { plugins: [createPinia(), router, ElementPlus] }
    })
    await flushPromises()

    const items = wrapper.findAll('.el-menu-item')
    const nodesItem = items.find((item) => item.text().includes('节点管理'))
    expect(nodesItem).toBeTruthy()

    await nodesItem!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/nodes')
    expect(wrapper.text()).toContain('nodes-view')

    wrapper.unmount()
  })

  it('switches and persists the visual theme', async () => {
    vi.mocked(kernelApi.getKernelStatus).mockResolvedValue({
      running: false,
      active_kernel: ''
    })

    const router = makeRouter()
    await router.push('/')
    await router.isReady()

    const wrapper = mount(AppShell, {
      global: { plugins: [createPinia(), router, ElementPlus] }
    })
    await flushPromises()

    expect(document.documentElement.dataset.theme).toBe('light')
    await wrapper.find('.theme-toggle').trigger('click')
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(localStorage.getItem('duorelay-theme')).toBe('dark')

    wrapper.unmount()
  })
})
