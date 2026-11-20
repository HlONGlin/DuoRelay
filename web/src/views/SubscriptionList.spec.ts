import { flushPromises, mount } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
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
  refreshSubscription: vi.fn(), deleteSubscription: vi.fn(), setSubscriptionSchedule: vi.fn()
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

  it('pauses and resumes automatic updates while keeping manual refresh available', async () => {
    const automatic = { ...manual, interval: 24, disabled: false }
    vi.mocked(api.listSubscriptions).mockResolvedValue([automatic])
    vi.mocked(api.setSubscriptionSchedule).mockResolvedValueOnce({ ...automatic, disabled: true }).mockResolvedValueOnce(automatic)
    const wrapper = await render()
    const button = (label: string) => wrapper.findAll('button').find(button => button.text() === label)!
    await button('Pause automatic updates').trigger('click')
    await flushPromises()
    expect(api.setSubscriptionSchedule).toHaveBeenNthCalledWith(1, automatic.id, true)
    expect(wrapper.text()).toContain('Automatic updates paused')
    expect(button('Update').attributes('disabled')).toBeUndefined()
    await button('Resume automatic updates').trigger('click')
    await flushPromises()
    expect(api.setSubscriptionSchedule).toHaveBeenNthCalledWith(2, automatic.id, false)
    expect(wrapper.text()).not.toContain('Automatic updates paused')
    wrapper.unmount()
  })

  it.each([true, false])('shows the server result after refresh (partial: %s)', async (partial) => {
    vi.mocked(api.listSubscriptions).mockResolvedValue([{ ...manual, skipped: [{ line: 1, reason: 'Old reason' }] }])
    const skipped = partial ? [{ line: 3, reason: 'Unsupported transport' }] : null
    const subscription = { ...manual, last_status: 'ok', last_update: '2026-01-03T10:00:00Z', skipped: skipped ?? undefined,
      diff: { added: 2, removed: 1, changed: 0, duplicates: 1 } }
    vi.mocked(api.refreshSubscription).mockResolvedValue({ nodes_added: 2, nodes_removed: 1, nodes_changed: 0,
      nodes_total: 4, duplicates: 1, skipped, subscription })
    const warning = vi.spyOn(ElMessage, 'warning')
    const success = vi.spyOn(ElMessage, 'success')
    const wrapper = await render()
    await wrapper.findAll('button').find(button => button.text() === 'Update')!.trigger('click')
    await flushPromises()
    expect(wrapper.findComponent({ name: 'SubscriptionCard' }).props('subscription')).toEqual(subscription)
    expect(wrapper.text()).not.toContain('Old reason')
    if (partial) {
      expect(warning).toHaveBeenCalledWith('Subscription partially updated; 1 entries skipped')
      expect(success).not.toHaveBeenCalled()
      expect(wrapper.text()).toContain('Unsupported transport')
    } else {
      expect(success).toHaveBeenCalledWith('Subscription updated; 2 nodes added')
      expect(warning).not.toHaveBeenCalled()
      expect(wrapper.find('details').exists()).toBe(false)
    }
    wrapper.unmount()
    warning.mockRestore()
    success.mockRestore()
  })
})
