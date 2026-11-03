<template>
  <div class="kernel-status page-stack">
    <div class="page-heading"><h1>{{ t('内核状态', 'Kernel status') }}</h1></div>
    <el-alert
      v-if="store.error"
      :title="store.error"
      type="error"
      show-icon
      :closable="false"
      class="page-alert"
    />

    <el-card v-loading="store.loading" shadow="never" class="status-card">
      <div class="status-head">
        <div class="status-main">
          <div class="status-indicator" :class="{ running: store.status.running }" />
          <div>
            <div class="status-title">
              {{ appearance.language === 'en' ? (store.status.running ? 'Running' : store.status.active_kernel ? 'Stopped' : 'Not configured') : store.displayStatus }}
              <el-tag v-if="store.status.active_kernel" size="small" effect="plain">
                {{ store.status.active_kernel }}
              </el-tag>
            </div>
            <div class="status-sub">
              <span>{{ t('版本', 'Version') }} {{ store.status.version || '—' }}</span>
              <span>PID {{ store.status.pid || '—' }}</span>
              <span>{{ t('运行', 'Uptime') }} {{ formatUptime(store.status.uptime_seconds) }}</span>
            </div>
          </div>
        </div>
        <div class="status-actions">
          <el-button
            v-if="!store.status.running"
            type="primary"
            :loading="store.busy"
            @click="handleStart"
          >
            {{ t('启动内核', 'Start kernel') }}
          </el-button>
          <el-button
            v-else
            plain
            :loading="store.busy"
            @click="handleStop"
          >
            {{ t('停止内核', 'Stop kernel') }}
          </el-button>
          <el-button :loading="store.loading" @click="store.fetchStatus()">{{ t('刷新', 'Refresh') }}</el-button>
        </div>
      </div>

      <div class="traffic-row">
        <div class="traffic-item">
          <span class="traffic-label">{{ t('本次运行上行', 'Upload this session') }}</span>
          <span class="traffic-value">{{ formatBytes(store.connections.upload_total) }}</span>
        </div>
        <div class="traffic-item">
          <span class="traffic-label">{{ t('本次运行下行', 'Download this session') }}</span>
          <span class="traffic-value">{{ formatBytes(store.connections.download_total) }}</span>
        </div>
        <div class="traffic-item">
          <span class="traffic-label">{{ t('活动连接', 'Active connections') }}</span>
          <span class="traffic-value">{{ store.connections.connections.length }}</span>
        </div>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-tabs v-model="activeTab">
        <el-tab-pane :label="t('内核日志', 'Kernel logs')" name="logs">
          <div class="tab-toolbar">
            <el-switch v-model="autoRefresh" :active-text="t('自动刷新', 'Auto refresh')" />
            <div class="log-filters">
              <el-select v-model="logLevel" size="small" class="log-filter-select" @change="refreshLogs">
                <el-option :label="t('全部级别', 'All levels')" value="all" />
                <el-option :label="t('信息', 'Info')" value="info" />
                <el-option :label="t('警告', 'Warning')" value="warn" />
                <el-option :label="t('错误', 'Error')" value="error" />
                <el-option :label="t('调试', 'Debug')" value="debug" />
              </el-select>
              <el-select v-model="logSource" size="small" class="log-source-select" @change="refreshLogs">
                <el-option :label="t('全部来源', 'All sources')" value="all" />
                <el-option :label="t('标准输出', 'Standard output')" value="stdout" />
                <el-option :label="t('错误输出', 'Standard error')" value="stderr" />
              </el-select>
              <el-input
                v-model="logSearch"
                size="small"
                clearable
                class="log-search"
                :placeholder="t('搜索日志', 'Search logs')"
                @keyup.enter="refreshLogs"
                @clear="refreshLogs"
              />
              <el-button size="small" :loading="logsLoading" @click="refreshLogs">{{ t('刷新日志', 'Refresh logs') }}</el-button>
            </div>
          </div>
          <div class="log-summary">
            <span>{{ t('总计', 'Total') }} {{ store.logSummary.total }}</span>
            <span class="summary-info">{{ t('信息', 'Info') }} {{ store.logSummary.levels.info }}</span>
            <span class="summary-warn">{{ t('警告', 'Warning') }} {{ store.logSummary.levels.warn }}</span>
            <span class="summary-error">{{ t('错误', 'Error') }} {{ store.logSummary.levels.error }}</span>
            <span class="summary-debug">{{ t('调试', 'Debug') }} {{ store.logSummary.levels.debug }}</span>
          </div>
          <div v-if="store.logSummary.clusters.length" class="log-clusters">
            <span class="cluster-title">{{ t('重复错误', 'Repeated errors') }}</span>
            <el-tag
              v-for="cluster in store.logSummary.clusters.slice(0, 3)"
              :key="cluster.message + '-' + cluster.level"
              size="small"
              effect="plain"
              :type="cluster.level === 'error' ? 'danger' : 'warning'"
              :title="cluster.message"
            >
              {{ cluster.count }} {{ t('次', 'times') }} {{ cluster.message }}
            </el-tag>
          </div>
          <el-scrollbar height="420px" class="log-panel">
            <div v-if="store.logs.length === 0" class="log-empty">{{ t('暂无日志', 'No logs') }}</div>
            <div v-for="(log, index) in store.logs" :key="`${log.time}-${index}`" class="log-line">
              <span class="log-time">{{ formatDate(log.time) }}</span>
              <span class="log-level" :class="`level-${log.level}`">{{ log.level }}</span>
              <span class="log-message">{{ log.message }}</span>
            </div>
          </el-scrollbar>
        </el-tab-pane>

        <el-tab-pane :label="t('连接列表', 'Connections')" name="connections">
          <div class="tab-toolbar">
            <span class="connection-hint">{{ appearance.language === 'en' ? `${store.connections.connections.length} connections` : `共 ${store.connections.connections.length} 条连接` }}</span>
            <el-button size="small" :loading="connectionsLoading" @click="refreshConnections">
              {{ t('刷新连接', 'Refresh connections') }}
            </el-button>
          </div>
          <div class="table-scroll"><el-table style="min-width: 950px" :data="store.connections.connections" :empty-text="t('暂无活动连接', 'No active connections')" height="420">
            <el-table-column :label="t('主机', 'Host')" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.metadata.host || row.metadata.destinationIP || '—' }}
              </template>
            </el-table-column>
            <el-table-column :label="t('网络', 'Network')" width="90">
              <template #default="{ row }">{{ row.metadata.network || '—' }}</template>
            </el-table-column>
            <el-table-column :label="t('链路', 'Chain')" min-width="160" show-overflow-tooltip>
              <template #default="{ row }">{{ (row.chains || []).join(' → ') || '—' }}</template>
            </el-table-column>
            <el-table-column :label="t('上行', 'Upload')" width="110" align="right" class-name="mono">
              <template #default="{ row }">{{ formatBytes(row.upload) }}</template>
            </el-table-column>
            <el-table-column :label="t('下行', 'Download')" width="110" align="right" class-name="mono">
              <template #default="{ row }">{{ formatBytes(row.download) }}</template>
            </el-table-column>
            <el-table-column :label="t('匹配规则', 'Matched rule')" min-width="150" show-overflow-tooltip><template #default="{ row }">{{ row.rule || '—' }} {{ row.rulePayload || '' }}</template></el-table-column>
            <el-table-column :label="t('开始时间', 'Start time')" width="180">
              <template #default="{ row }">{{ formatDate(row.start) }}</template>
            </el-table-column>
          </el-table></div>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { openKernelLogStream } from '@/api/kernel'
import { useKernelStore } from '@/stores/kernel'
import { useNodesStore } from '@/stores/nodes'
import { useAppearanceStore } from '@/stores/appearance'
import { formatBytes, formatDate, formatUptime } from '@/utils/format'

const store = useKernelStore()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const nodes = useNodesStore()

const activeTab = ref('logs')
const autoRefresh = ref(true)
const logsLoading = ref(false)
const connectionsLoading = ref(false)
const logLevel = ref<'all' | 'info' | 'warn' | 'error' | 'debug'>('all')
const logSource = ref<'all' | 'stdout' | 'stderr'>('all')
const logSearch = ref('')

let pollTimer: number | undefined
let logSocket: WebSocket | undefined

async function refreshLogs() {
  logsLoading.value = true
  try {
    await store.fetchLogs({
      level: logLevel.value,
      source: logSource.value === 'all' ? undefined : logSource.value,
      search: logSearch.value || undefined
    })
  } finally {
    logsLoading.value = false
  }
}

function stopLogStream() {
  if (logSocket) {
    logSocket.close()
    logSocket = undefined
  }
}

function startLogStream() {
  stopLogStream()
  if (!store.status.running || !autoRefresh.value) return
  logSocket = openKernelLogStream({
    kernel: store.status.active_kernel || undefined,
    level: logLevel.value,
    source: logSource.value === 'all' ? undefined : logSource.value,
    search: logSearch.value || undefined
  })
  logSocket.onmessage = (event) => {
    try {
      const message = JSON.parse(event.data) as { type: string; logs?: typeof store.logs; log?: typeof store.logs[number] }
      if (message.type === 'snapshot' && message.logs) store.logs = message.logs
      if (message.type === 'log' && message.log) store.appendLiveLog(message.log)
    } catch {
      // Ignore malformed stream messages; the HTTP refresh remains available.
    }
  }
  logSocket.onerror = () => stopLogStream()
}

async function refreshConnections() {
  connectionsLoading.value = true
  try {
    await store.fetchConnections()
  } finally {
    connectionsLoading.value = false
  }
}

async function refreshAll() {
  await store.fetchStatus()
  if (store.status.running) {
    await Promise.all([refreshLogs(), refreshConnections()])
    startLogStream()
  }
}

async function handleStart() {
  try {
    await store.start()
    await nodes.fetchAllNodes()
    ElMessage.success(t('内核已启动', 'Kernel started'))
    await refreshAll()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : t('启动内核失败', 'Failed to start kernel'))
  }
}

async function handleStop() {
  try {
    await ElMessageBox.confirm(t('确定停止当前内核吗？停止后代理将不可用。', 'Stop the current kernel? The proxy will become unavailable.'), t('停止内核', 'Stop kernel'), {
      type: 'warning',
      confirmButtonText: t('停止', 'Stop'),
      cancelButtonText: t('取消', 'Cancel')
    })
    await store.stop()
    await nodes.fetchAllNodes()
    ElMessage.success(t('内核已停止，当前节点选择已清除', 'Kernel stopped and current node selection cleared'))
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err instanceof Error ? err.message : t('停止内核失败', 'Failed to stop kernel'))
    }
  }
}

function stopPolling() {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
}

function startPolling() {
  stopPolling()
  pollTimer = window.setInterval(async () => {
    if (!autoRefresh.value) return
    await store.fetchStatus()
    if (store.status.running) {
      await Promise.all([refreshLogs(), refreshConnections()])
    }
  }, 5000)
}

watch(autoRefresh, (enabled) => {
  if (enabled) {
    void refreshLogs()
    void refreshConnections()
    startLogStream()
  } else {
    stopLogStream()
  }
})

watch([logLevel, logSource, logSearch], () => {
  if (store.status.running) startLogStream()
})

onMounted(async () => {
  await refreshAll()
  startPolling()
})

onBeforeUnmount(() => {
  stopPolling()
  stopLogStream()
})
</script>

<style scoped>
.kernel-status {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-alert {
  margin: 0;
}

.status-card {
  border-radius: var(--rounded-lg);
}

.status-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.status-main {
  display: flex;
  align-items: center;
  gap: 14px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--ink-tertiary);
}

.status-indicator.running {
  background: var(--semantic-success);
}

.status-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--typography-p-headline-font-size);
  font-weight: 500;
  color: var(--ink);
}

.status-sub {
  display: flex;
  gap: 16px;
  margin-top: 6px;
  font-size: var(--typography-p-caption-font-size);
  color: var(--ink-subtle);
}

.status-actions {
  display: flex;
  gap: 8px;
}

.traffic-row {
  display: flex;
  gap: 40px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--hairline);
  flex-wrap: wrap;
}

.traffic-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.traffic-label {
  color: var(--ink-subtle);
  font-size: var(--typography-p-caption-font-size);
}

.traffic-value {
  color: var(--ink);
  font-family: var(--font-mono);
  font-size: var(--typography-p-headline-font-size);
  font-weight: 600;
}

.tab-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.log-filters {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.log-filter-select {
  width: 120px;
}

.log-source-select {
  width: 120px;
}

.log-search {
  width: 220px;
}

.log-summary,
.log-clusters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 10px;
  font-size: var(--typography-p-caption-font-size);
  color: var(--ink-subtle);
}

.summary-info {
  color: var(--ink-subtle);
}

.summary-warn {
  color: var(--semantic-warning);
}

.summary-error {
  color: var(--semantic-danger);
}

.summary-debug {
  color: var(--ink-muted);
}

.cluster-title {
  font-weight: 600;
  color: var(--ink);
}

.connection-hint {
  color: var(--ink-subtle);
  font-size: var(--typography-p-body-sm-font-size);
}

.log-panel {
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: var(--rounded-md);
  padding: 10px 12px;
}

.log-empty {
  color: var(--ink-subtle);
  font-size: var(--typography-p-body-sm-font-size);
  padding: 8px;
}

.log-line {
  display: flex;
  gap: 12px;
  padding: 3px 4px;
  color: var(--ink-muted);
  font-family: var(--font-mono);
  font-size: var(--typography-p-caption-font-size);
  white-space: pre-wrap;
  word-break: break-all;
}

.log-time {
  color: var(--ink-subtle);
  flex-shrink: 0;
}

.log-level {
  flex-shrink: 0;
  text-transform: uppercase;
  color: var(--ink-subtle);
}

.level-error {
  color: var(--semantic-danger);
}

.level-warn {
  color: var(--semantic-warning);
}

.level-debug {
  color: var(--ink-muted);
}

@media (max-width: 760px) {
  .tab-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }

  .log-filters {
    justify-content: flex-start;
    width: 100%;
  }

  .log-search {
    flex: 1;
    min-width: 150px;
  }
}
</style>
