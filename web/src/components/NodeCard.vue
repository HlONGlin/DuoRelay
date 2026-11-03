<template>
  <div class="node-card" :class="{ active: node.status === 'active' }">
    <div class="node-head">
      <div class="node-title">
        <span class="node-name">{{ node.name }}</span>
        <el-tag v-if="node.status === 'active'" size="small" type="success">使用中</el-tag>
      </div>
      <el-tooltip :content="protocolDescription(node.protocol)" placement="top">
        <div class="protocol-chip">
          <span class="protocol-glyph">{{ protocolGlyph(node.protocol) }}</span>
          <el-tag size="small" effect="plain">{{ protocolLabel(node.protocol) }}</el-tag>
        </div>
      </el-tooltip>
    </div>
    <div class="node-meta">
      <span>{{ nodeEndpointLabel(node) }}</span>
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
          :disabled="node.status === 'active' || Boolean(nodeUnavailableReason(node))"
          :title="nodeUnavailableReason(node)"
          @click="$emit('activate', node)"
        >
          {{ nodeUnavailableReason(node) || '使用' }}
        </el-button>
        <el-button size="small" @click="$emit('test', node)">测速</el-button>
        <el-button size="small" plain @click="$emit('remove', node)">
          删除
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import DelayBadge from './DelayBadge.vue'
import { nodeUnavailableReason } from '@/stores/variant'
import {
  nodeEndpointLabel,
  ownerLabel,
  protocolDescription,
  protocolGlyph,
  protocolLabel
} from '@/utils/format'
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
  border: 1px solid var(--hairline);
  border-radius: var(--rounded-lg);
  background: var(--surface-1);
  padding: 16px;
}

.node-card:hover {
  background: var(--surface-2);
}

.node-card.active {
  border-color: var(--primary);
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

.protocol-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.protocol-glyph {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  height: 20px;
  padding: 0 4px;
  border-radius: var(--rounded-xs);
  background: var(--surface-3);
  color: var(--ink-muted);
  font-family: var(--font-mono);
  font-size: var(--typography-p-micro-font-size);
  font-weight: 700;
  letter-spacing: 0.02em;
}

.node-name {
  color: var(--ink);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 8px;
  color: var(--ink-subtle);
  font-size: var(--typography-p-body-sm-font-size);
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
