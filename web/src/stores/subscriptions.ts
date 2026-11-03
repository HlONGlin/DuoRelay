import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { localize } from './appearance'
import {
  createSubscription,
  deleteSubscription,
  listSubscriptions,
  refreshSubscription,
  updateSubscription
} from '@/api/subscriptions'
import type { Subscription, SubscriptionInput } from '@/types'

export const useSubscriptionsStore = defineStore('subscriptions', () => {
  const items = ref<Subscription[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const updatingId = ref('')

  const totalUsed = computed(() =>
    items.value.reduce((sum, item) => sum + (item.used ?? 0), 0)
  )
  const totalQuota = computed(() =>
    items.value.reduce((sum, item) => sum + (item.total ?? 0), 0)
  )
  const updatedCount = computed(
    () => items.value.filter((item) => item.last_status === 'success').length
  )

  async function fetchSubscriptions() {
    loading.value = true
    error.value = null
    try {
      items.value = await listSubscriptions()
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('加载订阅失败', 'Failed to load subscriptions')
    } finally {
      loading.value = false
    }
  }

  async function add(input: SubscriptionInput) {
    error.value = null
    const created = await createSubscription(input)
    items.value = [created, ...items.value]
    return created
  }

  async function edit(id: string, input: SubscriptionInput) {
    error.value = null
    const updated = await updateSubscription(id, input)
    items.value = items.value.map((item) => (item.id === id ? updated : item))
    return updated
  }

  async function remove(id: string) {
    error.value = null
    await deleteSubscription(id)
    items.value = items.value.filter((item) => item.id !== id)
  }

  async function refresh(id: string) {
    updatingId.value = id
    error.value = null
    try {
      const result = await refreshSubscription(id)
      items.value = items.value.map((item) =>
        item.id === id
          ? {
              ...item,
              last_status: 'success',
              last_update: new Date().toISOString()
            }
          : item
      )
      return result
    } catch (err) {
      items.value = items.value.map((item) =>
        item.id === id ? { ...item, last_status: 'error' } : item
      )
      error.value = err instanceof Error ? err.message : localize('更新订阅失败', 'Failed to update subscription')
      throw err
    } finally {
      updatingId.value = ''
    }
  }

  return {
    items,
    loading,
    error,
    updatingId,
    totalUsed,
    totalQuota,
    updatedCount,
    fetchSubscriptions,
    add,
    edit,
    remove,
    refresh
  }
})
