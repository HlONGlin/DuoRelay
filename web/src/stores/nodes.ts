import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  activateNode,
  deleteNode,
  listNodes,
  testNodeDelay,
  type NodeQuery
} from '@/api/nodes'
import type { ProxyNode } from '@/types'

export const useNodesStore = defineStore('nodes', () => {
  const items = ref<ProxyNode[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const search = ref('')
  const protocolFilter = ref('')
  const subscriptionFilter = ref('')
  const activeNodeId = ref('')

  const protocols = computed(() =>
    Array.from(new Set(items.value.map((node) => node.protocol))).sort()
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

  async function fetchNodes(query: NodeQuery = {}) {
    loading.value = true
    error.value = null
    try {
      items.value = await listNodes(query)
      const active = items.value.find((node) => node.status === 'active')
      activeNodeId.value = active?.id ?? ''
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载节点失败'
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
      error.value = err instanceof Error ? err.message : '激活节点失败'
      throw err
    }
  }

  async function testNode(id: string) {
    error.value = null
    try {
      const result = await testNodeDelay(id)
      items.value = items.value.map((node) =>
        node.id === id
          ? {
              ...node,
              last_delay_ms: result.delay_ms,
              last_test_at: new Date().toISOString(),
              status: 'idle'
            }
          : node
      )
      return result.delay_ms
    } catch (err) {
      items.value = items.value.map((node) =>
        node.id === id ? { ...node, status: 'error' } : node
      )
      error.value = err instanceof Error ? err.message : '测速失败'
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
      error.value = err instanceof Error ? err.message : '删除节点失败'
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
    fetchNodes,
    activate,
    testNode,
    remove,
    resetFilters
  }
})
