<template>
  <div class="kernel-status">
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
              {{ store.displayStatus }}
              <el-tag v-if="store.status.active_kernel" size="small" effect="plain">
                {{ store.status.active_kernel }}
              </el-tag>
            </div>
            <div class="status-sub">
              <span>版本 {{ store.status.version || '—' }}</span>
              <span>PID {{ store.status.pid || '—' }}</span>
              <span>运行 {{ formatUptime(store.status.uptime_seconds) }}</span>
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
            启动内核
          </el-button>
          <el-button
            v-else
            type="danger"
            plain
            :loading="store.busy"
            @click="handleStop"
          >
            停止内核
          </el-button>
          <el-button :loading="store.loading" @click="store.fetchStatus()">刷新</el-button>
        </div>
      </div>

      <div class="traffic-row">
        <div class="traffic-item">
          <span class="traffic-label">上行流量</span>
          <span class="traffic-value">{{ formatBytes(store.connections.upload_total) }}</span>
        </div>
        <div class="traffic-item">
          <span class="traffic-label">下行流量</span>
          <span class="traffic-value">{{ formatBytes(store.connections.download_total) }}</span>
        </div>
        <div class="traffic-item">
          <span class="traffic-label">活动连接</span>
          <span class="traffic-value">{{ store.connections.connections.length }}</span>
        </div>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="内核日志" name="logs">
          <div class="tab-toolbar">
            <el-switch v-model="autoRefresh" active-text="自动刷新" />
            <el-button size="small" :loading="logsLoading" @click="refreshLogs">刷新日志</el-button>
          </div>
          <el-scrollbar height="420px" class="log-panel">
            <div v-if="store.logs.length === 0" class="log-empty">暂无日志</div>
            <div v-for="(log, index) in store.logs" :key="`${log.time}-${index}`" class="log-line">
              <span class="log-time">{{ formatDate(log.time) }}</span>
              <span class="log-level" :class="`level-${log.level}`">{{ log.level }}</span>
              <span class="log-message">{{ log.message }}</span>
            </div>
          </el-scrollbar>
        </el-tab-pane>

        <el-tab-pane label="连接列表" name="connections">
          <div class="tab-toolbar">
            <span class="connection-hint">共 {{ store.connections.connections.length }} 条连接</span>
            <el-button size="small" :loading="connectionsLoading" @click="refreshConnections">
              刷新连接
            </el-button>
          </div>
          <el-table :data="store.connections.connections" empty-text="暂无活动连接" height="420">
            <el-table-column label="主机" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.metadata.host || row.metadata.destinationIP || '—' }}
              </template>
            </el-table-column>
            <el-table-column label="网络" width="90">
              <template #default="{ row }">{{ row.metadata.network || '—' }}</template>
            </el-table-column>
            <el-table-column label="链路" min-width="160" show-overflow-tooltip>
              <template #default="{ row }">{{ (row.chains || []).join(' → ') || '—' }}</template>
            </el-table-column>
            <el-table-column label="上行" width="110">
              <template #default="{ row }">{{ formatBytes(row.upload) }}</template>
            </el-table-column>
            <el-table-column label="下行" width="110">
              <template #default="{ row }">{{ formatBytes(row.download) }}</template>
            </el-table-column>
            <el-table-column label="开始时间" width="180">
              <template #default="{ row }">{{ formatDate(row.start) }}</template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useKernelStore } from '@/stores/kernel'
import { formatBytes, formatDate, formatUptime } from '@/utils/format'

const store = useKernelStore()

const activeTab = ref('logs')
const autoRefresh = ref(true)
const logsLoading = ref(false)
const connectionsLoading = ref(false)

let pollTimer: number | undefined

async function refreshLogs() {
  logsLoading.value = true
  try {
    await store.fetchLogs()
  } finally {
    logsLoading.value = false
  }
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
  }
}

async function handleStart() {
  try {
    await store.start()
    ElMessage.success('内核已启动')
    await refreshAll()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '启动内核失败')
  }
}

async function handleStop() {
  try {
    await ElMessageBox.confirm('确定停止当前内核吗？停止后代理将不可用。', '停止内核', {
      type: 'warning',
      confirmButtonText: '停止',
      cancelButtonText: '取消'
    })
    await store.stop()
    ElMessage.success('内核已停止')
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err instanceof Error ? err.message : '停止内核失败')
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
  }
})

onMounted(async () => {
  await refreshAll()
  startPolling()
})

onBeforeUnmount(stopPolling)
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
  border-radius: 10px;
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
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #c0c4cc;
}

.status-indicator.running {
  background: #67c23a;
  box-shadow: 0 0 0 4px rgba(103, 194, 58, 0.2);
}

.status-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.status-sub {
  display: flex;
  gap: 16px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--text-secondary);
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
  border-top: 1px solid var(--border-color);
  flex-wrap: wrap;
}

.traffic-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.traffic-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.traffic-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.tab-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.connection-hint {
  font-size: 13px;
  color: var(--text-secondary);
}

.log-panel {
  background: #0f172a;
  border-radius: 8px;
  padding: 10px 12px;
}

.log-empty {
  color: #94a3b8;
  font-size: 13px;
  padding: 8px;
}

.log-line {
  display: flex;
  gap: 12px;
  padding: 3px 4px;
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12px;
  color: #e2e8f0;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-time {
  color: #94a3b8;
  flex-shrink: 0;
}

.log-level {
  flex-shrink: 0;
  text-transform: uppercase;
  color: #38bdf8;
}

.level-error {
  color: #f87171;
}

.level-warn {
  color: #fbbf24;
}

.level-debug {
  color: #a78bfa;
}
</style>
