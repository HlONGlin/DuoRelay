<template>
  <div class="sub-card">
    <div class="sub-head">
      <div class="sub-title">
        <span class="sub-name">{{ subscription.name }}</span>
        <el-tag size="small" :type="statusType(subscription.last_status)">
          {{ statusLabel }}
        </el-tag>
      </div>
      <span class="sub-interval">
        {{ subscription.interval ? `每 ${subscription.interval} 小时` : '手动更新' }}
      </span>
    </div>

    <div class="sub-url">{{ maskUrl(subscription.url) }}</div>

    <div class="sub-usage">
      <div class="usage-row">
        <span>流量使用</span>
        <span class="mono">{{ subscription.total ? formatBytes(subscription.used) + ' / ' + formatBytes(subscription.total) : '未提供流量配额' }}</span>
      </div>
      <el-progress
        :percentage="usagePercent(subscription.used, subscription.total)"
        :show-text="false"
        :stroke-width="3"
        :color="progressColor"
      />
    </div>

    <div class="sub-meta">
      <span>节点 <span class="mono">{{ nodeCount ?? '—' }}</span></span>
      <span>更新: <span class="mono">{{ formatDate(subscription.last_update) }}</span></span>
      <span :class="{ expired }">{{ expired ? '已到期: ' : '到期: ' }}<span class="mono">{{ formatDate(subscription.expire) }}</span></span>
    </div>

    <div class="sub-actions">
      <el-button size="small" text :loading="updating" @click="$emit('refresh', subscription)">
        更新
      </el-button>
      <el-button size="small" text @click="$emit('edit', subscription)">编辑</el-button>
      <el-button size="small" text @click="$emit('remove', subscription)">
        删除
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  formatBytes,
  formatDate,
  statusType,
  usagePercent
} from '@/utils/format'
import type { Subscription } from '@/types'

const props = defineProps<{
  subscription: Subscription
  nodeCount?: number
  updating?: boolean
}>()

defineEmits<{
  (event: 'refresh', subscription: Subscription): void
  (event: 'edit', subscription: Subscription): void
  (event: 'remove', subscription: Subscription): void
}>()

const statusLabel = computed(() => {
  switch (props.subscription.last_status) {
    case 'success':
      return '正常'
    case 'error':
      return '失败'
    case 'updating':
      return '更新中'
    default:
      return '未更新'
  }
})

const progressColor = computed(() => {
  const percent = usagePercent(props.subscription.used, props.subscription.total)
  if (percent > 85) return 'var(--semantic-warning)'
  return 'var(--primary)'
})
const expired = computed(() => Boolean(props.subscription.expire && new Date(props.subscription.expire).getTime() > 0 && new Date(props.subscription.expire).getTime() < Date.now()))

function maskUrl(url?: string): string {
  if (!url) return '—'
  try {
    const parsed = new URL(url)
    const token = parsed.pathname.length > 8 ? `${parsed.pathname.slice(0, 8)}…` : parsed.pathname
    return `${parsed.protocol}//${parsed.host}${token}`
  } catch {
    return url.length > 48 ? `${url.slice(0, 48)}…` : url
  }
}
</script>

<style scoped>
.sub-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  border: 1px solid var(--hairline);
  border-radius: var(--rounded-lg);
  background: var(--surface-1);
  padding: 20px;
}

.sub-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.sub-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.sub-name {
  color: var(--ink);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sub-interval {
  color: var(--ink-subtle);
  font-size: var(--typography-p-caption-font-size);
  white-space: nowrap;
}

.sub-url {
  color: var(--ink-subtle);
  font-family: var(--font-mono);
  font-size: var(--typography-p-caption-font-size);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.usage-row {
  display: flex;
  justify-content: space-between;
  font-size: var(--typography-p-body-sm-font-size);
  color: var(--ink-muted);
  margin-bottom: 6px;
}

.sub-meta {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  font-size: var(--typography-p-caption-font-size);
  color: var(--ink-subtle);
}

.sub-actions {
  display: flex;
  gap: 6px;
}
.expired { color: var(--semantic-danger); }
.sub-meta { flex-wrap: wrap; }
.sub-actions { border-top: 1px solid var(--hairline); padding-top: 12px; margin-top: auto; }
.sub-actions :deep(.el-button + .el-button) { margin-left: 0; }
</style>
