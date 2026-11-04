<template>
  <el-dialog :model-value="modelValue" :title="isBalance ? t('负载均衡', 'Load balancing') : t('链式代理', 'Proxy chain')" width="620px" class="proxy-group-dialog" :close-on-click-modal="!busy" :close-on-press-escape="!busy" :show-close="!busy" @update:model-value="emit('update:modelValue', $event)">
    <p class="group-description">{{ isBalance ? t('将新连接分配到多个健康节点，不会把单个连接的带宽叠加。保存后会生成一个可单独启动的负载均衡节点。', 'Distributes new connections across healthy nodes without combining bandwidth for an individual connection. Saving creates a load balancing node that can be started separately.') : t('按列表顺序连接：本机 → 入口节点 → 中间节点 → 最终出口 → 目标网站。保存后会生成一个可单独启动的链式代理节点。', 'Connects in order: local device → entry → intermediate nodes → exit → destination. Saving creates a proxy chain node that can be started separately.') }}</p>
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon class="group-error" />
    <el-form label-position="top" :disabled="busy" @submit.prevent>
      <el-form-item :label="t('新节点名称', 'New node name')"><el-input v-model="name" maxlength="256" :placeholder="isBalance ? t('例如：自动负载均衡', 'For example: Auto load balancing') : t('例如：办公室链式代理', 'For example: Office proxy chain')" /></el-form-item>
      <el-form-item :label="isBalance ? t('参与负载均衡的节点', 'Load balancing nodes') : t('按经过顺序选择节点', 'Select nodes in connection order')">
        <el-select v-model="ids" multiple filterable :disabled="busy" :placeholder="t('选择至少两个节点', 'Select at least two nodes')" :multiple-limit="isBalance ? 64 : 8" :aria-label="t('组合代理节点', 'Proxy group nodes')">
          <el-option v-for="node in nodes" :key="node.id" :label="node.name" :value="node.id" :disabled="!eligible(node)"><span>{{ node.name }}</span><span v-if="!eligible(node)" class="unavailable"> · {{ appearance.language === 'en' ? (variant === 'unknown' ? 'Unable to read build capabilities' : 'Not supported by mihomo') : (nodeUnavailableReason(node) || '不支持 mihomo') }}</span></el-option>
        </el-select>
      </el-form-item>
      <ol class="group-members" :aria-label="isBalance ? t('负载均衡节点', 'Load balancing nodes') : t('链式代理顺序', 'Proxy chain order')">
        <li v-for="(id, index) in ids" :key="id">
          <span class="hop-index mono">{{ index + 1 }}</span>
          <span class="member-name">{{ nodeName(id) }}<small v-if="!isBalance">{{ index === 0 ? t('入口节点', 'Entry') : index === ids.length - 1 ? t('最终出口', 'Exit') : t('中间节点', 'Intermediate') }}<template v-if="plan?.supported"> · {{ plan.hops[index]?.kernel }} · {{ plan.hops[index]?.network.toUpperCase() }} {{ t('连接', 'connection') }}</template></small></span>
          <div class="member-actions">
            <template v-if="!isBalance || balance.strategy === 'fallback'"><el-button text size="small" :disabled="busy || index === 0" :aria-label="t('上移 ', 'Move up ') + nodeName(id)" @click="move(index, -1)">{{ t('上移', 'Move up') }}</el-button><el-button text size="small" :disabled="busy || index === ids.length - 1" :aria-label="t('下移 ', 'Move down ') + nodeName(id)" @click="move(index, 1)">{{ t('下移', 'Move down') }}</el-button></template>
            <el-button text size="small" :disabled="busy" :aria-label="t('移除 ', 'Remove ') + nodeName(id)" @click="ids.splice(index, 1)">{{ t('移除', 'Remove') }}</el-button>
          </div>
        </li>
      </ol>
      <template v-if="isBalance"><el-divider /><LoadBalanceFields v-model="balance" /><p class="group-description">{{ balanceStrategyDescription }}</p><p class="group-description">{{ t('这里的设置用于本次启动；可在“设置 → 负载均衡”中保存默认值。', 'These settings apply to this session. Save defaults in Settings → Load balancing.') }}</p></template>
      <template v-else>
        <p v-if="checking" class="group-description" role="status">{{ t('正在检查每一跳的协议和传输能力…', 'Checking protocol and transport capabilities for each hop…') }}</p>
        <el-alert v-if="plan && !plan.supported" class="group-error" :title="translateBackendMessage(plan.reason)" type="warning" :closable="false" show-icon />
        <p v-if="plan?.supported" class="group-description">{{ plan.cross_kernel ? t('双内核桥接：自动连接 sing-box 与 mihomo', 'Dual-kernel bridge: connects sing-box and mihomo automatically') : (appearance.language === 'en' ? `Uses ${plan.kernel}` : `使用 ${plan.kernel} 内核`) }} · {{ plan.udp ? t('链路可承载 TCP / UDP', 'Chain supports TCP / UDP') : t('链路仅承载 TCP', 'Chain supports TCP only') }}</p>
        <p v-for="warning in plan?.warnings ?? []" :key="warning" class="group-description">{{ translateBackendMessage(warning) }}</p>
        <p class="group-description">{{ t('按“本机 → 入口 → 中间节点 → 最终出口”的顺序连接。支持的协议可混用，系统会检查每一跳所需的 TCP / UDP 能力。', 'Connects local device → entry → intermediate nodes → exit. Supported protocols can be mixed; TCP / UDP capabilities are checked for each hop.') }}</p>
      </template>
      <p class="group-description">{{ t('保存后不会启动代理；请在节点列表中点击新节点的“选择并启动”。', 'Saving does not start the proxy; click “Select and start” on the new node in the node list when ready.') }}</p>
    </el-form>
    <template #footer><el-button :disabled="busy" @click="emit('update:modelValue', false)">{{ t('取消', 'Cancel') }}</el-button><el-button type="primary" :loading="busy" :disabled="!name.trim() || ids.length < 2 || (!isBalance && (checking || !plan?.supported))" @click="start">{{ isBalance ? t('生成负载均衡节点', 'Create load balancing node') : t('生成链式代理节点', 'Create proxy chain node') }}</el-button></template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { previewChain } from '@/api/config'
import { createCompositionNode } from '@/api/nodes'
import { ApiError } from '@/api/client'
import { defaultLoadBalance } from '@/stores/config'
import { nodeUnavailableReason, variant } from '@/stores/variant'
import { useAppearanceStore } from '@/stores/appearance'
import { translateBackendMessage } from '@/utils/backendMessages'
import LoadBalanceFields from './LoadBalanceFields.vue'
import type { ChainPlan, LoadBalanceOptions, ProxyGroupMode, ProxyNode } from '@/types'

const props = defineProps<{ modelValue: boolean; mode: ProxyGroupMode; nodes: ProxyNode[]; selectedIds: string[]; defaults: LoadBalanceOptions }>()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; created: [node: ProxyNode]; busy: [value: boolean] }>()
const ids = ref<string[]>([])
const name = ref('')
const balance = ref(defaultLoadBalance())
const busy = ref(false)
const error = ref('')
const isBalance = computed(() => props.mode === 'load_balance')
const balanceStrategyDescription = computed(() => balance.value.strategy === 'fallback'
  ? t('故障切换按下方顺序尝试节点，健康检查发现首选节点不可用时切到下一个健康节点。', 'Fallback tries nodes in order and switches to the next healthy node when the preferred node fails its health check.')
  : balance.value.strategy === 'url-test'
    ? t('自动选优定期检查延迟，并将新连接分配给当前延迟较低的节点。', 'Automatic selection checks latency periodically and assigns new connections to a lower-latency node.')
    : t('负载均衡会按所选策略在健康节点间分配新连接。', 'Load balancing distributes new connections among healthy nodes using the selected strategy.'))
const plan = ref<ChainPlan>()
const checking = ref(false)
let previewVersion = 0
function eligible(node: ProxyNode) { return !node.composition && !nodeUnavailableReason(node) && (isBalance.value ? Boolean(node.owner & 2) : true) }
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
  } catch (err) { if (version === previewVersion) error.value = err instanceof Error ? err.message : t('无法检查链路，请重试', 'Unable to check the chain; try again') }
  finally { if (version === previewVersion) checking.value = false }
})
function nodeName(id: string) { return props.nodes.find(node => node.id === id)?.name ?? t('节点已移除', 'Node removed') }
function move(index: number, offset: number) { const next = [...ids.value]; [next[index], next[index + offset]] = [next[index + offset], next[index]]; ids.value = next }
watch(() => props.modelValue, visible => {
  if (!visible) return
  ids.value = props.selectedIds.filter(id => { const node = props.nodes.find(n => n.id === id); return node && eligible(node) })
  balance.value = { ...defaultLoadBalance(), ...props.defaults }
  name.value = isBalance.value ? t('负载均衡节点', 'Load balancing node') : t('链式代理节点', 'Proxy chain node')
  error.value = ''
})
async function start() {
  if (busy.value) return
  if (!name.value.trim()) { error.value = t('请输入组合节点名称', 'Enter a name for the composition node'); return }
  const selected = ids.value.map(id => props.nodes.find(node => node.id === id))
  if (selected.some(node => !node || !eligible(node))) { error.value = t('所选节点已移除、不支持此方式或不能嵌套组合节点，请重新选择', 'A selected node was removed, unsupported, or is already a composition node; select nodes again'); return }
  if (selected.length < 2) { error.value = t('请至少选择两个节点', 'Select at least two nodes'); return }
  if (!isBalance.value && (checking.value || !plan.value?.supported)) return
  error.value = ''; busy.value = true; emit('busy', true)
  try {
    const node = await createCompositionNode({ name: name.value.trim(), mode: props.mode, node_ids: [...ids.value], ...(isBalance.value ? { load_balance: { ...balance.value } } : {}) })
    emit('created', node)
    emit('update:modelValue', false)
  } catch (err) { error.value = err instanceof ApiError && err.why ? `${err.message}：${err.why}` : err instanceof Error ? err.message : t('生成节点失败', 'Failed to create node') }
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
