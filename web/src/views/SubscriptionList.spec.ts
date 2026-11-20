import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/subscriptions'
import { useAppearanceStore } from '@/stores/appearance'
import type { Subscription } from '@/types'
import SubscriptionList from './SubscriptionList.vue'

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/api/nodes', () => ({ listNodes: vi.fn().mockResolvedValue([]) }))
vi.mock('@/api/subscriptions', () => ({
  listSubscriptions: vi.fn(), createSubscription: vi.fn(), updateSubscription: vi.fn(),
  refreshSubscription: vi.fn(), deleteSubscription: vi.fn()
}))

const manual: Subscription = { id: 'manual', name: 'Provider', url: 'https://example.test/sub', interval: 0 }

describe('SubscriptionList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.listSubscriptions).mockResolvedValue([])
    vi.mocked(api.createSubscription).mockResolvedValue(manual)
    vi.mocked(api.updateSubscription).mockResolvedValue(manual)
  })

  afterEach(() => { document.body.replaceChildren() })

  async function render() {
    const pinia = createPinia()
    useAppearanceStore(pinia).language = 'en'
    const wrapper = mount(SubscriptionList, { attachTo: document.body, global: { plugins: [pinia, ElementPlus] } })
    await flushPromises()
    return wrapper
  }

  it('creates and edits a manual subscription without changing its zero interval', async () => {
    const wrapper = await render()
    const click = (label: string) => wrapper.findAll('button').find(button => button.text() === label)!.trigger('click')
    await click('Add subscription')
    await wrapper.get('.el-dialog input').setValue(manual.name)
    await wrapper.get('.el-dialog textarea').setValue(manual.url)
    const interval = () => wrapper.get<HTMLInputElement>('.el-input-number input')
    await interval().setValue('0')
    await interval().trigger('change')
    await click('Add')
    await flushPromises()
    expect(api.createSubscription).toHaveBeenCalledWith(expect.objectContaining({ interval: 0 }))
    expect(wrapper.get('.sub-interval').text()).toBe('Manual updates')
    await click('Edit')
    expect(interval().element.value).toBe('0')
    await click('Save')
    await flushPromises()
    expect(api.updateSubscription).toHaveBeenCalledWith(manual.id, expect.objectContaining({ interval: 0 }))
    expect(wrapper.get('.sub-interval').text()).toBe('Manual updates')
    wrapper.unmount()
  })
})
