<template>
  <el-container class="app-shell">
    <el-aside :width="sidebarWidth" class="sidebar">
      <div class="brand" :class="{ collapsed }">
        <span class="brand-mark">DR</span>
        <span v-if="!collapsed" class="brand-text">DuoRelay</span>
      </div>
      <el-menu
        :default-active="route.path"
        :collapse="collapsed"
        :collapse-transition="false"
        router
        class="nav-menu"
      >
        <el-menu-item v-for="item in menuItems" :key="item.path" :index="item.path">
          <el-icon><component :is="item.icon" /></el-icon>
          <template #title>{{ item.label }}</template>
        </el-menu-item>
      </el-menu>
      <div class="sidebar-footer">
        <el-button text class="collapse-btn" @click="collapsed = !collapsed">
          <el-icon><component :is="collapsed ? Expand : Fold" /></el-icon>
          <span v-if="!collapsed">收起菜单</span>
        </el-button>
      </div>
    </el-aside>

    <el-container class="main-wrap">
      <el-header class="topbar">
        <div class="page-title">{{ pageTitle }}</div>
        <div class="topbar-right">
          <el-tag :type="kernel.statusType" effect="light" round>
            {{ kernel.status.running ? '内核运行中' : '内核未运行' }}
          </el-tag>
          <span v-if="kernel.status.version" class="kernel-version">
            {{ kernel.status.active_kernel }} {{ kernel.status.version }}
          </span>
        </div>
      </el-header>
      <el-main class="page-body">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  Connection,
  Expand,
  Fold,
  Link,
  Monitor,
  Odometer,
  Setting
} from '@element-plus/icons-vue'
import { useKernelStore } from '@/stores/kernel'

const route = useRoute()
const kernel = useKernelStore()

const menuItems = [
  { path: '/', label: '仪表盘', icon: Odometer },
  { path: '/nodes', label: '节点管理', icon: Connection },
  { path: '/subscriptions', label: '订阅管理', icon: Link },
  { path: '/kernel', label: '内核状态', icon: Monitor },
  { path: '/settings', label: '设置', icon: Setting }
]

const collapsed = ref(false)
const isMobile = ref(false)
const sidebarWidth = computed(() => (collapsed.value ? '64px' : '220px'))
const pageTitle = computed(() => String(route.meta.title ?? 'DuoRelay'))

let mediaQuery: MediaQueryList | null = null
let statusTimer: number | undefined

function handleViewport(event: MediaQueryListEvent | MediaQueryList) {
  isMobile.value = event.matches
  if (event.matches) collapsed.value = true
}

onMounted(() => {
  mediaQuery = window.matchMedia('(max-width: 900px)')
  handleViewport(mediaQuery)
  mediaQuery.addEventListener('change', handleViewport)

  void kernel.fetchStatus()
  statusTimer = window.setInterval(() => {
    void kernel.fetchStatus()
  }, 15000)
})

onBeforeUnmount(() => {
  mediaQuery?.removeEventListener('change', handleViewport)
  if (statusTimer) window.clearInterval(statusTimer)
})
</script>

<style scoped>
.app-shell {
  min-height: 100vh;
}

.sidebar {
  display: flex;
  flex-direction: column;
  background: #111827;
  color: #e5e7eb;
  transition: width 0.2s ease;
  overflow: hidden;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 60px;
  padding: 0 18px;
  font-weight: 600;
  letter-spacing: 0.5px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.brand.collapsed {
  justify-content: center;
  padding: 0;
}

.brand-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  background: linear-gradient(135deg, #409eff, #6366f1);
  color: #fff;
  font-size: 13px;
}

.brand-text {
  font-size: 18px;
}

.nav-menu {
  flex: 1;
  border-right: none;
  background: transparent;
  --el-menu-bg-color: transparent;
  --el-menu-text-color: #cbd5e1;
  --el-menu-hover-bg-color: rgba(255, 255, 255, 0.06);
  --el-menu-active-color: #ffffff;
}

.nav-menu :deep(.el-menu-item.is-active) {
  background: rgba(64, 158, 255, 0.18);
  border-right: 3px solid #409eff;
}

.sidebar-footer {
  padding: 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.collapse-btn {
  width: 100%;
  color: #94a3b8;
}

.main-wrap {
  min-width: 0;
  background: var(--bg-color);
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 60px;
  background: #fff;
  border-bottom: 1px solid var(--border-color);
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.kernel-version {
  font-size: 12px;
  color: var(--text-secondary);
}

.page-body {
  padding: 20px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@media (max-width: 900px) {
  .page-body {
    padding: 12px;
  }

  .kernel-version {
    display: none;
  }
}
</style>
