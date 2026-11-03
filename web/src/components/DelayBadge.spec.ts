import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import DelayBadge from './DelayBadge.vue'

describe('DelayBadge', () => {
  beforeEach(() => { localStorage.removeItem('duorelay-language'); setActivePinia(createPinia()) })
  it('renders the delay value', () => {
    const wrapper = mount(DelayBadge, { props: { delay: 120 } })
    expect(wrapper.text()).toContain('120 ms')
    expect(wrapper.classes()).toContain('delay-warning')
  })

  it('renders a placeholder when delay is missing', () => {
    const wrapper = mount(DelayBadge)
    expect(wrapper.text()).toContain('—')
    expect(wrapper.classes()).toContain('delay-info')
  })

  it('marks high latency as danger', () => {
    const wrapper = mount(DelayBadge, { props: { delay: 900 } })
    expect(wrapper.classes()).toContain('delay-danger')
  })
})
