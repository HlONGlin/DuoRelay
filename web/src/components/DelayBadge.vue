<template>
  <span class="delay-badge" :class="`delay-${type}`">
    <span class="delay-dot" />
    {{ formatDelay(delay) }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { delayType, formatDelay } from '@/utils/format'

const props = defineProps<{
  delay?: number
}>()

const type = computed(() => delayType(props.delay))
</script>

<style scoped>
.delay-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: var(--text-regular);
}

.delay-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #909399;
}

.delay-success .delay-dot {
  background: #67c23a;
}

.delay-warning .delay-dot {
  background: #e6a23c;
}

.delay-danger .delay-dot {
  background: #f56c6c;
}
</style>
