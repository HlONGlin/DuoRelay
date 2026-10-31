<template>
  <div class="dashboard">
    <el-alert
      v-if="errorMessage"
      :title="errorMessage"
      type="warning"
      show-icon
      :closable="false"
      class="page-alert"
    />

    <el-row :gutter="16">
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-label">节点总数</div>
          <div class="stat-value">{{ nodes.items.length }}</div>
          <div class="stat-extra">可用 {{ nodes.filtered.length }} 个</div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-label">订阅数量</div>
          <div class="stat-value">{{ subscriptions.items.length }}</div>
          <div class="stat-extra">正常 {{ subscriptions.updatedCount }} 个</div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-label">内核状态</div>
          <div class="stat-value">{{ kernel.displayStatus }}</div>
          <div class="stat-extra">
            {{ kernel.status.active_kernel || '未选择内核' }}
            <template v-if="kernel.status.version"> · {{ kernel.status.version }}</template>
          </div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-label">订阅流量</div>
          <div class="stat-value">{{ formatBytes(subscriptions.totalUsed) }}</div>
          <div class="stat-extra">配额 {{ formatBytes(subscriptions.totalQuota) }}</div>
        </div>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="dashboard-row">
      <el-col :xs="24" :lg="12">
        <el-card shadow="never" class="panel">
          <template #header>
            <div class="panel-header">
              <span>当前节点</span>
              <el-button text type="primary" @click="router.push('/nodes')">管理节点</el-button>
            </div>
          </template>
          <NodeCard v-if="nodes.activeNode" :node="nodes.activeNode" />
          <el-empty v-else description="尚未激活节点">
            <el-button type="primary" @click="router.push('/nodes')">去选择节点</el-button>
          </el-empty>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="12">
        <el-card shadow="never" class="panel">
          <template #header>
            <div class="panel-header">
              <span>快捷操作</span>
            </div>
          </template>
          <div class="quick-actions">
            <el-button type="primary" plain @click="router.push('/nodes')">节点列表</el-button>
            <el-button plain @click="router.push('/subscriptions')">订阅管理</el-button>
            <el-button plain @click="router.push('/kernel')">内核状态</el-button>
            <el-button plain @click="router.push('/settings')">系统设置</el-button>
          </div>
          <div class="quick-summary">
            <div class="summary-item">
              <span class="summary-label">活跃节点延迟</span>
              <DelayBadge :delay="nodes.activeNode?.last_delay_ms" />
            </div>
            <div class="summary-item">
              <span class="summary-label">运行时长</span>
              <span class="summary-value">{{ formatUptime(kernel.status.uptime_seconds) }}</span>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="panel recent-panel">
      <template #header>
        <div class="panel-header">
          <span>最近节点</span>
          <el-button text type="primary" @click="router.push('/nodes')">查看全部</el-button>
        </div>
      </template>
      <el-table :data="recentNodes" v-loading="nodes.loading" empty-text="暂无节点数据">
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column label="协议" width="130">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ protocolLabel(row.protocol) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="服务器" min-width="180">
          <template #default="{ row }">{{ row.server.host }}:{{ row.server.port }}</template>
        </el-table-column>
        <el-table-column label="延迟" width="110">
          <template #default="{ row }">
            <DelayBadge :delay="row.last_delay_ms" />
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useNodesStore } from '@/stores/nodes'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import { useKernelStore } from '@/stores/kernel'
import NodeCard from '@/components/NodeCard.vue'
import DelayBadge from '@/components/DelayBadge.vue'
import { formatBytes, formatUptime, protocolLabel } from '@/utils/format'

const router = useRouter()
const nodes = useNodesStore()
const subscriptions = useSubscriptionsStore()
const kernel = useKernelStore()

const recentNodes = computed(() =>
  [...nodes.items]
    .sort((a, b) => (b.updated_at ?? '').localeCompare(a.updated_at ?? ''))
    .slice(0, 5)
)

const errorMessage = computed(
  () => nodes.error ?? subscriptions.error ?? kernel.error ?? ''
)

onMounted(() => {
  void nodes.fetchNodes()
  void subscriptions.fetchSubscriptions()
  void kernel.fetchStatus()
})
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-alert {
  margin: 0;
}

.stat-card {
  background: #fff;
  border: 1px solid var(--border-color);
  border-radius: 10px;
  padding: 18px;
  height: 100%;
}

.stat-label {
  color: var(--text-secondary);
  font-size: 13px;
}

.stat-value {
  margin-top: 8px;
  font-size: 26px;
  font-weight: 600;
  color: var(--text-primary);
}

.stat-extra {
  margin-top: 6px;
  font-size: 12px;
  color: var(--text-secondary);
}

.dashboard-row {
  margin: 0;
}

.panel {
  border-radius: 10px;
  height: 100%;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
}

.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.quick-summary {
  display: flex;
  gap: 32px;
  margin-top: 22px;
  padding-top: 16px;
  border-top: 1px dashed var(--border-color);
}

.summary-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.summary-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.summary-value {
  font-size: 14px;
  color: var(--text-primary);
}

.recent-panel {
  margin-bottom: 8px;
}
</style>
