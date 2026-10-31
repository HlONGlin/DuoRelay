<template>
  <div class="node-list">
    <el-alert
      v-if="nodes.error"
      :title="nodes.error"
      type="error"
      show-icon
      :closable="false"
      class="page-alert"
    />

    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <div class="toolbar-left">
          <el-input
            v-model="nodes.search"
            placeholder="搜索名称 / 服务器 / 协议"
            clearable
            class="search-input"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          <el-select v-model="nodes.protocolFilter" placeholder="全部协议" clearable class="filter-select">
            <el-option
              v-for="protocol in nodes.protocols"
              :key="protocol"
              :label="protocolLabel(protocol)"
              :value="protocol"
            />
          </el-select>
          <el-select
            v-model="nodes.subscriptionFilter"
            placeholder="全部订阅"
            clearable
            class="filter-select"
          >
            <el-option
              v-for="id in nodes.subscriptionIds"
              :key="id"
              :label="subscriptionLabel(id)"
              :value="id"
            />
          </el-select>
        </div>
        <div class="toolbar-right">
          <span class="count-hint">共 {{ nodes.filtered.length }} 个节点</span>
          <el-button :loading="nodes.loading" @click="refresh">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-table
        v-loading="nodes.loading"
        :data="nodes.filtered"
        empty-text="暂无节点，请先添加订阅或手动导入节点"
        row-key="id"
      >
        <el-table-column prop="name" label="名称" min-width="170" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="name-cell">
              <span>{{ row.name }}</span>
              <el-tag v-if="row.status === 'active'" size="small" type="success">使用中</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="协议" width="130">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ protocolLabel(row.protocol) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="内核" width="110">
          <template #default="{ row }">{{ ownerLabel(row.owner) }}</template>
        </el-table-column>
        <el-table-column label="服务器" min-width="170" show-overflow-tooltip>
          <template #default="{ row }">{{ row.server.host }}:{{ row.server.port }}</template>
        </el-table-column>
        <el-table-column label="传输" width="110">
          <template #default="{ row }">{{ row.transport?.type || 'tcp' }}</template>
        </el-table-column>
        <el-table-column label="延迟" width="120">
          <template #default="{ row }">
            <DelayBadge :delay="row.last_delay_ms" />
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="statusType(row.status)">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button
              size="small"
              type="primary"
              link
              :disabled="row.status === 'active'"
              @click="handleActivate(row)"
            >
              激活
            </el-button>
            <el-button size="small" link @click="handleTest(row)">测速</el-button>
            <el-button size="small" type="danger" link @click="handleRemove(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import DelayBadge from '@/components/DelayBadge.vue'
import { useNodesStore } from '@/stores/nodes'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import { ownerLabel, protocolLabel, statusType } from '@/utils/format'
import type { ProxyNode } from '@/types'

const nodes = useNodesStore()
const subscriptions = useSubscriptionsStore()

function subscriptionLabel(id: string): string {
  return subscriptions.items.find((item) => item.id === id)?.name ?? id
}

function statusText(status?: string): string {
  switch (status) {
    case 'active':
      return '使用中'
    case 'error':
      return '异常'
    case 'testing':
      return '测速中'
    default:
      return '正常'
  }
}

async function refresh() {
  await nodes.fetchNodes()
}

async function handleActivate(node: ProxyNode) {
  try {
    await nodes.activate(node.id)
    ElMessage.success(`已切换到 ${node.name}`)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '激活节点失败')
  }
}

async function handleTest(node: ProxyNode) {
  try {
    const delay = await nodes.testNode(node.id)
    ElMessage.success(`${node.name} 延迟 ${delay} ms`)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '测速失败')
  }
}

async function handleRemove(node: ProxyNode) {
  try {
    await ElMessageBox.confirm(`确定删除节点「${node.name}」吗？`, '删除节点', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
    await nodes.remove(node.id)
    ElMessage.success('节点已删除')
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err instanceof Error ? err.message : '删除节点失败')
    }
  }
}

onMounted(() => {
  void nodes.fetchNodes()
  if (subscriptions.items.length === 0) {
    void subscriptions.fetchSubscriptions()
  }
})
</script>

<style scoped>
.node-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-alert {
  margin: 0;
}

.toolbar-card {
  border-radius: 10px;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.search-input {
  width: 260px;
}

.filter-select {
  width: 160px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.count-hint {
  font-size: 13px;
  color: var(--text-secondary);
}

.name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 768px) {
  .search-input,
  .filter-select {
    width: 100%;
  }

  .toolbar-left {
    width: 100%;
  }
}
</style>
