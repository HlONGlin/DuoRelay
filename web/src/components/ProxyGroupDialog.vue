<template>
  <el-dialog :model-value="modelValue" :title="isBalance ? '负载均衡' : '链式代理'" width="620px" class="proxy-group-dialog" :close-on-click-modal="!busy" :close-on-press-escape="!busy" :show-close="!busy" @update:model-value="emit('update:modelValue', $event)">
    <p class="group-description">{{ isBalance ? '将新连接分配到多个健康节点，不会把单个连接的带宽叠加。启动时使用 mihomo 内核。' : '按列表顺序连接：本机 → 入口节点 → 中间节点 → 最终出口 → 目标网站。' }}</p>
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon class="group-error" />
    <el-form label-position="top" :disabled="busy" @submit.prevent>
      <el-form-item :label="isBalance ? '参与负载均衡的节点' : '按经过顺序选择节点'">
        <el-select v-model="ids" multiple filterable :disabled="busy" placeholder="选择至少两个节点" :multiple-limit="isBalance ? 64 : 8" aria-label="组合代理节点">
          <el-option v-for="node in nodes" :key="node.id" :label="node.name" :value="node.id" :disabled="!eligible(node)"><span>{{ node.name }}</span><span v-if="!eligible(node)" class="unavailable"> · {{ nodeUnavailableReason(node) || '不支持 mihomo' }}</span></el-option>
        </el-select>
      </el-form-item>
      <ol class="group-members" :aria-label="isBalance ? '负载均衡节点' : '链式代理顺序'">
        <li v-for="(id, index) in ids" :key="id">
          <span class="hop-index mono">{{ index + 1 }}</span>
          <span class="member-name">{{ nodeName(id) }}<small v-if="!isBalance">{{ index === 0 ? '入口节点' : index === ids.length - 1 ? '最终出口' : '中间节点' }}<template v-if="plan?.supported"> · {{ plan.hops[index]?.kernel }} · {{ plan.hops[index]?.network.toUpperCase() }} 连接</template></small></span>
          <div class="member-actions">
            <template v-if="!isBalance || balance.strategy === 'fallback'"><el-button text size="small" :disabled="busy || index === 0" :aria-label="'上移 ' + nodeName(id)" @click="move(index, -1)">上移</el-button><el-button text size="small" :disabled="busy || index === ids.length - 1" :aria-label="'下移 ' + nodeName(id)" @click="move(index, 1)">下移</el-button></template>
            <el-button text size="small" :disabled="busy" :aria-label="'移除 ' + nodeName(id)" @click="ids.splice(index, 1)">移除</el-button>
          </div>
        </li>
      </ol>
      <template v-if="isBalance"><el-divider /><LoadBalanceFields v-model="balance" /><p class="group-description">{{ balanceStrategyDescription }}</p><p class="group-description">这里的设置用于本次启动；可在“设置 → 负载均衡”中保存默认值。</p></template>
      <template v-else>
        <p v-if="checking" class="group-description" role="status">正在检查每一跳的协议和传输能力…</p>
        <el-alert v-if="plan && !plan.supported" class="group-error" :title="plan.reason" type="warning" :closable="false" show-icon />
        <p v-if="plan?.supported" class="group-description">{{ plan.cross_kernel ? '双内核桥接：自动连接 sing-box 与 mihomo' : `使用 ${plan.kernel} 内核` }} · {{ plan.udp ? '链路可承载 TCP / UDP' : '链路仅承载 TCP' }}</p>
        <p v-for="warning in plan?.warnings ?? []" :key="warning" class="group-description">{{ warning }}</p>
        <p class="group-description">按“本机 → 入口 → 中间节点 → 最终出口”的顺序连接。支持的协议可混用，系统会检查每一跳所需的 TCP / UDP 能力。</p>
      </template>
      <p class="group-description">启动会替换当前代理配置；校验失败时保留原配置。</p>
    </el-form>
    <template #footer><el-button :disabled="busy" @click="emit('update:modelValue', false)">取消</el-button><el-button type="primary" :loading="busy" :disabled="ids.length < 2 || (!isBalance && (checking || !plan?.supported))" @click="start">{{ isBalance ? '启动负载均衡' : '启动链式代理' }}</el-button></template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { applyConfig, previewChain } from '@/api/config'
import { ApiError } from '@/api/client'
import { defaultLoadBalance } from '@/stores/config'
import { isLite, nodeUnavailableReason } from '@/stores/variant'
import LoadBalanceFields from './LoadBalanceFields.vue'
import type { ChainPlan, ConfigurationState, LoadBalanceOptions, ProxyGroupMode, ProxyNode } from '@/types'

const props = defineProps<{ modelValue: boolean; mode: ProxyGroupMode; nodes: ProxyNode[]; selectedIds: string[]; defaults: LoadBalanceOptions }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; started: [state: ConfigurationState]; busy: [value: boolean] }>()
const ids = ref<string[]>([])
const balance = ref(defaultLoadBalance())
const busy = ref(false)
const error = ref('')
const isBalance = computed(() => props.mode === 'load_balance')
const balanceStrategyDescription = computed(() => balance.value.strategy === 'fallback'
  ? '故障切换按下方顺序尝试节点，健康检查发现首选节点不可用时切到下一个健康节点。'
  : balance.value.strategy === 'url-test'
    ? '自动选优定期检查延迟，并将新连接分配给当前延迟较低的节点。'
    : '负载均衡会按所选策略在健康节点间分配新连接。')
const plan = ref<ChainPlan>()
const checking = ref(false)
let previewVersion = 0
function eligible(node: ProxyNode) { return !nodeUnavailableReason(node) && (isBalance.value ? Boolean(node.owner & 2) : true) }
watch([() => [...ids.value], () => props.modelValue, () => props.mode], async () => {
  const version = ++previewVersion
  plan.value = undefined
  checking.value = false
  if (!props.modelValue || isBalance.value || ids.value.length < 2) return
  checking.value = true
  error.value = ''
  try {
    const value = await previewChain([...ids.value])
    if (version === previewVersion) plan.value = value
  } catch (err) { if (version === previewVersion) error.value = err instanceof Error ? err.message : '无法检查链路，请重试' }
  finally { if (version === previewVersion) checking.value = false }
})
function nodeName(id: string) { return props.nodes.find(node => node.id === id)?.name ?? '节点已移除' }
function move(index: number, offset: number) { const next = [...ids.value]; [next[index], next[index + offset]] = [next[index + offset], next[index]]; ids.value = next }
watch(() => props.modelValue, visible => {
  if (!visible) return
  ids.value = isLite.value ? props.selectedIds.filter(id => { const node = props.nodes.find(n => n.id === id); return node && eligible(node) }) : [...props.selectedIds]
  balance.value = { ...defaultLoadBalance(), ...props.defaults }
  error.value = ''
})
async function start() {
  if (busy.value) return
  const selected = ids.value.map(id => props.nodes.find(node => node.id === id))
  if (selected.some(node => !node || !eligible(node))) { error.value = '所选节点已移除或不支持此启动方式，请重新选择'; return }
  if (selected.length < 2) { error.value = '请至少选择两个节点'; return }
  if (!isBalance.value && (checking.value || !plan.value?.supported)) return
  error.value = ''; busy.value = true; emit('busy', true)
  try {
    const state = await applyConfig({ mode: props.mode, node_ids: [...ids.value], ...(isBalance.value ? { load_balance: { ...balance.value } } : {}) })
    emit('started', state)
    emit('update:modelValue', false)
  } catch (err) { error.value = err instanceof ApiError && err.why ? `${err.message}：${err.why}` : err instanceof Error ? err.message : '启动失败' }
  finally { busy.value = false; emit('busy', false) }
}
</script>

<style scoped>
.group-description { color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); line-height: 1.7; margin: 0 0 16px; }
.group-error { margin-bottom: 16px; }
.el-select { width: 100%; }
.group-members { margin: 0; padding: 0; list-style: none; max-height: 260px; overflow-y: auto; }
.group-members li { display: flex; align-items: center; gap: 12px; padding: 10px 0; border-bottom: 1px solid var(--hairline); }
.hop-index { color: var(--ink-subtle); width: 22px; flex-shrink: 0; }
.member-name { flex: 1; min-width: 0; overflow-wrap: anywhere; color: var(--ink); }
.member-name small { display: block; color: var(--ink-subtle); margin-top: 3px; }
.member-actions { display: flex; flex-wrap: wrap; justify-content: end; }
.member-actions :deep(.el-button + .el-button) { margin-left: 0; }
.unavailable { color: var(--ink-subtle); }
</style>
