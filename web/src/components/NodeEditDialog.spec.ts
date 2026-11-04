import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { updateNode } from '@/api/nodes'
import NodeEditDialog from './NodeEditDialog.vue'

vi.mock('@/api/nodes', () => ({ updateNode: vi.fn() }))

beforeEach(() => { localStorage.removeItem('duorelay-language'); setActivePinia(createPinia()); vi.clearAllMocks() })

describe('node editor', () => {
  it('initializes an already-open dialog and saves existing connection parameters', async () => {
    vi.mocked(updateNode).mockResolvedValue({ id: 'n1' } as Awaited<ReturnType<typeof updateNode>>)
    const wrapper = mount(NodeEditDialog, { props: { modelValue: true, node: { id: 'n1', name: 'existing', protocol: 'socks5', owner: 3, server: { host: '127.0.0.1', port: 1080 }, proto: { username: 'kept' } } }, global: { plugins: [ElementPlus], stubs: { teleport: true } } })
    await flushPromises()
    const save = wrapper.findAll('button').find(button => button.text() === '保存节点')!
    await save.trigger('click')
    await flushPromises()
    expect(updateNode).toHaveBeenCalledWith('n1', expect.objectContaining({ name: 'existing', server: { host: '127.0.0.1', port: 1080 }, proto: { username: 'kept' } }))
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })
})
