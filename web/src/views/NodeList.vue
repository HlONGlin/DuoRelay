<template>
  <div class="page-stack node-list">
    <div class="page-heading"><h1>节点管理 <span class="heading-count mono">{{ nodes.items.length }}</span></h1><div class="actions"><el-button :disabled="nodes.items.length < 2 || mutationBusy || kernel.busy" @click="openGroup('chain')">链式代理</el-button><el-button :disabled="nodes.items.length < 2 || mutationBusy || kernel.busy" @click="openGroup('load_balance')">负载均衡</el-button><el-button :loading="nodes.loading" @click="refresh"><UiIcon name="refresh" :size="16" />刷新</el-button><el-button @click="importVisible = true"><UiIcon name="plus" :size="16" />导入节点</el-button></div></div>
    <el-alert v-if="isLite" title="精简版不支持 ShadowTLS、Naive、Tor、OpenConnect；这些节点会保留，可切换完整版使用。" type="info" :closable="false" />
    <el-alert v-if="nodes.error" :title="nodes.error" type="error" show-icon :closable="false" />
    <div class="active-banner">
      <div class="active-banner-content"><span class="status-dot" :class="{ running: nodes.activeNodes.length > 0 }" /><div><strong>{{ activeSummary }}</strong><p v-if="!nodes.activeNodes.length">可单独启动，或勾选多个节点组成负载均衡、链式代理。</p><p v-else-if="activeState?.mode === 'chain'">本机 → {{ activeNames.join(' → ') }} → 目标网站</p><p v-else-if="activeState?.mode === 'load_balance'">{{ balanceStrategyLabel }} · {{ activeState.kernel }} · 新连接按策略分配到健康节点</p></div></div>
      <div v-if="activeState?.mode === 'load_balance' && proxyGroupStatus?.active" class="proxy-group-status" aria-live="polite">
        <p>当前节点：<strong>{{ proxyGroupStatus.now || '健康检查中' }}</strong></p>
        <ul><li v-for="member in proxyGroupStatus.members" :key="member.name"><span class="status-dot" :class="{ running: member.alive }" /><span>{{ member.name }}</span><span>{{ member.alive ? '健康' : '不可用' }}</span><span class="mono">{{ member.delay_ms != null ? member.delay_ms + ' ms' : '未测速' }}</span></li></ul>
      </div>
      <p v-else-if="proxyGroupStatusError" class="proxy-group-status-error" role="status">{{ proxyGroupStatusError }}</p>
      <el-button v-if="nodes.activeNodes.length" :loading="kernel.busy" :disabled="mutationBusy" @click="handleStopCurrent">停止并取消选择</el-button>
    </div>
    <section class="node-panel">
      <div class="filter-bar">
        <el-input v-model="nodes.search" placeholder="搜索名称、服务器或协议" clearable class="search-input" aria-label="搜索节点"><template #prefix><UiIcon name="search" :size="16" /></template></el-input>
        <el-select v-model="nodes.protocolFilter" placeholder="全部协议" clearable class="filter-select" aria-label="筛选协议"><el-option-group v-for="group in protocolGroups" :key="group.id" :label="group.label"><el-option v-for="protocol in group.protocols" :key="protocol" :label="protocolLabel(protocol)" :value="protocol" /></el-option-group></el-select>
        <el-select v-model="nodes.subscriptionFilter" placeholder="全部订阅" clearable class="filter-select" aria-label="筛选订阅"><el-option v-for="id in nodes.subscriptionIds" :key="id" :label="subscriptionLabel(id)" :value="id" /></el-select>
        <el-button v-if="hasFilters" text @click="nodes.resetFilters">清除筛选</el-button>
        <div class="filter-spacer" />
        <el-select v-model="density" class="density-select" aria-label="列表密度"><el-option label="舒适" value="comfortable" /><el-option label="紧凑" value="compact" /></el-select>
      </div>
      <div class="batch-bar">
        <span class="muted type-p-caption">{{ selected.size ? '已勾选 ' + selected.size + ' 个' : '匹配 ' + sorted.length + ' 个节点' }}</span>
        <el-button v-if="selected.size" size="small" text @click="selected.clear()">取消勾选</el-button>
        <div class="filter-spacer" />
        <span v-if="batchRunning" class="batch-progress mono" role="status">{{ batchDone }} / {{ batchTotal }}</span>
        <el-button v-if="batchRunning" size="small" @click="cancelBatch = true">停止测速</el-button>
        <el-button v-else type="primary" size="small" :disabled="!sorted.length || mutationBusy" @click="testBatch">{{ selected.size ? '测速勾选节点' : '一键测速' }}</el-button>
      </div>
      <el-progress v-if="batchRunning" :percentage="batchTotal ? Math.round(batchDone / batchTotal * 100) : 0" :stroke-width="2" :show-text="false" />
      <div ref="scrollArea" v-loading="nodes.loading" class="node-scroll" :style="{ height: sorted.length ? Math.min(560, 36 + sorted.length * rowHeight) + 'px' : '280px' }" @scroll="onScroll">
        <table v-if="sorted.length" class="node-table" :style="{ '--row-height': rowHeight + 'px' }" aria-label="节点列表" :aria-rowcount="sorted.length + 1">
          <colgroup><col style="width:44px" /><col style="width:240px" /><col style="width:120px" /><col style="width:110px" /><col style="width:210px" /><col style="width:140px" /><col style="width:260px" /></colgroup>
          <thead><tr><th><input type="checkbox" aria-label="勾选全部筛选结果" :checked="allSelected" :indeterminate="someSelected && !allSelected" @change="toggleAll" /></th><th><button type="button" class="sort-button" @click="toggleSort('name')">节点名称 <span>{{ sortIndicator('name') }}</span></button></th><th>协议</th><th>支持内核</th><th>服务器</th><th class="numeric"><button type="button" class="sort-button" @click="toggleSort('delay')">延迟 <span>{{ sortIndicator('delay') }}</span></button></th><th class="row-actions-head">操作</th></tr></thead>
          <tbody>
            <tr v-if="startIndex" aria-hidden="true" class="spacer-row" :style="{ height: startIndex * rowHeight + 'px' }"><td colspan="7" /></tr>
            <tr v-for="(node, index) in visibleNodes" :key="node.id" class="node-row" :class="{ 'is-active': node.status === 'active', 'is-failed': (node.last_delay_ms ?? 0) < 0 }" :aria-rowindex="startIndex + index + 2">
              <td><input type="checkbox" :disabled="Boolean(nodeUnavailableReason(node))" :title="nodeUnavailableReason(node)" :checked="selected.has(node.id)" :aria-label="'勾选 ' + node.name" @change="toggleSelected(node.id)" /></td>
              <td><div class="node-name" :title="node.name"><span v-if="node.status === 'active'" class="status-dot running" /><span>{{ node.name }}</span><span v-if="node.status === 'active'" class="active-label">使用中</span></div></td>
              <td><span class="neutral-badge" :title="protocolDescription(node.protocol)">{{ protocolLabel(node.protocol) }}</span></td>
              <td class="kernel-cell">{{ ownerLabel(node.owner) }}</td>
              <td class="endpoint mono" :title="nodeEndpointLabel(node)">{{ nodeEndpointLabel(node) }}</td>
              <td class="numeric"><span v-if="testing.has(node.id)" class="muted type-p-caption">测试中…</span><DelayBadge v-else :delay="node.last_delay_ms" /></td>
              <td><div class="row-actions"><el-button v-if="node.status !== 'active'" class="activate-button" size="small" :loading="activatingId === node.id" :disabled="mutationBusy || kernel.busy || Boolean(nodeUnavailableReason(node))" :title="nodeUnavailableReason(node)" @click="handleActivate(node)">{{ nodeUnavailableReason(node) || '选择并启动' }}</el-button><el-button v-else size="small" :loading="kernel.busy" :disabled="mutationBusy" @click="handleStopCurrent">{{ nodes.activeNodes.length > 1 ? '停止组合' : '取消选择' }}</el-button><el-button size="small" text :disabled="testing.has(node.id) || batchRunning || groupBusy" @click="handleTest(node)">测速</el-button><el-button size="small" text :disabled="node.status === 'active' || testing.has(node.id) || batchRunning || mutationBusy" @click="handleRemove(node)">删除</el-button></div></td>
            </tr>
            <tr v-if="endIndex < sorted.length" aria-hidden="true" class="spacer-row" :style="{ height: (sorted.length - endIndex) * rowHeight + 'px' }"><td colspan="7" /></tr>
          </tbody>
        </table>
        <EmptyState v-else :title="hasFilters ? '没有匹配的节点' : '暂无节点'" :description="hasFilters ? '请调整搜索或筛选条件。' : '支持导入节点链接或订阅内容。'"><el-button v-if="hasFilters" @click="nodes.resetFilters">清除筛选</el-button><el-button v-else @click="importVisible = true">导入节点</el-button></EmptyState>
      </div>
      <div class="table-footer"><span>勾选节点后，可批量测速或配置负载均衡、链式代理；点击启动后生效。</span><span class="mono">{{ sorted.length }} 个节点</span></div>
    </section>
    <el-dialog v-model="importVisible" title="导入节点或订阅内容" width="620px" destroy-on-close>
      <p class="import-hint">支持 v2rayN 链接合集、Base64、Clash、sing-box、Xray JSON、SIP008、SSD 及 Surge / Loon / Quantumult X 节点配置，包括 AnyTLS 和 Mieru 原生链接。</p>
      <el-input v-model="importContent" type="textarea" :rows="10" placeholder="粘贴链接或配置内容，每行一个节点链接" aria-label="节点链接或配置" />
      <template #footer><el-button @click="importVisible = false">取消</el-button><el-button type="primary" :loading="importLoading" @click="submitImport">导入节点</el-button></template>
    </el-dialog>
    <ProxyGroupDialog v-model="groupVisible" :mode="groupMode" :nodes="nodes.items" :selected-ids="groupSelection" :defaults="config.config.load_balance" @started="groupStarted" @busy="groupBusy = $event" />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import DelayBadge from '@/components/DelayBadge.vue'
import EmptyState from '@/components/EmptyState.vue'
import UiIcon from '@/components/UiIcon.vue'
import ProxyGroupDialog from '@/components/ProxyGroupDialog.vue'
import { getCurrentConfig, getProxyGroupStatus } from '@/api/config'
import { useConfigStore } from '@/stores/config'
import { useNodesStore } from '@/stores/nodes'
import { useKernelStore } from '@/stores/kernel'
import { isLite, nodeUnavailableReason } from '@/stores/variant'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import { nodeEndpointLabel, ownerLabel, protocolDescription, protocolLabel } from '@/utils/format'
import { protocolCatalog, protocolGroupLabel, type ProtocolGroup } from '@/utils/protocols'
import type { ConfigurationState, ProxyGroupMode, ProxyGroupStatus, ProxyNode } from '@/types'

const nodes = useNodesStore()
const kernel = useKernelStore()
const subscriptions = useSubscriptionsStore()
const config = useConfigStore()
const activeState = ref<ConfigurationState | null>(null)
const proxyGroupStatus = ref<ProxyGroupStatus | null>(null)
const proxyGroupStatusError = ref('')
const groupVisible = ref(false), groupBusy = ref(false)
const groupMode = ref<ProxyGroupMode>('load_balance')
const groupSelection = ref<string[]>([])
const route = useRoute()
const importVisible = ref(false), importLoading = ref(false), importContent = ref('')
const activatingId = ref('')
const mutationBusy = computed(() => Boolean(activatingId.value) || groupBusy.value)
const activeNames = computed(() => (activeState.value?.node_ids ?? nodes.activeNodes.map(node => node.id)).map(id => nodes.items.find(node => node.id === id)?.name ?? '节点已移除'))
const activeSummary = computed(() => !nodes.activeNodes.length ? '未选择节点' : activeState.value?.mode === 'load_balance' ? `负载均衡 · ${nodes.activeNodes.length} 个节点` : activeState.value?.mode === 'chain' ? `链式代理 · ${nodes.activeNodes.length} 跳` : '当前节点：' + nodes.activeNodes.map(node => node.name).join('、'))
const balanceStrategyLabel = computed(() => ({ 'round-robin': '轮询', 'consistent-hashing': '一致性哈希', 'sticky-sessions': '会话保持', 'url-test': '自动选优', fallback: '故障切换' })[activeState.value?.load_balance?.strategy ?? 'round-robin'])
const testing = ref(new Set<string>())
const selected = ref(new Set<string>())
const density = ref('compact')
const isTouch = ref(false)
const rowHeight = computed(() => density.value === 'compact' && !isTouch.value ? 36 : 44)
const scrollArea = ref<HTMLElement>()
const scrollTop = ref(0), viewportHeight = ref(520)
const sortKey = ref<'name' | 'delay' | ''>(''), sortDescending = ref(false)
const sorted = computed(() => {
  if (!sortKey.value) return nodes.filtered
  return [...nodes.filtered].sort((a, b) => {
    if (sortKey.value === 'name') return a.name.localeCompare(b.name, 'zh-CN') * (sortDescending.value ? -1 : 1)
    const validA = (a.last_delay_ms ?? 0) > 0, validB = (b.last_delay_ms ?? 0) > 0
    if (validA !== validB) return validA ? -1 : 1
    return ((a.last_delay_ms ?? 0) - (b.last_delay_ms ?? 0)) * (sortDescending.value ? -1 : 1)
  })
})
const startIndex = computed(() => Math.min(Math.max(0, sorted.value.length - 1), Math.max(0, Math.floor(Math.max(0, scrollTop.value - 36) / rowHeight.value) - 6)))
const endIndex = computed(() => Math.min(sorted.value.length, startIndex.value + Math.ceil(viewportHeight.value / rowHeight.value) + 12))
const visibleNodes = computed(() => sorted.value.slice(startIndex.value, endIndex.value))
const allSelected = computed(() => sorted.value.length > 0 && sorted.value.filter(n => !nodeUnavailableReason(n)).every(n => selected.value.has(n.id)))
const someSelected = computed(() => sorted.value.some(n => selected.value.has(n.id)))
function toggleSelected(id: string) { if (selected.value.has(id)) selected.value.delete(id); else selected.value.add(id) }
function toggleAll() { const remove = allSelected.value; for (const n of sorted.value.filter(n => !nodeUnavailableReason(n))) { if (remove) selected.value.delete(n.id); else selected.value.add(n.id) } }
function toggleSort(key: 'name' | 'delay') { sortDescending.value = sortKey.value === key ? !sortDescending.value : false; sortKey.value = key }
function sortIndicator(key: string) { return sortKey.value === key ? (sortDescending.value ? '↓' : '↑') : '↕' }
function onScroll(event: Event) { scrollTop.value = (event.target as HTMLElement).scrollTop }
watch([() => nodes.search, () => nodes.protocolFilter, () => nodes.subscriptionFilter, sortKey, sortDescending, rowHeight], () => { scrollTop.value = 0; if (scrollArea.value) scrollArea.value.scrollTop = 0 })
watch(() => nodes.items, items => { const ids = new Set(items.map(n => n.id)); selected.value = new Set([...selected.value].filter(id => ids.has(id))) })

const batchRunning = ref(false), batchDone = ref(0), batchTotal = ref(0)
const cancelBatch = ref(false)
async function testBatch() {
  const targets = selected.value.size ? nodes.items.filter(n => selected.value.has(n.id)) : [...sorted.value]
  batchRunning.value = true; batchDone.value = 0; batchTotal.value = targets.length; cancelBatch.value = false
  let next = 0, failed = 0
  await Promise.all(Array.from({ length: Math.min(4, targets.length) }, async () => {
    while (!cancelBatch.value && next < targets.length) {
      const node = targets[next++]
      testing.value.add(node.id)
      try { const delay = await nodes.testNode(node.id); if (delay <= 0) failed++ } catch { failed++ }
      finally { testing.value.delete(node.id); batchDone.value++ }
    }
  }))
  batchRunning.value = false
  if (!disposed) ElMessage.info((cancelBatch.value ? '测速已停止' : '测速完成') + '：已测 ' + batchDone.value + ' 个，失败 ' + failed + ' 个')
}
const protocolGroups = computed(() => {
  const available = new Set(nodes.protocols)
  return (['shared', 'sing-box', 'mihomo'] as ProtocolGroup[]).map(id => ({
    id, label: protocolGroupLabel(id), protocols: protocolCatalog.filter(item => item.group === id && available.has(item.id)).map(item => item.id)
  })).filter(group => group.protocols.length > 0)
})
const hasFilters = computed(() => Boolean(nodes.search || nodes.protocolFilter || nodes.subscriptionFilter))
function subscriptionLabel(id: string) { return subscriptions.items.find(item => item.id === id)?.name ?? id }
async function fetchActiveState() {
  try { activeState.value = await getCurrentConfig() } catch { activeState.value = null }
  await fetchProxyGroupStatus()
}
async function fetchProxyGroupStatus() {
  if (activeState.value?.mode !== 'load_balance' || activeState.value.kernel !== 'mihomo') {
    proxyGroupStatus.value = null
    proxyGroupStatusError.value = ''
    return
  }
  try {
    proxyGroupStatus.value = await getProxyGroupStatus()
    proxyGroupStatusError.value = ''
  } catch (err) {
    proxyGroupStatus.value = null
    proxyGroupStatusError.value = err instanceof Error ? err.message : '无法读取节点健康状态'
  }
}
async function refresh() { await Promise.all([nodes.fetchAllNodes(), fetchActiveState()]) }
function openGroup(mode: ProxyGroupMode) {
  groupMode.value = mode
  groupSelection.value = selected.value.size ? [...selected.value] : activeState.value?.mode === mode ? [...activeState.value.node_ids] : []
  groupVisible.value = true
}
async function groupStarted(state: ConfigurationState) {
  activeState.value = state
  await fetchProxyGroupStatus()
  await Promise.all([nodes.fetchAllNodes(), kernel.fetchStatus()])
  ElMessage.success(state.mode === 'chain' ? '链式代理已启动' : '负载均衡已启动')
}
async function submitImport() {
  if (!importContent.value.trim()) { ElMessage.warning('请粘贴节点链接或订阅内容'); return }
  importLoading.value = true
  try { const result = await nodes.importContent(importContent.value.trim()); ElMessage.success('已导入 ' + result.nodes_total + ' 个节点'); importContent.value = ''; importVisible.value = false }
  catch (err) { ElMessage.error(err instanceof Error ? err.message : '导入节点失败') }
  finally { importLoading.value = false }
}
async function handleActivate(node: ProxyNode) {
  if (nodeUnavailableReason(node)) return
  if (mutationBusy.value) return
  activatingId.value = node.id
  try { await nodes.activate(node.id); await Promise.all([kernel.fetchStatus(), fetchActiveState()]); ElMessage.success('已选择并启动：' + node.name) }
  catch (err) { ElMessage.error(err instanceof Error ? err.message : '启动节点失败') }
  finally { activatingId.value = '' }
}
async function handleStopCurrent() {
  try {
    await ElMessageBox.confirm(nodes.activeNodes.length > 1 ? '这会停止整个代理组合，并取消所有成员节点的运行选择。' : '停止后会清除当前节点选择，代理也会停止。', '停止并取消选择', { type: 'warning', confirmButtonText: '停止并取消选择', cancelButtonText: '保留运行' })
    await kernel.stop(); activeState.value = null; proxyGroupStatus.value = null; await nodes.fetchAllNodes(); ElMessage.success('已停止代理并取消当前节点选择')
  } catch (err) { if (err !== 'cancel' && err !== 'close') ElMessage.error(err instanceof Error ? err.message : '停止代理失败') }
}
async function handleTest(node: ProxyNode) {
  testing.value.add(node.id)
  try { const delay = await nodes.testNode(node.id); if (delay > 0) ElMessage.success(node.name + '：' + delay + ' ms'); else ElMessage.warning(node.name + '：测速失败') }
  catch (err) { ElMessage.error(err instanceof Error ? err.message : '测速失败') }
  finally { testing.value.delete(node.id) }
}
async function handleRemove(node: ProxyNode) {
  try {
    await ElMessageBox.confirm('确定删除节点「' + node.name + '」吗？', '删除节点', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    await nodes.remove(node.id); selected.value.delete(node.id); ElMessage.success('节点已删除')
  } catch (err) { if (err !== 'cancel' && err !== 'close') ElMessage.error(err instanceof Error ? err.message : '删除节点失败') }
}
let observer: ResizeObserver | undefined
let media: MediaQueryList | undefined
let disposed = false
let proxyStatusTimer: number | undefined
function touchChange(event: MediaQueryListEvent | MediaQueryList) { isTouch.value = event.matches }
onMounted(() => {
  void refresh()
  proxyStatusTimer = window.setInterval(() => { void fetchActiveState() }, 5000)
  void config.fetchConfig()
  if (route.query.import === '1') importVisible.value = true
  if (!subscriptions.items.length) void subscriptions.fetchSubscriptions()
  media = window.matchMedia('(max-width: 768px), (pointer: coarse)'); touchChange(media); media.addEventListener('change', touchChange)
  if (scrollArea.value) { observer = new ResizeObserver(([entry]) => { viewportHeight.value = entry.contentRect.height }); observer.observe(scrollArea.value) }
})
onBeforeUnmount(() => { disposed = true; cancelBatch.value = true; observer?.disconnect(); media?.removeEventListener('change', touchChange); if (proxyStatusTimer) window.clearInterval(proxyStatusTimer) })
</script>

<style scoped>
.heading-count { color: var(--ink-subtle); font-size: var(--typography-p-body-font-size); font-weight: 400; margin-left: 8px; }
.active-banner { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 0 0 16px; border-bottom: 1px solid var(--hairline); }
.active-banner-content { display: flex; align-items: center; gap: 12px; min-width: 0; }
.active-banner strong { font-weight: 500; font-size: var(--typography-p-body-font-size); overflow-wrap: anywhere; }
.active-banner p { font-size: var(--typography-p-caption-font-size); color: var(--ink-subtle); margin-top: 4px; }
.active-banner-content p { overflow-wrap: anywhere; line-height: 1.6; }
.proxy-group-status { min-width: 220px; max-width: 420px; font-size: var(--typography-p-caption-font-size); color: var(--ink-subtle); }
.proxy-group-status p { margin-bottom: 6px; }
.proxy-group-status strong { color: var(--ink); font-weight: 500; }
.proxy-group-status ul { display: flex; flex-wrap: wrap; gap: 4px 12px; margin: 0; padding: 0; list-style: none; }
.proxy-group-status li { display: inline-flex; align-items: center; gap: 5px; }
.proxy-group-status-error { max-width: 280px; color: var(--danger); font-size: var(--typography-p-caption-font-size); }
.page-heading .actions { flex-wrap: wrap; }
.node-panel { border: 1px solid var(--hairline); border-radius: var(--rounded-lg); background: var(--surface-1); overflow: hidden; min-width: 0; }
.filter-bar { padding: 16px; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; border-bottom: 1px solid var(--hairline); }
.search-input { width: 260px; }.filter-select { width: 150px; }.density-select { width: 90px; }
.filter-spacer { flex: 1; }
.batch-bar { display: flex; align-items: center; gap: 8px; min-height: 48px; padding: 8px 16px; }
.batch-progress { font-size: var(--typography-p-caption-font-size); color: var(--ink-subtle); }
.node-scroll { overflow: auto; max-height: 65vh; height: 560px; position: relative; }
.node-table { width: 100%; min-width: 1224px; table-layout: fixed; border-collapse: separate; border-spacing: 0; font-size: var(--typography-p-body-sm-font-size); }
.node-table thead { position: sticky; top: 0; z-index: 2; background: var(--surface-2); }
.node-table th { height: 36px; font-size: var(--typography-p-micro-font-size); color: var(--ink-subtle); font-weight: 500; text-align: left; border-bottom: 1px solid var(--hairline); padding: 0 12px; }
.node-table td { padding: 0 12px; height: var(--row-height); border-bottom: 1px solid var(--hairline); overflow: hidden; white-space: nowrap; text-overflow: ellipsis; color: var(--ink-muted); }
.node-table td:first-child, .node-table th:first-child { text-align: center; padding: 0; }
.node-table .node-row:hover td { background: var(--surface-2); }
.node-table .node-row.is-active td { background: var(--primary-soft); }
.node-table .node-row.is-failed td { color: var(--ink-tertiary); }
.node-table .numeric { text-align: right; }
.node-table .spacer-row td { height: inherit; border: 0; padding: 0; }
.node-table input[type='checkbox'] { accent-color: var(--primary); width: 14px; height: 14px; vertical-align: middle; cursor: pointer; }
.sort-button { display: inline-flex; align-items: center; gap: 8px; background: transparent; color: inherit; padding: 0; border: 0; font: inherit; }
.sort-button span { color: var(--ink-tertiary); }
.node-name { display: flex; gap: 7px; align-items: center; }
.node-name > span:nth-last-child(1):not(.active-label), .node-name > span:nth-child(2):not(.active-label) { overflow: hidden; text-overflow: ellipsis; }
.active-label { font-size: var(--typography-p-micro-font-size); flex-shrink: 0; color: var(--primary-deep); }
.kernel-cell { font-size: var(--typography-p-caption-font-size); }.endpoint { font-size: var(--typography-p-caption-font-size); color: var(--ink-subtle); }
.row-actions { display: flex; align-items: center; gap: 4px; justify-content: flex-end; }
.row-actions :deep(.el-button + .el-button) { margin-left: 0; }
.node-table .row-actions-head { text-align: right; }
.node-table td:last-child, .node-table th:last-child { position: sticky; right: 0; background: var(--surface-1); border-left: 1px solid var(--hairline); }
.node-table th:last-child { background: var(--surface-2); z-index: 3; }
.node-table .is-active td:last-child { background: var(--primary-soft); }
.table-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; border-top: 1px solid var(--hairline); padding: 12px 16px; font-size: var(--typography-p-micro-font-size); color: var(--ink-subtle); }
.import-hint { font-size: var(--typography-p-body-sm-font-size); color: var(--ink-subtle); margin-bottom: 16px; }
@media (max-width: 768px) { .search-input { width: 100%; } .filter-select { width: calc(50% - 4px); } .density-select { display: none; } .active-banner { flex-wrap: wrap; padding: 16px; } .table-footer span:first-child { max-width: 220px; } .node-table td:last-child, .node-table th:last-child { position: static; border-left: 0; } }
</style>
