import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import NodeCard from './NodeCard.vue'
import { variant } from '@/stores/variant'

afterEach(() => { variant.value = 'full' })
beforeEach(() => { localStorage.removeItem('duorelay-language'); setActivePinia(createPinia()) })
describe('lite protocol availability', () => {
  it.each(['shadowtls', 'naive', 'tor', 'openconnect'] as const)('disables %s before activation with a visible explanation', protocol => {
    variant.value = 'lite'
    const wrapper = mount(NodeCard, { props: { showActions: true, node: { id: protocol, name: protocol, protocol, owner: 1, server: { host: 'example.com', port: 443 } } }, global: { plugins: [ElementPlus] } })
    const use = wrapper.findAll('button')[0]
    expect(use.attributes('disabled')).toBeDefined()
    expect(use.text()).toBe('精简版不支持')
    expect(wrapper.emitted('activate')).toBeUndefined()
    variant.value = 'full'
    wrapper.unmount()
  })
})
