import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    component: () => import('@/views/Dashboard.vue'),
    meta: { title: '仪表盘' }
  },
  {
    path: '/nodes',
    name: 'nodes',
    component: () => import('@/views/Nodes.vue'),
    meta: { title: '节点管理' }
  },
  {
    path: '/subscriptions',
    name: 'subscriptions',
    component: () => import('@/views/Subscriptions.vue'),
    meta: { title: '订阅管理' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, _from, next) => {
  document.title = `${to.meta.title || 'DuoRelay'} - DuoRelay`
  next()
})

export default router
