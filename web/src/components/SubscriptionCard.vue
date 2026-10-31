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

    <div class="sub-url" :title="subscription.url">{{ maskUrl(subscription.url) }}</div>

    <div class="sub-usage">
      <div class="usage-row">
        <span>流量使用</span>
        <span>{{ formatBytes(subscription.used) }} / {{ formatBytes(subscription.total) }}</span>
      </div>
      <el-progress
        :percentage="usagePercent(subscription.used, subscription.total)"
        :show-text="false"
        :stroke-width="8"
        :color="progressColor"
      />
    </div>

    <div class="sub-meta">
      <span>更新: {{ formatDate(subscription.last_update) }}</span>
      <span>到期: {{ formatDate(subscription.expire) }}</span>
    </div>

    <div class="sub-actions">
      <el-button size="small" type="primary" plain :loading="updating" @click="$emit('refresh', subscription)">
        更新
      </el-button>
      <el-button size="small" @click="$emit('edit', subscription)">编辑</el-button>
      <el-button size="small" type="danger" plain @click="$emit('remove', subscription)">
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
  if (percent >= 90) return '#f56c6c'
  if (percent >= 70) return '#e6a23c'
  return '#409eff'
})

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
  border: 1px solid var(--border-color);
  border-radius: 10px;
  background: #fff;
  padding: 16px;
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
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sub-interval {
  color: var(--text-secondary);
  font-size: 12px;
  white-space: nowrap;
}

.sub-url {
  font-size: 12px;
  color: var(--text-secondary);
  font-family: Consolas, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.usage-row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  color: var(--text-regular);
  margin-bottom: 6px;
}

.sub-meta {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  font-size: 12px;
  color: var(--text-secondary);
}

.sub-actions {
  display: flex;
  gap: 6px;
}
</style>
