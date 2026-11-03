<template>
  <div class="balance-fields">
    <el-form-item :label="t('分配策略', 'Distribution strategy')">
      <el-select v-model="model.strategy" :aria-label="t('负载均衡策略', 'Load balancing strategy')">
        <el-option :label="t('轮询：依次分配新连接', 'Round robin: distribute new connections in order')" value="round-robin" />
        <el-option :label="t('一致性哈希：同一目标尽量使用相同节点', 'Consistent hashing: prefer the same node for each destination')" value="consistent-hashing" />
        <el-option :label="t('会话保持：同一来源与目标尽量保持节点', 'Sticky sessions: prefer the same node for each source and destination')" value="sticky-sessions" />
        <el-option :label="t('自动选优：定时测速并优先使用延迟较低的节点', 'Automatic selection: periodically test and prefer lower-latency nodes')" value="url-test" />
        <el-option :label="t('故障切换：按列表顺序使用首个健康节点', 'Fallback: use the first healthy node in the list')" value="fallback" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('健康检查地址', 'Health check URL')"><el-input v-model="model.url" placeholder="https://www.gstatic.com/generate_204" :aria-label="t('健康检查地址', 'Health check URL')" /></el-form-item>
    <div class="balance-timing">
      <el-form-item :label="t('检查间隔（秒）', 'Interval (seconds)')"><el-input-number v-model="model.interval" :min="10" :max="86400" :aria-label="t('健康检查间隔', 'Health check interval')" /></el-form-item>
      <el-form-item :label="t('超时（毫秒）', 'Timeout (milliseconds)')"><el-input-number v-model="model.timeout" :min="100" :max="60000" :step="100" :aria-label="t('健康检查超时', 'Health check timeout')" /></el-form-item>
    </div>
    <el-form-item :label="t('仅使用时检查', 'Check only when in use')"><el-switch v-model="model.lazy" /><div class="field-hint">{{ t('关闭后，即使没有代理流量也会定期检查节点。', 'When disabled, nodes are checked periodically even without proxy traffic.') }}</div></el-form-item>
  </div>
</template>

<script setup lang="ts">
import type { LoadBalanceOptions } from '@/types'
import { useAppearanceStore } from '@/stores/appearance'
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const model = defineModel<LoadBalanceOptions>({ required: true })
</script>

<style scoped>
.balance-fields :deep(.el-select) { width: 100%; }
.balance-timing { display: flex; flex-wrap: wrap; gap: 12px 24px; }
.field-hint { width: 100%; color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); margin-top: 6px; }
</style>
