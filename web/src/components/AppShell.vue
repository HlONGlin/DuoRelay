<template>
  <div class="app-shell" :class="{ 'is-collapsed': collapsed, 'drawer-open': drawerOpen, 'auth-page': isAuthPage }">
    <button type="button" v-if="!isAuthPage && drawerOpen" class="drawer-backdrop" aria-label="关闭导航" @click="drawerOpen = false" />
    <aside v-if="!isAuthPage" class="sidebar" aria-label="主导航">
      <el-menu :default-active="route.path" :collapse="collapsed && !drawerOpen" :collapse-transition="false" router class="nav-menu" @select="drawerOpen = false">
        <el-menu-item v-for="item in menuItems" :key="item.path" :index="item.path">
          <UiIcon :name="item.icon" /><template #title><span>{{ item.label }}</span></template>
        </el-menu-item>
      </el-menu>
      <div class="sidebar-footer">
        <button type="button" class="rail-toggle" :aria-label="collapsed ? '展开导航' : '收起导航'" @click="collapsed = !collapsed"><UiIcon name="menu" :size="16" /><span v-if="!collapsed">收起导航</span></button>
      </div>
    </aside>
    <div class="main-wrap">
      <header v-if="!isAuthPage" class="topbar">
        <div class="topbar-left">
          <button type="button" class="mobile-menu icon-button" aria-label="打开导航" :aria-expanded="drawerOpen" @click="drawerOpen = !drawerOpen"><UiIcon name="menu" /></button>
          <span class="page-title">{{ pageTitle }}</span>
        </div>
        <div class="topbar-right">
          <router-link to="/kernel" class="kernel-indicator"><span class="status-dot" :class="{ running: kernel.status.running }" />{{ kernel.status.running ? '内核运行中' : '内核未运行' }}</router-link>
          <span v-if="kernel.status.version" class="kernel-version mono">{{ kernel.status.active_kernel }} {{ kernel.status.version }}</span>
          <button type="button" class="logout-button" @click="handleLogout">退出</button>
          <button type="button" class="theme-toggle icon-button" :aria-label="appearance.theme === 'dark' ? '切换浅色主题' : '切换深色主题'" @click="appearance.toggleTheme()"><UiIcon :name="appearance.theme === 'dark' ? 'sun' : 'moon'" /></button>
        </div>
      </header>
      <main class="page-body"><router-view /></main>
    </div>
    <nav v-if="!isAuthPage" class="bottom-nav" aria-label="移动导航">
      <router-link v-for="item in menuItems" :key="item.path" :to="item.path" :class="{ active: route.path === item.path }"><UiIcon :name="item.icon" /><span>{{ item.short }}</span></router-link>
    </nav>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useKernelStore } from '@/stores/kernel'
import { useAppearanceStore } from '@/stores/appearance'
import UiIcon from './UiIcon.vue'

const route = useRoute()
const router = useRouter()
const kernel = useKernelStore()
const appearance = useAppearanceStore()
const auth = useAuthStore()
const collapsed = ref(false)
const drawerOpen = ref(false)
const isAuthPage = computed(() => route.meta.public === true)
const menuItems = [
  { path: '/', label: '仪表盘', short: '概览', icon: 'grid' },
  { path: '/nodes', label: '节点管理', short: '节点', icon: 'nodes' },
  { path: '/subscriptions', label: '订阅管理', short: '订阅', icon: 'link' },
  { path: '/kernel', label: '内核状态', short: '内核', icon: 'terminal' },
  { path: '/settings', label: '设置', short: '设置', icon: 'settings' }
]
const pageTitle = computed(() => String(route.meta.title ?? menuItems.find(i => i.path === route.path)?.label ?? 'DuoRelay'))
let mediaQuery: MediaQueryList | undefined
let statusTimer: number | undefined
function handleViewport(event: MediaQueryListEvent | MediaQueryList) { collapsed.value = event.matches; drawerOpen.value = false }
function handleEscape(event: KeyboardEvent) { if (event.key === 'Escape') drawerOpen.value = false }
async function handleLogout() { await auth.logout(); await router.replace('/login') }
watch(() => route.path, () => { drawerOpen.value = false })
onMounted(() => {
  document.documentElement.dataset.theme = appearance.theme
  if (isAuthPage.value) return
  mediaQuery = window.matchMedia('(max-width: 1024px)')
  handleViewport(mediaQuery)
  mediaQuery.addEventListener('change', handleViewport)
  window.addEventListener('keydown', handleEscape)
  void kernel.fetchStatus()
  statusTimer = window.setInterval(() => void kernel.fetchStatus(), 15000)
})
onBeforeUnmount(() => {
  mediaQuery?.removeEventListener('change', handleViewport)
  window.removeEventListener('keydown', handleEscape)
  if (statusTimer) window.clearInterval(statusTimer)
})
</script>

<style scoped>
.app-shell { min-height: 100vh; display: grid; grid-template-columns: 220px minmax(0, 1fr); }
.app-shell.auth-page { display: block; }
.sidebar { position: sticky; top: 0; height: 100vh; display: flex; flex-direction: column; background: var(--surface-1); border-right: 1px solid var(--hairline); min-width: 0; z-index: 20; }
.nav-menu { flex: 1; border: 0; padding: 16px 12px; width: 100%; --el-menu-bg-color: transparent; --el-menu-text-color: var(--ink-muted); --el-menu-hover-bg-color: var(--surface-2); --el-menu-active-color: var(--primary-deep); }
.nav-menu :deep(.el-menu-item) { height: 44px; line-height: 44px; padding: 0 12px !important; margin-bottom: 4px; border-radius: var(--rounded-sm); gap: 12px; font-size: var(--typography-p-body-sm-font-size); }
.nav-menu :deep(.el-menu-item.is-active) { background: var(--primary-soft); font-weight: 500; }
.sidebar-footer { display: flex; align-items: center; padding: 12px; border-top: 1px solid var(--hairline); }
.rail-toggle, .icon-button { display: inline-grid; place-items: center; border: 0; border-radius: var(--rounded-sm); background: transparent; color: var(--ink-subtle); width: 32px; height: 32px; flex-shrink: 0; }
.rail-toggle { display: flex; justify-content: flex-start; gap: 12px; width: 100%; padding: 0 12px; font-size: var(--typography-p-caption-font-size); }
.icon-button:hover, .rail-toggle:hover { background: var(--surface-2); color: var(--ink); }
.main-wrap { min-width: 0; }
.topbar { display: flex; justify-content: space-between; align-items: center; gap: 16px; height: var(--top-nav-height); padding: 0 var(--spacing-lg); border-bottom: 1px solid var(--hairline); }
.topbar-left, .topbar-right { display: flex; align-items: center; gap: 14px; }
.page-title { font-size: var(--typography-p-body-sm-font-size); font-weight: 500; white-space: nowrap; }
.kernel-indicator { display: flex; align-items: center; gap: 6px; color: var(--ink-muted); font-size: var(--typography-p-caption-font-size); white-space: nowrap; }
.kernel-version { color: var(--ink-subtle); font-size: var(--typography-p-micro-font-size); }
.logout-button { border: 0; background: transparent; color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); padding: 6px 8px; border-radius: var(--rounded-sm); }
.logout-button:hover { color: var(--danger); background: var(--surface-2); }
.page-body { padding: var(--spacing-lg); }
.mobile-menu, .bottom-nav, .drawer-backdrop { display: none; }
.is-collapsed { grid-template-columns: 64px minmax(0, 1fr); }
.is-collapsed .sidebar-footer { padding: 16px; justify-content: center; }
.is-collapsed .rail-toggle { justify-content: center; padding: 0; }
.is-collapsed .nav-menu { padding: 16px 8px; }
.is-collapsed .nav-menu :deep(.el-menu-item) { justify-content: center; }
@media (max-width: 1024px) { .kernel-version { display: none; } .page-body { padding: 24px; } }
@media (max-width: 768px) {
  .app-shell, .app-shell.is-collapsed { display: block; }
  .sidebar { display: none; }
  .topbar { padding: 0 16px; height: 56px; }
  .mobile-menu { display: inline-grid; }
  .page-body { padding: 20px 16px; }
  .drawer-open .sidebar { display: flex; position: fixed; width: 220px; top: 0; left: 0; }
  .drawer-open .nav-menu :deep(.el-menu-item) { justify-content: flex-start; }
  .drawer-open .sidebar-footer { display: none; }
  .drawer-backdrop { display: block; position: fixed; inset: 0; background: var(--el-overlay-color-light); border: 0; z-index: 19; }
}
@media (max-width: 480px) {
  .mobile-menu { display: none; }
  .topbar-left { gap: 0; }
  .kernel-indicator { font-size: var(--typography-p-micro-font-size); }
  .topbar-right { gap: 8px; }
  .page-body { padding-bottom: 88px; }
  .bottom-nav { display: grid; grid-template-columns: repeat(5, 1fr); position: fixed; bottom: 0; left: 0; right: 0; border-top: 1px solid var(--hairline); background: var(--surface-1); padding: 6px 4px calc(6px + env(safe-area-inset-bottom)); z-index: 30; }
  .bottom-nav a { display: flex; flex-direction: column; align-items: center; gap: 3px; color: var(--ink-subtle); font-size: var(--typography-p-micro-font-size); min-height: 44px; justify-content: center; border-radius: var(--rounded-sm); }
  .bottom-nav a.active { background: var(--primary-soft); color: var(--primary-deep); }
}
</style>
