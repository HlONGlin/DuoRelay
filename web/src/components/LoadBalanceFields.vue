<template>
  <div class="balance-fields">
    <el-form-item label="分配策略">
      <el-select v-model="model.strategy" aria-label="负载均衡策略">
        <el-option label="轮询：依次分配新连接" value="round-robin" />
        <el-option label="一致性哈希：同一目标尽量使用相同节点" value="consistent-hashing" />
        <el-option label="会话保持：同一来源与目标尽量保持节点" value="sticky-sessions" />
        <el-option label="自动选优：定时测速并优先使用延迟较低的节点" value="url-test" />
        <el-option label="故障切换：按列表顺序使用首个健康节点" value="fallback" />
      </el-select>
    </el-form-item>
    <el-form-item label="健康检查地址"><el-input v-model="model.url" placeholder="https://www.gstatic.com/generate_204" aria-label="健康检查地址" /></el-form-item>
    <div class="balance-timing">
      <el-form-item label="检查间隔（秒）"><el-input-number v-model="model.interval" :min="10" :max="86400" aria-label="健康检查间隔" /></el-form-item>
      <el-form-item label="超时（毫秒）"><el-input-number v-model="model.timeout" :min="100" :max="60000" :step="100" aria-label="健康检查超时" /></el-form-item>
    </div>
    <el-form-item label="仅使用时检查"><el-switch v-model="model.lazy" /><div class="field-hint">关闭后，即使没有代理流量也会定期检查节点。</div></el-form-item>
  </div>
</template>

<script setup lang="ts">
import type { LoadBalanceOptions } from '@/types'
const model = defineModel<LoadBalanceOptions>({ required: true })
</script>

<style scoped>
.balance-fields :deep(.el-select) { width: 100%; }
.balance-timing { display: flex; flex-wrap: wrap; gap: 12px 24px; }
.field-hint { width: 100%; color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); margin-top: 6px; }
</style>
