import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'dashboard',
    component: () => import('@/views/Dashboard.vue'),
    meta: { title: '仪表盘' }
  },
  {
    path: '/nodes',
    name: 'nodes',
    component: () => import('@/views/NodeList.vue'),
    meta: { title: '节点管理' }
  },
  {
    path: '/subscriptions',
    name: 'subscriptions',
    component: () => import('@/views/SubscriptionList.vue'),
    meta: { title: '订阅管理' }
  },
  {
    path: '/kernel',
    name: 'kernel',
    component: () => import('@/views/KernelStatus.vue'),
    meta: { title: '内核状态' }
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/Settings.vue'),
    meta: { title: '设置' }
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, _from, next) => {
  document.title = `${to.meta.title ?? 'DuoRelay'} - DuoRelay`
  next()
})

export default router
