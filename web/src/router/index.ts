import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { readLanguage } from '@/stores/appearance'

const routes: RouteRecordRaw[] = [
	{
		path: '/login',
		name: 'login',
		component: () => import('@/views/Login.vue'),
		meta: { title: '登录', titleEn: 'Log in', public: true }
	},
	{
		path: '/register',
		name: 'register',
		component: () => import('@/views/Register.vue'),
		meta: { title: '首次设置', titleEn: 'Initial setup', public: true }
	},
  ...(import.meta.env.DEV ? [{
    path: '/design-system',
    name: 'design-system',
    component: () => import('@/views/DesignSystem.vue'),
    meta: { title: '设计系统验证', titleEn: 'Design system' }
  }] : []),
  {
    path: '/',
    name: 'dashboard',
    component: () => import('@/views/Dashboard.vue'),
    meta: { title: '仪表盘', titleEn: 'Dashboard' }
  },
  {
    path: '/nodes',
    name: 'nodes',
    component: () => import('@/views/NodeList.vue'),
    meta: { title: '节点管理', titleEn: 'Nodes' }
  },
  {
    path: '/subscriptions',
    name: 'subscriptions',
    component: () => import('@/views/SubscriptionList.vue'),
    meta: { title: '订阅管理', titleEn: 'Subscriptions' }
  },
  {
    path: '/kernel',
    name: 'kernel',
    component: () => import('@/views/KernelStatus.vue'),
    meta: { title: '内核状态', titleEn: 'Kernel status' }
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/Settings.vue'),
    meta: { title: '设置', titleEn: 'Settings' }
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

let authReady = false

router.beforeEach(async (to) => {
	document.title = `${readLanguage() === 'en' ? to.meta.titleEn ?? 'DuoRelay' : to.meta.title ?? 'DuoRelay'} - DuoRelay`
	const auth = useAuthStore()
	if (!authReady) {
		try {
			await auth.fetchStatus()
			authReady = true
		} catch {
			return '/login'
		}
	}
	if (!auth.status.configured) {
		return to.meta.public && to.name === 'register' ? true : '/register'
	}
	if (!auth.status.authenticated) {
		return to.meta.public && to.name === 'login' ? true : { path: '/login', query: { redirect: to.fullPath } }
	}
	if (to.name === 'login' || to.name === 'register') return '/'
	return true
})

export default router
