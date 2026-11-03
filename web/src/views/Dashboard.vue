<template>
  <div class="page-stack dashboard">
    <div class="page-heading"><h1>{{ t('连接概览', 'Connection overview') }}</h1><el-button @click="router.push('/nodes')">{{ t('管理节点', 'Manage nodes') }}</el-button></div>
    <el-alert v-if="errorMessage" :title="errorMessage" type="warning" show-icon :closable="false" />
    <div class="metrics-grid">
      <div class="metric"><span class="metric-label">{{ t('节点总数', 'Total nodes') }}</span><strong class="metric-number">{{ nodes.items.length }}</strong><span class="metric-detail">{{ appearance.language === 'en' ? `${nodes.protocols.length} protocols · ${subscriptions.items.length} subscriptions` : `${nodes.protocols.length} 种协议 · ${subscriptions.items.length} 个订阅` }}</span></div>
      <div class="metric"><span class="metric-label">{{ t('内核状态', 'Kernel status') }}</span><strong class="kernel-value"><span class="status-dot" :class="{ running: kernel.status.running }" />{{ appearance.language === 'en' ? (kernel.status.running ? 'Running' : kernel.status.active_kernel ? 'Stopped' : 'Not configured') : kernel.displayStatus }}</strong><span class="metric-detail mono">{{ kernel.status.active_kernel || t('尚未选择', 'Not selected') }}<template v-if="kernel.status.version"> · {{ kernel.status.version }}</template></span></div>
      <div class="metric"><span class="metric-label">{{ t('本次内核运行上行', 'Upload this session') }}</span><strong class="metric-number">{{ totalsAvailable ? formatBytes(kernel.connections.upload_total) : '—' }}</strong><span class="metric-detail">{{ t('内核累计，重启后重新计数', 'Kernel total, reset on restart') }}</span></div>
      <div class="metric"><span class="metric-label">{{ t('本次内核运行下行', 'Download this session') }}</span><strong class="metric-number">{{ totalsAvailable ? formatBytes(kernel.connections.download_total) : '—' }}</strong><span class="metric-detail">{{ t('内核累计，重启后重新计数', 'Kernel total, reset on restart') }}</span></div>
    </div>
    <div class="overview-grid">
      <section class="panel current-panel">
        <div class="panel-heading"><h2>{{ t('当前代理节点', 'Current proxy node') }}</h2></div>
        <template v-if="nodes.activeNode">
          <div class="current-name"><div><h3>{{ nodes.activeNode.name }}</h3><span class="muted type-p-caption">{{ protocolLabel(nodes.activeNode.protocol) }} · {{ ownerLabel(nodes.activeNode.owner) }}</span></div></div>
          <div class="current-meta"><div><span>{{ t('连接延迟', 'Latency') }}</span><DelayBadge :delay="nodes.activeNode.last_delay_ms" /></div><div><span>{{ t('运行时长', 'Uptime') }}</span><strong class="mono">{{ formatUptime(kernel.status.uptime_seconds) }}</strong></div></div>
          <router-link to="/nodes" class="current-link">{{ t('切换或停止节点', 'Switch or stop node') }}</router-link>
        </template>
        <EmptyState v-else :title="t('未选择节点', 'No node selected')"><el-button @click="router.push('/nodes')">{{ t('选择节点', 'Select node') }}</el-button></EmptyState>
      </section>
      <section class="panel chart-panel">
        <div class="panel-heading"><div><h2>{{ t('实时流量', 'Live traffic') }}</h2><p class="chart-subtitle">{{ appearance.language === 'en' ? `Last ${traffic.history.length} samples · updates every 2 seconds` : `最近 ${traffic.history.length} 次采样 · 每 2 秒更新` }}</p></div><div class="chart-legend"><span class="legend-up">{{ t('上行', 'Upload') }}</span><span class="legend-down">{{ t('下行', 'Download') }}</span></div></div>
        <div class="rates"><div><span>{{ t('上行', 'Upload') }}</span><strong>{{ traffic.error || !traffic.snapshot.sampled_at ? '—' : formatBytes(traffic.snapshot.up) + '/s' }}</strong></div><div><span>{{ t('下行', 'Download') }}</span><strong>{{ traffic.error || !traffic.snapshot.sampled_at ? '—' : formatBytes(traffic.snapshot.down) + '/s' }}</strong></div></div>
        <TrafficChart :samples="traffic.history" :unavailable="Boolean(traffic.error)" />
      </section>
    </div>
    <section class="panel">
      <div class="panel-heading"><h2>{{ t('最近更新的节点', 'Recently updated nodes') }}</h2><el-button text @click="router.push('/nodes')">{{ t('查看全部', 'View all') }}</el-button></div>
      <div v-if="recentNodes.length" class="table-scroll"><el-table :data="recentNodes" v-loading="nodes.loading" style="min-width: 660px">
        <el-table-column prop="name" :label="t('节点名称', 'Node name')" min-width="190" show-overflow-tooltip />
        <el-table-column :label="t('协议', 'Protocol')" width="130"><template #default="{ row }"><span class="neutral-badge">{{ protocolLabel(row.protocol) }}</span></template></el-table-column>
        <el-table-column :label="t('服务器', 'Server')" min-width="180" show-overflow-tooltip><template #default="{ row }"><span class="mono endpoint">{{ nodeEndpointLabel(row) }}</span></template></el-table-column>
        <el-table-column :label="t('延迟', 'Latency')" width="150" align="right"><template #default="{ row }"><DelayBadge :delay="row.last_delay_ms" /></template></el-table-column>
      </el-table></div>
      <EmptyState v-else :title="t('暂无节点', 'No nodes yet')" :description="t('可导入节点链接或添加订阅。', 'Import node links or add a subscription.')"><el-button @click="router.push('/subscriptions')">{{ t('添加订阅', 'Add subscription') }}</el-button></EmptyState>
    </section>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useNodesStore } from '@/stores/nodes'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import { useKernelStore } from '@/stores/kernel'
import { useTrafficStore } from '@/stores/traffic'
import { useAppearanceStore } from '@/stores/appearance'
import { getConnections } from '@/api/kernel'
import TrafficChart from '@/components/TrafficChart.vue'
import EmptyState from '@/components/EmptyState.vue'
import DelayBadge from '@/components/DelayBadge.vue'
import { formatBytes, formatUptime, nodeEndpointLabel, ownerLabel, protocolLabel } from '@/utils/format'
const router = useRouter(), nodes = useNodesStore(), subscriptions = useSubscriptionsStore(), kernel = useKernelStore(), traffic = useTrafficStore()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const totalsAvailable = ref(false)
const recentNodes = computed(() => [...nodes.items].sort((a,b) => (b.updated_at ?? '').localeCompare(a.updated_at ?? '')).slice(0,8))
const errorMessage = computed(() => nodes.error ?? subscriptions.error ?? kernel.error ?? '')
let timer: ReturnType<typeof setInterval> | undefined
let disposed = false
async function fetchTotals() {
  if (!kernel.status.running) { totalsAvailable.value = false; return }
  try { const data = await getConnections(); if (!disposed) { kernel.connections = data; totalsAvailable.value = true } }
  catch { totalsAvailable.value = false }
}
onMounted(async () => {
  void nodes.fetchAllNodes(); void subscriptions.fetchSubscriptions()
  traffic.startPolling()
  await kernel.fetchStatus()
  if (disposed) return
  await fetchTotals()
  if (disposed) return
  timer = setInterval(() => void fetchTotals(), 5000)
})
onUnmounted(() => { disposed = true; traffic.stopPolling(); if (timer) clearInterval(timer) })
</script>
<style scoped>
.metrics-grid { display: grid; grid-template-columns: repeat(4, minmax(0,1fr)); border: 1px solid var(--hairline); border-radius: var(--rounded-lg); background: var(--surface-1); }
.metric { padding: 24px; display: flex; flex-direction: column; gap: 12px; min-width: 0; }
.metric + .metric { border-left: 1px solid var(--hairline); }
.metric-label, .metric-detail { color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); }
.metric-number { font: 500 var(--data-display-font-size)/1.25 var(--font-mono); color: var(--ink); letter-spacing: -.02em; }
.kernel-value { display: flex; align-items: center; gap: 8px; font-size: var(--data-display-font-size); font-weight: 500; line-height: 1.4; }
.overview-grid { display: grid; grid-template-columns: minmax(270px, .8fr) minmax(0, 1.7fr); gap: 24px; }
.current-panel { display: flex; flex-direction: column; }
.current-name { display: flex; align-items: center; gap: 12px; margin: 20px 0 28px; min-width: 0; }
.current-name h3 { font-size: var(--typography-p-headline-font-size); font-weight: 500; overflow-wrap: anywhere; margin-bottom: 6px; }
.current-meta { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.current-meta > div { display: flex; flex-direction: column; align-items: flex-start; gap: 12px; }
.current-meta > div > span:first-child { font-size: var(--typography-p-caption-font-size); color: var(--ink-subtle); }
.current-meta strong { font-size: var(--typography-p-body-sm-font-size); font-weight: 500; }
.current-link { display: block; font-size: var(--typography-p-caption-font-size); padding-top: 24px; margin-top: auto; }
.chart-subtitle { font-size: var(--typography-p-micro-font-size); color: var(--ink-subtle); margin-top: 6px; }
.chart-legend { display: flex; gap: 16px; font-size: var(--typography-p-micro-font-size); color: var(--ink-subtle); }
.chart-legend span { display: flex; align-items: center; gap: 5px; }
.chart-legend span::before { content: ''; width: 14px; height: 0; border-top: 2px solid var(--chart-line); }
.chart-legend .legend-down::before { border-top: 2px dashed var(--chart-line-2); }
.rates { display: flex; gap: 40px; margin-bottom: 12px; }
.rates > div { display: flex; align-items: baseline; gap: 10px; }
.rates span { font-size: var(--typography-p-micro-font-size); color: var(--ink-subtle); }
.rates strong { font: 500 var(--typography-p-headline-font-size)/1.4 var(--font-mono); }
.endpoint { color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); }
@media (max-width: 1150px) { .metric { padding: 20px 16px; } .metric-number { font-size: var(--data-display-font-size); } .overview-grid { grid-template-columns: minmax(240px, 1fr) minmax(0, 1.5fr); gap: 16px; } }
@media (max-width: 900px) { .metrics-grid { grid-template-columns: repeat(2, minmax(0,1fr)); } .metric:nth-child(3) { border-left: 0; }.metric:nth-child(n+3) { border-top: 1px solid var(--hairline); } .overview-grid { grid-template-columns: 1fr; } .current-link { margin-top: 16px; } .current-name { margin: 0 0 20px; } }
@media (max-width: 480px) { .metric-number { font-size: var(--data-display-font-size); } .kernel-value { font-size: var(--typography-p-headline-font-size); } .metric { gap: 8px; padding: 16px; } .rates { gap: 24px; } .rates > div { flex-direction: column; gap: 4px; } }
</style>
