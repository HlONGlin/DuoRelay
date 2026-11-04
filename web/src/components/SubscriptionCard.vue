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
        {{ subscription.interval ? (appearance.language === 'en' ? `Every ${subscription.interval} hours` : `每 ${subscription.interval} 小时`) : t('手动更新', 'Manual updates') }}
      </span>
    </div>

    <div class="sub-url">{{ maskUrl(subscription.url) }}</div>
    <el-alert v-if="subscription.last_error" :title="subscription.last_error" type="error" :closable="false" />
    <details v-if="subscription.skipped?.length"><summary>{{ t('跳过的无效条目：', 'Skipped entries: ') }}{{ subscription.skipped.length }}</summary><ul><li v-for="skip in subscription.skipped" :key="skip.line">{{ skip.line }}: {{ skip.reason }}</li></ul></details>
    <p v-if="subscription.diff?.duplicates">{{ t('已跳过重复节点：', 'Duplicates skipped: ') }}{{ subscription.diff.duplicates }}</p>

    <div class="sub-usage">
      <div class="usage-row">
        <span>{{ t('流量使用', 'Traffic usage') }}</span>
        <span class="mono">{{ subscription.total ? formatBytes(subscription.used) + ' / ' + formatBytes(subscription.total) : t('未提供流量配额', 'No traffic quota provided') }}</span>
      </div>
      <el-progress
        :percentage="usagePercent(subscription.used, subscription.total)"
        :show-text="false"
        :stroke-width="3"
        :color="progressColor"
      />
    </div>

    <div class="sub-meta">
      <span>{{ t('节点', 'Nodes') }} <span class="mono">{{ nodeCount ?? '—' }}</span></span>
      <span>{{ t('更新:', 'Updated:') }} <span class="mono">{{ formatDate(subscription.last_update) }}</span></span>
      <span :class="{ expired }">{{ expired ? t('已到期: ', 'Expired: ') : t('到期: ', 'Expires: ') }}<span class="mono">{{ formatDate(subscription.expire) }}</span></span>
    </div>

    <div class="sub-actions">
      <el-button size="small" text :loading="updating" @click="$emit('refresh', subscription)">
        {{ t('更新', 'Update') }}
      </el-button>
      <el-button size="small" text @click="$emit('edit', subscription)">{{ t('编辑', 'Edit') }}</el-button>
      <el-button size="small" text @click="$emit('remove', subscription)">
        {{ t('删除', 'Delete') }}
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
import { useAppearanceStore } from '@/stores/appearance'
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh

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
    case 'ok':
    case 'success':
      return t('正常', 'OK')
    case 'error':
      return t('失败', 'Failed')
    case 'updating':
      return t('更新中', 'Updating')
    default:
      return t('未更新', 'Not updated')
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
