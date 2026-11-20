import { request } from './client'
import type { Subscription, SubscriptionInput, SubscriptionSkip } from '@/types'

export function listSubscriptions(): Promise<Subscription[]> {
  return request<Subscription[]>({ url: '/subscriptions', method: 'get' })
}

export function createSubscription(input: SubscriptionInput): Promise<Subscription> {
  return request<Subscription>({ url: '/subscriptions', method: 'post', data: input })
}

export function updateSubscription(id: string, input: SubscriptionInput): Promise<Subscription> {
  return request<Subscription>({
    url: `/subscriptions/${encodeURIComponent(id)}`,
    method: 'put',
    data: input
  })
}

export function setSubscriptionSchedule(id: string, disabled: boolean): Promise<Subscription> {
  return request<Subscription>({
    url: `/subscriptions/${encodeURIComponent(id)}/schedule`,
    method: 'patch',
    data: { disabled }
  })
}

export function deleteSubscription(id: string): Promise<void> {
  return request<void>({ url: `/subscriptions/${encodeURIComponent(id)}`, method: 'delete' })
}

export interface UpdateResult {
  nodes_added: number
  nodes_removed: number
  nodes_changed: number
  nodes_total: number
  duplicates: number
  skipped: SubscriptionSkip[] | null
  subscription: Subscription
}

export function refreshSubscription(id: string): Promise<UpdateResult> {
  return request<UpdateResult>({
    url: `/subscriptions/${encodeURIComponent(id)}/update`,
    method: 'post'
  })
}
