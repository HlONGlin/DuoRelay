import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { localize } from './appearance'
import {
  activateNode,
  deleteNode,
  deleteNodes,
  importNodes,
  listNodes,
  testNodeDelay,
  type NodeQuery
} from '@/api/nodes'
import type { ProxyNode } from '@/types'
import { orderedProtocols } from '@/utils/protocols'

export const useNodesStore = defineStore('nodes', () => {
  const items = ref<ProxyNode[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const search = ref('')
  const protocolFilter = ref('')
  const subscriptionFilter = ref('')
  const activeNodeId = ref('')

  const protocols = computed(() =>
    orderedProtocols(Array.from(new Set(items.value.map((node) => node.protocol))))
  )

  const subscriptionIds = computed(() =>
    Array.from(
      new Set(
        items.value
          .map((node) => node.source?.subscription_id)
          .filter((id): id is string => Boolean(id))
      )
    )
  )

  const filtered = computed(() => {
    const keyword = search.value.trim().toLowerCase()
    return items.value.filter((node) => {
      if (protocolFilter.value && node.protocol !== protocolFilter.value) return false
      if (
        subscriptionFilter.value &&
        node.source?.subscription_id !== subscriptionFilter.value
      ) {
        return false
      }
      if (!keyword) return true
      return (
        node.name.toLowerCase().includes(keyword) ||
        node.server.host.toLowerCase().includes(keyword) ||
        node.protocol.toLowerCase().includes(keyword)
      )
    })
  })

  const activeNode = computed(
    () => items.value.find((node) => node.id === activeNodeId.value) ?? null
  )
  const activeNodes = computed(() => items.value.filter((node) => node.status === 'active'))

  async function fetchNodes(query: NodeQuery = {}) {
    loading.value = true
    error.value = null
    try {
      items.value = await listNodes(query)
      const active = items.value.find((node) => node.status === 'active')
      activeNodeId.value = active?.id ?? ''
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('加载节点失败', 'Failed to load nodes')
    } finally {
      loading.value = false
    }
  }

  async function fetchAllNodes() {
    loading.value = true
    error.value = null
    try {
      const result: ProxyNode[] = []
      const limit = 5000
      for (let offset = 0; ; offset += limit) {
        const page = await listNodes({ limit, offset })
        result.push(...page)
        if (page.length < limit) break
      }
      items.value = result
      activeNodeId.value = result.find(node => node.status === 'active')?.id ?? ''
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('加载节点失败', 'Failed to load nodes')
    } finally {
      loading.value = false
    }
  }

  async function activate(id: string) {
    error.value = null
    try {
      await activateNode(id)
      activeNodeId.value = id
      items.value = items.value.map((node) => ({
        ...node,
        status:
          node.id === id ? 'active' : node.status === 'active' ? 'idle' : node.status
      }))
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('激活节点失败', 'Failed to activate node')
      throw err
    }
  }

  async function importContent(content: string) {
    error.value = null
    try {
      const result = await importNodes(content)
      await fetchAllNodes()
      return result
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('导入节点失败', 'Failed to import nodes')
      throw err
    }
  }

  async function testNode(id: string) {
    error.value = null
    try {
      const result = await testNodeDelay(id)
      const delay = result.success === false ? -1 : result.delay_ms
      items.value = items.value.map((node) =>
        node.id === id
          ? {
              ...node,
              last_delay_ms: delay,
              last_test_at: new Date().toISOString(),
              status: node.status === 'active' ? 'active' : delay < 0 ? 'error' : 'idle'
            }
          : node
      )
      return delay
    } catch (err) {
      items.value = items.value.map((node) =>
        node.id === id ? { ...node, last_delay_ms: -1, status: node.status === 'active' ? 'active' : 'error' } : node
      )
      error.value = err instanceof Error ? err.message : localize('测速失败', 'Latency test failed')
      throw err
    }
  }

  async function remove(id: string) {
    error.value = null
    try {
      await deleteNode(id)
      items.value = items.value.filter((node) => node.id !== id)
      if (activeNodeId.value === id) activeNodeId.value = ''
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('删除节点失败', 'Failed to delete node')
      throw err
    }
  }

  async function removeMany(ids: string[]) {
    error.value = null
    try {
      await deleteNodes(ids)
      const removed = new Set(ids)
      items.value = items.value.filter((node) => !removed.has(node.id))
      if (removed.has(activeNodeId.value)) activeNodeId.value = ''
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('批量删除节点失败', 'Failed to delete nodes')
      throw err
    }
  }

  function resetFilters() {
    search.value = ''
    protocolFilter.value = ''
    subscriptionFilter.value = ''
  }

  return {
    items,
    loading,
    error,
    search,
    protocolFilter,
    subscriptionFilter,
    activeNodeId,
    protocols,
    subscriptionIds,
    filtered,
    activeNode,
    activeNodes,
    fetchNodes,
    fetchAllNodes,
    activate,
    importContent,
    testNode,
    remove,
    removeMany,
    resetFilters
  }
})
