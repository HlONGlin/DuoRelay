<template>
  <span class="delay-badge" :class="'delay-' + type">
    <span class="delay-bar" aria-hidden="true"><span :style="{ width: fill + '%' }" /></span>
    <span>{{ formatted }}</span>
  </span>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { delayType, formatDelay } from '@/utils/format'
import { useAppearanceStore } from '@/stores/appearance'
const appearance = useAppearanceStore()
const props = defineProps<{ delay?: number }>()
const type = computed(() => delayType(props.delay))
const formatted = computed(() => props.delay !== undefined && props.delay < 0 ? (appearance.language === 'en' ? 'Timeout' : '超时') : formatDelay(props.delay))
const fill = computed(() => props.delay && props.delay > 0 ? Math.min(props.delay / 300, 1) * 100 : 0)
</script>
<style scoped>
.delay-badge { display: inline-flex; align-items: center; justify-content: flex-end; gap: 8px; font-family: var(--font-mono); font-size: var(--typography-p-body-sm-font-size); font-weight: 500; font-variant-numeric: tabular-nums; white-space: nowrap; color: var(--latency-dead); }
.delay-success { color: var(--latency-good); }.delay-warning { color: var(--latency-mid); }.delay-danger { color: var(--latency-bad); }
.delay-bar { display: inline-flex; width: 48px; height: 3px; border-radius: var(--rounded-pill); background: var(--surface-3); overflow: hidden; }
.delay-bar > span { height: 100%; background: currentColor; border-radius: inherit; }
@media (max-width: 768px) { .delay-bar { display: none; } }
</style>
