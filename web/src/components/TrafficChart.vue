<template>
  <div class="traffic-chart">
    <div v-if="samples.length < 2" class="chart-empty">{{ unavailable ? t('内核未运行或流量采样暂不可用', 'Kernel stopped or traffic samples unavailable') : t('正在等待流量采样…', 'Waiting for traffic samples…') }}</div>
    <svg v-else viewBox="0 0 800 200" role="img" :aria-label="appearance.language === 'en' ? `Last ${samples.length} traffic samples, upload solid, download dashed` : '最近 ' + samples.length + ' 次流量采样，上行实线，下行虚线'" preserveAspectRatio="none">
      <g v-for="level in [0, 1, 2, 3]" :key="level"><line x1="0" x2="800" :y1="12 + level * 56" :y2="12 + level * 56" class="chart-grid" /></g>
      <path :d="area" class="upload-area" /><path :d="up" class="upload-line" /><path :d="down" class="download-line" />
    </svg>
    <div v-if="samples.length >= 2" class="chart-axis"><span>{{ firstTime }}</span><span>{{ formatBytes(maximum) }}/s {{ t('峰值刻度', 'peak scale') }}</span><span>{{ lastTime }}</span></div>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { TrafficSnapshot } from '@/types'
import { formatBytes } from '@/utils/format'
import { useAppearanceStore } from '@/stores/appearance'
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const props = defineProps<{ samples: TrafficSnapshot[]; unavailable?: boolean }>()
const maximum = computed(() => Math.max(1024, ...props.samples.flatMap(s => [s.up, s.down])))
function points(key: 'up' | 'down') {
  return props.samples.map((sample, i) => [i / Math.max(1, props.samples.length - 1) * 800, 180 - Math.max(0, sample[key]) / maximum.value * 168])
}
function path(key: 'up' | 'down') { return points(key).map(([x, y], i) => (i ? 'L' : 'M') + x.toFixed(2) + ',' + y.toFixed(2)).join(' ') }
const up = computed(() => path('up')), down = computed(() => path('down'))
const area = computed(() => up.value + ' L800,180 L0,180 Z')
function time(value?: string) { return value ? new Date(value).toLocaleTimeString(appearance.language, { hour12: false }) : '—' }
const firstTime = computed(() => time(props.samples[0]?.sampled_at))
const lastTime = computed(() => time(props.samples.at(-1)?.sampled_at))
</script>
<style scoped>
.traffic-chart { min-width: 0; }
svg { display: block; width: 100%; height: 200px; overflow: visible; }
.chart-grid { stroke: var(--hairline); stroke-width: 1; vector-effect: non-scaling-stroke; }
.upload-area { fill: var(--chart-fill); opacity: .08; }
.upload-line, .download-line { fill: none; stroke: var(--chart-line); stroke-width: 2; stroke-linejoin: round; vector-effect: non-scaling-stroke; }
.download-line { stroke: var(--chart-line-2); stroke-dasharray: 5 5; }
.chart-axis { display: flex; justify-content: space-between; gap: 8px; color: var(--ink-subtle); font-family: var(--font-mono); font-size: var(--typography-p-micro-font-size); margin-top: 12px; }
.chart-empty { display: grid; place-items: center; min-height: 200px; font-size: var(--typography-p-body-sm-font-size); color: var(--ink-subtle); border-bottom: 1px solid var(--hairline); }
@media (max-width: 480px) { .chart-axis span:nth-child(2) { display: none; } }
</style>
