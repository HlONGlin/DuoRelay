<template>
  <div class="page-stack settings">
    <div class="page-heading"><h1>设置</h1><span v-if="store.savedAt" class="muted type-p-caption">上次保存 {{ formatDate(store.savedAt) }}</span></div>
    <el-alert v-if="store.error" :title="store.error" type="warning" :closable="false" show-icon />
    <div class="settings-layout">
      <nav class="category-nav" aria-label="设置分类">
        <button type="button" v-for="item in categories" :key="item.id" :class="{ selected: category === item.id }" :aria-current="category === item.id ? 'page' : undefined" @click="category = item.id"><UiIcon :name="item.icon" :size="16" />{{ item.title }}</button>
      </nav>
      <section v-loading="store.loading" class="panel settings-panel">
        <div class="settings-section-heading"><h2>{{ currentCategory.title }}</h2><p>{{ currentCategory.description }}</p></div>
        <el-form @submit.prevent :model="form" label-position="top" class="settings-form">
          <template v-if="category === 'general'">
            <div class="form-grid">
              <el-form-item label="监听地址"><el-input v-model="form.inbound_host" placeholder="127.0.0.1" /><div class="field-hint">仅本机使用时保留 127.0.0.1。</div></el-form-item>
              <el-form-item label="允许局域网连接"><el-switch v-model="form.allow_lan" /><div class="field-hint">其他设备访问时需同时调整监听地址。</div></el-form-item>
              <el-form-item label="sing-box 混合代理端口"><el-input-number v-model="form.inbound_port" :min="1" :max="65535" /></el-form-item>
              <el-form-item label="mihomo 混合代理端口"><el-input-number v-model="form.mixed_port" :min="1" :max="65535" /></el-form-item>
            </div>
            <el-divider />
            <el-form-item label="仅 SOCKS5 端口使用代理"><el-switch v-model="form.socks_only" aria-label="仅 SOCKS5 端口使用代理" @change="changeSocksOnly" /><div class="field-hint">开启后，仅下方独立 SOCKS5 端口按路由规则使用代理；混合端口直连。此模式不能与 TUN 全流量接管同时启用。</div></el-form-item>
            <el-form-item v-if="form.socks_only" label="独立 SOCKS5 端口"><el-input-number v-model="form.socks_port" :min="1" :max="65535" aria-label="独立 SOCKS5 端口" /><div class="field-hint">请与上方两个混合端口保持不同。SOCKS5 端口会使用当前单节点、负载均衡或代理链。</div></el-form-item>
            <el-divider />
            <el-form-item label="DNS 服务器"><el-input v-model="dnsText" type="textarea" :rows="4" placeholder="每行一个 DNS 服务器" /><div class="field-hint">例如 223.5.5.5，每行填写一个地址。</div></el-form-item>
            <el-form-item label="Fake-IP"><el-switch v-model="form.dns_fake_ip" /><div class="field-hint">使用虚拟 IP 映射域名，需配合对应内核的 DNS 配置。</div></el-form-item>
          </template>
          <template v-if="category === 'kernel'">
            <el-form-item label="首选内核"><el-radio-group v-model="form.kernel"><el-radio-button value="sing-box">sing-box</el-radio-button><el-radio-button value="mihomo">mihomo</el-radio-button></el-radio-group><div class="field-hint">节点仅支持另一内核时，会按协议能力选择。</div></el-form-item>
            <el-divider />
            <el-form-item label="透明代理模式">
              <el-select v-model="form.transparent_mode" @change="changeTransparentMode">
                <el-option v-for="mode in transparentModes" :key="mode.mode" :value="mode.mode" :label="transparentModeLabel(mode.mode)" :disabled="!mode.supported"><span>{{ transparentModeLabel(mode.mode) }}</span><small v-if="!mode.supported">{{ mode.reason }}</small></el-option>
              </el-select>
              <div class="field-hint">{{ selectedTransparentMode?.prerequisites }}</div>
              <div class="field-hint">DNS：{{ selectedTransparentMode?.dns_description }}</div>
              <div class="field-hint">当前生效：{{ transparentModeLabel(form.transparent_active ?? 'off') }}</div>
            </el-form-item>
            <el-form-item label="TUN 全流量接管"><el-switch v-model="form.enable_tun" :disabled="!transparentModes.find((mode) => mode.mode === 'tun')?.supported" aria-label="TUN 全流量接管" @change="changeTun" /><div class="field-hint">接管 IPv4/IPv6 系统流量并劫持 TCP/UDP DNS；启用严格路由防止流量回落直连。需要管理员或 NET_ADMIN 权限及可用的 TUN 设备。路由规则仍决定哪些流量直连。</div></el-form-item>
            <el-form-item label="TUN 网络栈"><el-select v-model="form.tun_stack" :disabled="!form.enable_tun"><el-option v-for="stack in ['system', 'gvisor', 'mixed']" :key="stack" :value="stack" :label="stack" /></el-select></el-form-item>
            <el-divider />
            <el-form-item label="系统代理"><el-switch v-model="form.system_proxy" :disabled="!form.system_proxy_supported" /><div class="field-hint">{{ form.system_proxy_supported ? '将系统代理指向本地入口，退出或启动失败时恢复原设置。只接管遵循系统代理设置的应用；要接管设备的全部流量，请启用 TUN。' : '当前环境不支持系统代理接管；Windows 或带 GNOME D-Bus 会话的 Linux 可用。要接管设备的全部流量，请启用 TUN。' }}</div></el-form-item>
            <router-link to="/kernel" class="settings-link">查看运行状态与日志 →</router-link>
          </template>
          <template v-if="category === 'routing'">
            <el-form-item label="路由分组"><el-radio-group v-model="form.routing_mode" @change="applyRoutingMode"><el-radio-button v-for="mode in routingModes" :key="mode.value" :value="mode.value">{{ mode.label }}</el-radio-button></el-radio-group><div class="field-hint">{{ currentModeDescription }}</div></el-form-item>
            <el-form-item label="未匹配流量出口"><el-select v-model="form.default_outbound"><el-option v-for="outbound in outboundOptions" :key="outbound" :label="outbound === 'PROXY' ? '使用当前选择的代理节点' : outbound === 'DIRECT' ? '直接连接（DIRECT）' : outbound === 'REJECT' ? '阻断连接（REJECT）' : outbound" :value="outbound" /></el-select><div class="field-hint">流量未命中下方规则时使用此出口。代理出口会跟随当前启用的节点。</div></el-form-item>
            <div class="rules-heading"><span class="type-p-card-title">自定义规则 <span class="muted mono">{{ routingForms.length }}</span></span><el-button size="small" @click="openAddRule"><UiIcon name="plus" :size="16" />添加规则</el-button></div>
            <p class="field-hint rule-hint">从上到下匹配。切换分组会保留自定义规则；大陆分流范围取决于已配置的域名与 IP 规则。</p>
            <el-radio-group v-model="routingEditorMode" size="small" @change="syncRoutingTextFromList"><el-radio-button value="list">列表</el-radio-button><el-radio-button value="text">文本</el-radio-button></el-radio-group>
            <template v-if="routingEditorMode === 'text'">
              <div class="routing-text-wrap"><div class="line-numbers"><span v-for="line in routingTextLineCount" :key="line">{{ line }}</span></div><el-input v-model="routingText" type="textarea" :rows="14" class="routing-text" @input="validateRoutingEditor" /></div>
              <div v-for="error in routingTextErrors" :key="`${error.line}-${error.reason}`" class="routing-error">{{ error.line ? `第 ${error.line} 行：` : '' }}{{ error.reason }}</div>
              <div class="routing-actions"><el-button size="small" @click="applyRoutingText">回填列表</el-button><el-button size="small" @click="downloadRoutingText">导出文本</el-button><el-button size="small" @click="chooseRoutingFile">导入文本</el-button><input ref="routingFileInput" type="file" accept=".txt,text/plain" hidden @change="readRoutingFile" /></div>
            </template>
            <div class="table-scroll"><el-table :data="routingForms" class="rules-table" size="small" empty-text="暂无自定义规则">
              <el-table-column type="index" label="#" width="44" />
              <el-table-column label="匹配类型" width="116"><template #default="{ row }">{{ matchLabel(row.match) }}</template></el-table-column>
              <el-table-column label="来源" width="90"><template #default="{ row }">{{ row.source }}</template></el-table-column>
              <el-table-column label="域名 / IP / 规则值" min-width="200"><template #default="{ row, $index }"><button type="button" class="rule-summary" :title="row.valuesText" @click="openEditRule($index)">{{ valueSummary(row) }}</button></template></el-table-column>
              <el-table-column label="出口" width="100"><template #default="{ row }"><span class="neutral-badge">{{ row.outbound }}</span></template></el-table-column>
              <el-table-column label="启用" width="70"><template #default="{ row }"><el-switch v-model="row.enabled" /></template></el-table-column>
              <el-table-column label="操作" width="150"><template #default="{ $index }"><el-button link aria-label="上移规则" :disabled="$index === 0" @click="moveRoutingRule($index, -1)"><UiIcon name="up" :size="16" /></el-button><el-button link aria-label="下移规则" :disabled="$index === routingForms.length - 1" @click="moveRoutingRule($index, 1)"><UiIcon name="down" :size="16" /></el-button><el-button link aria-label="编辑规则" @click="openEditRule($index)"><UiIcon name="edit" :size="16" /></el-button><el-button link aria-label="删除规则" @click="removeRoutingRule($index)"><UiIcon name="trash" :size="16" /></el-button></template></el-table-column>
            </el-table></div>
          </template>
          <template v-if="category === 'subscription'">
            <div class="subscription-info"><p>更新周期与 User-Agent 在各订阅中设置。</p><router-link to="/subscriptions">打开订阅管理</router-link></div>
          </template>
          <template v-if="category === 'balance'"><LoadBalanceFields v-model="form.load_balance" /><p class="field-hint">保存为负载均衡弹窗的默认值。已有运行实例保持本次启动时的设置，重新启动负载均衡后应用新设置。</p><router-link to="/nodes" class="settings-link">前往节点管理，选择多个节点并启动 →</router-link></template>
          <template v-if="category === 'appearance'">
            <p class="field-hint appearance-hint">主题立即应用，并保存在当前浏览器中。</p>
            <div class="theme-options">
              <button type="button" v-for="theme in (['light', 'dark'] as const)" :key="theme" class="theme-option" :class="{ chosen: appearance.theme === theme }" :aria-pressed="appearance.theme === theme" @click="appearance.setTheme(theme)">
                <div class="theme-preview" :data-theme="theme"><div class="preview-sidebar"><i /><i /><i /></div><div class="preview-main"><div class="preview-top" /><div class="preview-cards"><i /><i /></div><div class="preview-table"><i /><i /><i /></div></div></div>
                <span>{{ theme === 'light' ? '浅色' : '深色' }}<UiIcon v-if="appearance.theme === theme" name="check" :size="16" /></span>
              </button>
            </div>
          </template>
          <template v-if="category === 'advanced'">
            <el-form-item label="日志级别"><el-select v-model="form.log_level"><el-option v-for="level in ['debug', 'info', 'warn', 'error']" :key="level" :value="level" :label="level" /></el-select><div class="field-hint">日常使用推荐 info；排查问题时选择 debug。</div></el-form-item>
            <el-divider /><div class="reset-settings"><div><h3>恢复默认配置</h3><p class="field-hint">先填入默认值，保存后才会覆盖服务器配置。</p></div><el-button @click="handleReset">填入默认值</el-button></div>
          </template>
        </el-form>
      </section>
    </div>
    <div v-if="category !== 'appearance' && category !== 'subscription'" class="save-bar"><span>修改保存后，重新选择并启动节点以应用。</span><el-button type="primary" :disabled="store.loading" :loading="store.saving" @click="handleSave">保存设置</el-button></div>
    <el-dialog v-model="ruleDialogVisible" :title="editingIndex < 0 ? '添加路由规则' : '编辑路由规则'" width="560px">
      <el-form @submit.prevent label-position="top">
        <el-form-item label="来源"><el-select v-model="editingRule.source" :disabled="editingRule.source !== 'user'"><el-option label="用户规则" value="user" /><el-option label="绕过规则" value="bypass" /><el-option label="内部路由" value="internal" /></el-select></el-form-item>
        <el-form-item label="匹配类型"><el-select v-model="editingRule.match"><el-option v-for="option in routingMatchOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
        <el-form-item label="域名 / IP / 规则值"><el-input v-model="editingRule.valuesText" type="textarea" :rows="8" placeholder="每行填写一条规则" class="rule-input" /><div class="field-hint">内容需要与匹配类型一致，例如域名后缀 example.com，或 CIDR 192.168.1.0/24。</div></el-form-item>
        <el-form-item label="流量出口"><el-select v-model="editingRule.outbound" filterable><el-option v-for="outbound in outboundOptions" :key="outbound" :label="outbound" :value="outbound" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="ruleDialogVisible = false">取消</el-button><el-button type="primary" @click="saveRule">确定</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { defaultConfig, useConfigStore } from '@/stores/config'
import { useNodesStore } from '@/stores/nodes'
import { useAppearanceStore } from '@/stores/appearance'
import UiIcon from '@/components/UiIcon.vue'
import LoadBalanceFields from '@/components/LoadBalanceFields.vue'
import { formatDate } from '@/utils/format'
import type { AppConfig, RoutingMatch, RoutingMode, RoutingRule } from '@/types'
import { exportRouting, importRoutingText, validateRoutingText } from '@/api/config'

const store = useConfigStore()
const nodes = useNodesStore()
const appearance = useAppearanceStore()
const category = ref('general')
const categories = [
  { id: 'general', title: '通用', icon: 'settings', description: '监听地址、端口与 DNS' },
  { id: 'kernel', title: '内核', icon: 'terminal', description: '双内核偏好与 TUN' },
  { id: 'routing', title: '路由规则', icon: 'nodes', description: '流量分流与规则管理' },
  { id: 'balance', title: '负载均衡', icon: 'nodes', description: '多出口分配策略与健康检查默认值' },
  { id: 'subscription', title: '订阅', icon: 'link', description: '订阅更新与内容导入' },
  { id: 'appearance', title: '外观', icon: 'sun', description: '浅色与深色主题' },
  { id: 'advanced', title: '高级', icon: 'grid', description: '日志与配置恢复' }
]
const currentCategory = computed(() => categories.find(item => item.id === category.value)!)


const form = reactive<AppConfig>({ ...store.config, load_balance: { ...store.config.load_balance } })
const dnsText = ref(store.config.dns_servers.join('\n'))
const routingModes: Array<{ label: string; value: RoutingMode; description: string }> = [
  { label: '绕过大陆', value: 'bypass_mainland', description: '中国大陆及局域网直连，其余流量使用代理。' },
  { label: '全局', value: 'global', description: '所有未命中自定义规则的流量使用代理。' },
  { label: '黑名单', value: 'blacklist', description: '默认直连，仅把自定义规则命中的流量送入代理。' }
]
const routingMatchOptions: Array<{ label: string; value: RoutingMatch }> = [
  { label: '域名', value: 'domain' },
  { label: '域名后缀', value: 'domain_suffix' },
  { label: '域名关键词', value: 'domain_keyword' },
  { label: '目标 CIDR', value: 'ip_cidr' },
  { label: '来源 CIDR', value: 'source_ip_cidr' },
  { label: '目标端口', value: 'port' },
  { label: '网络类型', value: 'network' },
  { label: '协议', value: 'protocol' },
  { label: 'GeoSite', value: 'geosite' },
  { label: 'GeoIP', value: 'geoip' }
]

interface RoutingForm {
  id?: string
  source?: string
  enabled?: boolean
  match: RoutingMatch
  valuesText: string
  outbound: string
}

const routingForms = ref<RoutingForm[]>([])
const routingEditorMode = ref<'list' | 'text'>('list')
const routingText = ref('')
const routingTextErrors = ref<{ line: number; reason: string }[]>([])
const routingFileInput = ref<HTMLInputElement>()
const routingTextLineCount = computed(() => Math.max(1, routingText.value.split('\n').length))
const ruleDialogVisible = ref(false)
const editingIndex = ref(-1)
const editingRule = reactive<RoutingForm>({ source: 'user', enabled: true, match: 'domain_suffix', valuesText: '', outbound: 'PROXY' })

const outboundOptions = computed(() =>
  Array.from(new Set(['DIRECT', 'PROXY', 'REJECT', ...nodes.items.map((node) => node.name)]))
)
const currentModeDescription = computed(
  () => routingModes.find((mode) => mode.value === form.routing_mode)?.description ?? ''
)
const fallbackTransparentModes = [
  { mode: 'off', supported: true, prerequisites: '无需系统权限', dns_description: '不接管 DNS。' },
  { mode: 'redirect', supported: false, reason: '当前系统未报告可用能力', prerequisites: '需要 Linux nftables/iptables、iproute2 和 NET_ADMIN。', dns_description: '不接管 UDP DNS。' },
  { mode: 'tproxy', supported: false, reason: '当前系统未报告可用能力', prerequisites: '需要 Linux TPROXY、策略路由和 NET_ADMIN。', dns_description: '需要显式配置代理 DNS。' },
  { mode: 'tun', supported: false, reason: '当前系统未报告可用能力', prerequisites: '需要 /dev/net/tun 和 NET_ADMIN。', dns_description: '由内核 DNS hijack 接管。' }
] as AppConfig['transparent_modes']
const transparentModes = computed(() => (store.config.transparent_modes?.length ? store.config.transparent_modes : fallbackTransparentModes) as NonNullable<AppConfig['transparent_modes']>)
const selectedTransparentMode = computed(() => transparentModes.value.find((mode) => mode.mode === form.transparent_mode))
function transparentModeLabel(mode: string) { return ({ off: '关闭', redirect: 'Linux redirect', tproxy: 'Linux TPROXY', tun: 'TUN 全流量' } as Record<string, string>)[mode] ?? mode }

function chinaRules(): RoutingForm[] {
  return [
    { match: 'domain_suffix', valuesText: 'cn\n中国\nlocal', outbound: 'DIRECT' },
    { match: 'ip_cidr', valuesText: '10.0.0.0/8\n100.64.0.0/10\n172.16.0.0/12\n192.168.0.0/16', outbound: 'DIRECT' }
  ]
}

function syncFromStore(config: AppConfig) {
  Object.assign(form, config, { routing_mode: config.routing_mode || 'bypass_mainland', load_balance: { ...config.load_balance } })
  dnsText.value = config.dns_servers.join('\n')
  routingForms.value = (config.routing_rules ?? []).map((rule) => ({
    id: rule.id,
    source: rule.source ?? 'user',
    enabled: rule.enabled !== false,
    match: rule.match,
    valuesText: rule.values.join('\n'),
    outbound: rule.outbound
  }))
  routingText.value = formatRoutingText()
  routingTextErrors.value = []
}

function applyRoutingMode(mode: RoutingMode) {
  form.routing_mode = mode
  form.default_outbound = mode === 'blacklist' ? 'DIRECT' : 'PROXY'
  // Mode changes retain user rules. Only manage the exact bundled defaults.
  const defaults = chinaRules()
  const same = (a: RoutingForm, b: RoutingForm) =>
    a.match === b.match && a.outbound === b.outbound && a.valuesText === b.valuesText
  const custom = routingForms.value.filter(rule => !defaults.some(preset => same(rule, preset)))
  routingForms.value = mode === 'bypass_mainland' ? [...custom, ...defaults] : custom
}

function changeTun(enabled: string | number | boolean) {
  if (enabled) { form.socks_only = false; form.transparent_mode = 'tun' }
}

function changeSocksOnly(enabled: string | number | boolean) {
  if (enabled) { form.enable_tun = false; if (form.transparent_mode === 'tun') form.transparent_mode = 'off' }
}

function changeTransparentMode(mode: string) {
  form.enable_tun = mode === 'tun'
  if (mode === 'tun') form.socks_only = false
}

function emptyRule(): RoutingForm {
  return { source: 'user', enabled: true, match: 'domain_suffix', valuesText: '', outbound: form.routing_mode === 'blacklist' ? 'PROXY' : 'DIRECT' }
}

function openAddRule() {
  editingIndex.value = -1
  Object.assign(editingRule, emptyRule())
  ruleDialogVisible.value = true
}

function openEditRule(index: number) {
  editingIndex.value = index
  Object.assign(editingRule, routingForms.value[index])
  ruleDialogVisible.value = true
}

function parseValues(value: string): string[] {
  return value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean)
}

function saveRule() {
  const values = parseValues(editingRule.valuesText)
  const outbound = editingRule.outbound.trim()
  if (values.length === 0) {
    ElMessage.warning('请至少填写一条域名或 IP 规则')
    return
  }
  if (!outbound) {
    ElMessage.warning('请选择流量出口')
    return
  }
  const next = { ...editingRule, match: editingRule.match, valuesText: values.join('\n'), outbound }
  if (editingIndex.value < 0) routingForms.value.push(next)
  else routingForms.value.splice(editingIndex.value, 1, next)
  ruleDialogVisible.value = false
}

function removeRoutingRule(index: number) {
  routingForms.value.splice(index, 1)
}

function moveRoutingRule(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= routingForms.value.length) return
  const [rule] = routingForms.value.splice(index, 1)
  routingForms.value.splice(target, 0, rule)
}

function matchLabel(match: RoutingMatch): string {
  return routingMatchOptions.find((option) => option.value === match)?.label ?? match
}

function valueSummary(rule: RoutingForm): string {
  const values = parseValues(rule.valuesText)
  return values.length > 2 ? `${values.slice(0, 2).join('、')} 等 ${values.length} 项` : values.join('、')
}

function routingRules(): RoutingRule[] {
	return routingForms.value.map((rule, index) => ({
		id: rule.id ?? `rule-${index + 1}`,
    source: rule.source ?? 'user',
    enabled: rule.enabled !== false,
    match: rule.match,
    values: parseValues(rule.valuesText),
    outbound: rule.outbound.trim()
  }))
}

function formatRoutingText() {
  return routingForms.value.map((rule) => `${rule.enabled ? '1' : '0'}\t${rule.source}\t${rule.match}\t${parseValues(rule.valuesText).join(',')}\t${rule.outbound}`).join('\n') + (routingForms.value.length ? '\n' : '')
}
function syncRoutingTextFromList() { if (routingEditorMode.value === 'text') routingText.value = formatRoutingText() }

async function validateRoutingEditor() {
  try { routingTextErrors.value = (await validateRoutingText(routingText.value)).errors ?? [] } catch (err) { routingTextErrors.value = [{ line: 0, reason: err instanceof Error ? err.message : '文本校验失败' }] }
}

async function applyRoutingText() {
  const result = await importRoutingText(routingText.value)
  routingTextErrors.value = result.errors ?? []
  if (routingTextErrors.value.length) { ElMessage.warning('文本存在错误，非法行未回填'); return }
  routingForms.value = result.rules.map((rule) => ({ id: rule.id, source: rule.source ?? 'user', enabled: rule.enabled !== false, match: rule.match, valuesText: rule.values.join('\n'), outbound: rule.outbound }))
  ElMessage.success('文本已回填列表')
}

async function downloadRoutingText() {
  const result = await exportRouting()
  const url = URL.createObjectURL(new Blob([result.text], { type: 'text/plain;charset=utf-8' }))
  const anchor = document.createElement('a'); anchor.href = url; anchor.download = 'duorelay-routing.txt'; anchor.click(); URL.revokeObjectURL(url)
}

function chooseRoutingFile() { routingFileInput.value?.click() }
async function readRoutingFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  routingText.value = await file.text()
  await validateRoutingEditor()
  routingEditorMode.value = 'text'
}

async function handleSave() {
  if (form.socks_only && (form.socks_port === form.inbound_port || form.socks_port === form.mixed_port)) {
    ElMessage.warning('独立 SOCKS5 端口不能与混合代理端口相同')
    return
  }
  const servers = dnsText.value.split('\n').map((line) => line.trim()).filter(Boolean)
  if (servers.length === 0) {
    ElMessage.warning('请至少配置一个 DNS 服务器')
    return
  }
  const rules = routingRules()
  if (rules.some((rule) => rule.values.length === 0 || !rule.outbound)) {
    ElMessage.warning('请完善路由规则后再保存')
    return
  }
  if (routingEditorMode.value === 'text') {
    await validateRoutingEditor()
    if (routingTextErrors.value.length) { ElMessage.warning('请先修复文本规则错误'); return }
  }
  try {
    await store.save({ ...form, dns_servers: servers, routing_rules: rules })
    ElMessage.success('设置已保存；重新选择并启动节点后应用到内核')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '保存设置失败')
  }
}

function handleReset() {
  syncFromStore(defaultConfig())
  ElMessage.info('已恢复默认值，点击保存后生效')
}

onMounted(async () => {
  await Promise.all([store.fetchConfig(), nodes.fetchAllNodes()])
  if (!store.error) syncFromStore(store.config)
})
</script>

<style scoped>
.settings-layout { display: grid; grid-template-columns: 168px minmax(0, 1fr); gap: 32px; align-items: start; }
.category-nav { display: flex; flex-direction: column; gap: 4px; position: sticky; top: 24px; }
.category-nav button { border: 0; background: transparent; border-radius: var(--rounded-sm); display: flex; align-items: center; gap: 10px; padding: 12px; text-align: left; font-size: var(--typography-p-body-sm-font-size); color: var(--ink-muted); min-height: 44px; }
.category-nav button:hover { background: var(--surface-2); }
.category-nav button.selected { background: var(--primary-soft); color: var(--primary-deep); font-weight: 500; }
.settings-panel { min-height: 450px; max-width: 980px; padding: 32px; }
.settings-section-heading { border-bottom: 1px solid var(--hairline); padding-bottom: 24px; margin-bottom: 24px; }
.settings-section-heading h2 { font-size: var(--typography-p-headline-font-size); font-weight: 600; }
.settings-section-heading p { font-size: var(--typography-p-body-sm-font-size); color: var(--ink-subtle); margin-top: 4px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 24px; }
.field-hint { color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); line-height: 1.5; margin-top: 8px; width: 100%; }
.settings-form :deep(.el-select) { max-width: 400px; }
.rules-heading { display: flex; align-items: center; justify-content: space-between; margin-top: 32px; }
.rule-hint { margin-bottom: 16px; }
.rules-table { min-width: 600px; }
.rule-summary { border: 0; background: transparent; color: var(--ink-muted); text-align: left; font-family: var(--font-mono); white-space: nowrap; max-width: 100%; overflow: hidden; text-overflow: ellipsis; text-decoration: underline dotted var(--hairline-tertiary); text-underline-offset: 4px; }
.rule-summary:hover { color: var(--primary); }
.rule-input :deep(textarea) { font-family: var(--font-mono); }
.routing-text-wrap { display: grid; grid-template-columns: 42px minmax(0, 1fr); margin-top: 16px; border: 1px solid var(--hairline); background: var(--surface-1); }
.line-numbers { padding: 12px 8px; text-align: right; color: var(--ink-subtle); background: var(--surface-2); font: 12px/1.5 var(--font-mono); user-select: none; }
.line-numbers span { display: block; }
.routing-text :deep(textarea) { border: 0; border-radius: 0; font: 12px/1.5 var(--font-mono); min-height: 280px; }
.routing-actions { display: flex; gap: 8px; margin: 12px 0; flex-wrap: wrap; }
.routing-error { color: var(--danger); font: 12px/1.5 var(--font-mono); margin-top: 4px; }
.routing-text-wrap + .routing-error { margin-top: 8px; }
.save-bar { position: sticky; bottom: 16px; align-self: flex-end; display: flex; align-items: center; gap: 24px; padding: 12px 16px; background: var(--surface-2); border: 1px solid var(--hairline-strong); border-radius: var(--rounded-lg); z-index: 5; }
.save-bar span { color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); }
.theme-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; max-width: 600px; }
.appearance-hint { margin-bottom: 24px; }
.theme-option { border: 1px solid var(--hairline); padding: 8px; background: transparent; border-radius: var(--rounded-lg); color: var(--ink-muted); }
.theme-option.chosen { border-color: var(--hairline-strong); outline: 1px solid var(--hairline-strong); }
.theme-option > span { display: flex; justify-content: space-between; align-items: center; padding: 12px 8px 4px; font-size: var(--typography-p-body-sm-font-size); }
.theme-option.chosen > span { color: var(--primary); }
.theme-preview { height: 150px; display: flex; background: var(--canvas); border: 1px solid var(--hairline); border-radius: var(--rounded-sm); overflow: hidden; }
.preview-sidebar { width: 24%; padding: 22px 8px; display: flex; flex-direction: column; gap: 12px; background: var(--surface-1); border-right: 1px solid var(--hairline); }
.preview-sidebar i { height: 6px; background: var(--surface-4); border-radius: var(--rounded-xs); }
.preview-sidebar i:first-child { background: var(--primary); }
.preview-main { flex: 1; padding: 12px; }
.preview-top { height: 8px; width: 40%; background: var(--surface-4); margin-bottom: 16px; border-radius: var(--rounded-xs); }
.preview-cards { display: flex; gap: 8px; }
.preview-cards i { height: 36px; flex: 1; background: var(--surface-1); border: 1px solid var(--hairline); border-radius: var(--rounded-xs); }
.preview-table { margin-top: 12px; display: flex; flex-direction: column; gap: 8px; }
.preview-table i { height: 4px; background: var(--surface-3); }
.subscription-info { display: flex; flex-direction: column; align-items: flex-start; gap: 16px; padding: 16px 0; color: var(--ink-muted); }
.reset-settings h3 { font-size: var(--typography-p-card-title-font-size); font-weight: 500; }
.subscription-info p { font-size: var(--typography-p-body-sm-font-size); color: var(--ink-subtle); max-width: 450px; }
.reset-settings { display: flex; gap: 24px; justify-content: space-between; align-items: center; }
@media (max-width: 900px) { .settings-layout { grid-template-columns: 140px minmax(0, 1fr); gap: 20px; } .settings-panel { padding: 24px; } .form-grid { grid-template-columns: 1fr; } }
@media (max-width: 768px) { .settings-layout { display: block; } .category-nav { flex-direction: row; position: static; overflow-x: auto; margin-bottom: 16px; } .category-nav button { white-space: nowrap; flex-shrink: 0; } .settings-panel { padding: 20px 16px; min-height: 380px; } .save-bar { align-self: stretch; justify-content: space-between; gap: 12px; } .save-bar span { max-width: 190px; } .theme-options { gap: 12px; } .theme-preview { height: 120px; } }
@media (max-width: 480px) { .save-bar { bottom: 76px; } }
</style>
