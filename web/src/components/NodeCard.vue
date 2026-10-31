<template>
  <div class="node-card" :class="{ active: node.status === 'active' }">
    <div class="node-head">
      <div class="node-title">
        <span class="node-name">{{ node.name }}</span>
        <el-tag v-if="node.status === 'active'" size="small" type="success">使用中</el-tag>
      </div>
      <el-tag size="small" effect="plain">{{ protocolLabel(node.protocol) }}</el-tag>
    </div>
    <div class="node-meta">
      <span>{{ node.server.host }}:{{ node.server.port }}</span>
      <span>{{ ownerLabel(node.owner) }}</span>
      <span v-if="node.transport?.type && node.transport.type !== 'tcp'">
        {{ node.transport.type }}
      </span>
    </div>
    <div class="node-foot">
      <DelayBadge :delay="node.last_delay_ms" />
      <div v-if="showActions" class="node-actions">
        <el-button
          size="small"
          type="primary"
          plain
          :disabled="node.status === 'active'"
          @click="$emit('activate', node)"
        >
          使用
        </el-button>
        <el-button size="small" @click="$emit('test', node)">测速</el-button>
        <el-button size="small" type="danger" plain @click="$emit('remove', node)">
          删除
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import DelayBadge from './DelayBadge.vue'
import { ownerLabel, protocolLabel } from '@/utils/format'
import type { ProxyNode } from '@/types'

withDefaults(
  defineProps<{
    node: ProxyNode
    showActions?: boolean
  }>(),
  { showActions: false }
)

defineEmits<{
  (event: 'activate', node: ProxyNode): void
  (event: 'test', node: ProxyNode): void
  (event: 'remove', node: ProxyNode): void
}>()
</script>

<style scoped>
.node-card {
  border: 1px solid var(--border-color);
  border-radius: 10px;
  background: #fff;
  padding: 14px 16px;
  transition: box-shadow 0.15s ease, border-color 0.15s ease;
}

.node-card:hover {
  box-shadow: 0 6px 18px rgba(15, 23, 42, 0.08);
}

.node-card.active {
  border-color: var(--primary-color);
}

.node-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.node-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.node-name {
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 8px;
  color: var(--text-secondary);
  font-size: 13px;
}

.node-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 12px;
  gap: 8px;
}

.node-actions {
  display: flex;
  gap: 4px;
}
</style>
