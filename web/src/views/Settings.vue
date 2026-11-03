<template>
  <div class="page-stack settings">
    <div class="page-heading"><h1>{{ appearance.language === 'en' ? 'Settings' : '设置' }}</h1><span v-if="store.savedAt" class="muted type-p-caption">{{ appearance.language === 'en' ? 'Last saved' : '上次保存' }} {{ formatDate(store.savedAt) }}</span></div>
    <el-alert v-if="store.error" :title="store.error" type="warning" :closable="false" show-icon />
    <div class="settings-layout">
      <nav class="category-nav" :aria-label="t('设置分类', 'Settings categories')">
        <button type="button" v-for="item in categories" :key="item.id" :class="{ selected: category === item.id }" :aria-current="category === item.id ? 'page' : undefined" @click="category = item.id"><UiIcon :name="item.icon" :size="16" />{{ item.title }}</button>
      </nav>
      <section v-loading="store.loading" class="panel settings-panel">
        <div class="settings-section-heading"><h2>{{ currentCategory.title }}</h2><p>{{ currentCategory.description }}</p></div>
        <el-form @submit.prevent :model="form" label-position="top" class="settings-form">
          <template v-if="category === 'general'">
            <div class="form-grid">
              <el-form-item :label="t('监听地址', 'Listen address')"><el-input v-model="form.inbound_host" placeholder="127.0.0.1" /><div class="field-hint">{{ t('仅本机使用时保留 127.0.0.1。', 'Keep 127.0.0.1 for local access only.') }}</div></el-form-item>
              <el-form-item :label="t('允许局域网连接', 'Allow LAN connections')"><el-switch v-model="form.allow_lan" /><div class="field-hint">{{ t('其他设备访问时需同时调整监听地址。', 'Adjust the listen address as well to allow access from other devices.') }}</div></el-form-item>
              <el-form-item :label="isLite ? t('透明代理监听端口', 'Transparent proxy listen port') : t('sing-box 混合代理端口', 'sing-box mixed proxy port')"><el-input-number v-model="form.inbound_port" :min="1" :max="65535" /></el-form-item>
              <el-form-item :label="t('mihomo 混合代理端口', 'mihomo mixed proxy port')"><el-input-number v-model="form.mixed_port" :min="1" :max="65535" /></el-form-item>
            </div>
            <el-divider />
            <el-form-item :label="t('仅 SOCKS5 端口使用代理', 'Proxy only through SOCKS5 port')"><el-switch v-model="form.socks_only" :aria-label="t('仅 SOCKS5 端口使用代理', 'Proxy only through SOCKS5 port')" @change="changeSocksOnly" /><div class="field-hint">{{ t('开启后，仅下方独立 SOCKS5 端口按路由规则使用代理；混合端口直连。此模式不能与 TUN 全流量接管同时启用。', 'Only the dedicated SOCKS5 port follows proxy routing rules; mixed ports connect directly. This mode cannot be combined with TUN.') }}</div></el-form-item>
            <el-form-item v-if="form.socks_only" :label="t('独立 SOCKS5 端口', 'Dedicated SOCKS5 port')"><el-input-number v-model="form.socks_port" :min="1" :max="65535" :aria-label="t('独立 SOCKS5 端口', 'Dedicated SOCKS5 port')" /><div class="field-hint">{{ t('请与上方两个混合端口保持不同。SOCKS5 端口会使用当前单节点、负载均衡或代理链。', 'Use a different port from the mixed ports above. SOCKS5 uses the current node, load balancer or proxy chain.') }}</div></el-form-item>
            <el-divider />
            <div class="socks-listeners-heading"><div><h3>{{ t('端口绑定节点', 'Bind nodes to SOCKS5 ports') }}</h3><p class="field-hint">{{ t('每个端口固定走指定节点，不受主代理、负载均衡和路由规则影响，并沿用当前入站监听地址。保存后重新应用配置生效。', 'Each port uses its assigned node directly, independent of the main proxy, load balancing and routing rules, and inherits the current listen address. Reapply the configuration after saving.') }}</p></div><el-button size="small" :disabled="form.socks_listeners.length >= 32" @click="addSOCKSListener"><UiIcon name="plus" :size="16" />{{ t('添加端口', 'Add port') }}</el-button></div>
            <div v-for="(listener, index) in form.socks_listeners" :key="index" class="socks-listener-row">
              <el-input-number v-model="listener.port" :min="1" :max="65535" :aria-label="t('SOCKS5 端口', 'SOCKS5 port')" />
              <el-select v-model="listener.node_id" filterable :placeholder="t('选择固定节点', 'Select a pinned node')" :aria-label="t('端口绑定节点', 'Node for SOCKS5 port')">
                <el-option v-for="node in nodes.items" :key="node.id" :value="node.id" :disabled="Boolean(nodeUnavailableReason(node)) || !supportsSelectedKernel(node)" :label="`${node.name} · ${node.protocol}${supportsSelectedKernel(node) ? '' : t('（不支持当前内核）', ' (unsupported by selected kernel)')}`" />
                <el-option v-if="listener.node_id && !nodes.items.some(node => node.id === listener.node_id)" :value="listener.node_id" :label="t('节点已删除：', 'Missing node: ') + listener.node_id" disabled />
              </el-select>
              <el-button text type="danger" :aria-label="t('删除 SOCKS5 端口', 'Remove SOCKS5 port')" @click="form.socks_listeners.splice(index, 1)">{{ t('删除', 'Remove') }}</el-button>
            </div>
            <p v-if="form.socks_listeners.length === 0" class="field-hint">{{ t('尚未添加端口绑定。', 'No additional port bindings.') }}</p>
            <el-divider />
            <el-form-item :label="t('DNS 服务器', 'DNS servers')"><el-input v-model="dnsText" type="textarea" :rows="4" :placeholder="t('每行一个 DNS 服务器', 'One DNS server per line')" /><div class="field-hint">{{ t('例如 223.5.5.5，每行填写一个地址。', 'For example, 223.5.5.5; enter one address per line.') }}</div></el-form-item>
            <el-form-item label="Fake-IP"><el-switch v-model="form.dns_fake_ip" /><div class="field-hint">{{ t('使用虚拟 IP 映射域名，需配合对应内核的 DNS 配置。', 'Map domains to virtual IP addresses; requires appropriate kernel DNS configuration.') }}</div></el-form-item>
          </template>
          <template v-if="category === 'kernel'">
            <el-form-item :label="t('首选内核', 'Preferred kernel')"><el-radio-group v-model="form.kernel"><el-radio-button value="sing-box" :disabled="isLite">sing-box{{ isLite ? t('（精简版不支持）', ' (unavailable in lite)') : '' }}</el-radio-button><el-radio-button value="mihomo">mihomo</el-radio-button></el-radio-group><div class="field-hint">{{ t('节点仅支持另一内核时，会按协议能力选择。', 'When a node supports only the other kernel, its protocol capabilities determine the choice.') }}</div></el-form-item>
            <el-divider />
            <el-form-item :label="t('透明代理模式', 'Transparent proxy mode')">
              <el-select v-model="form.transparent_mode" @change="changeTransparentMode">
                <el-option v-for="mode in transparentModes" :key="mode.mode" :value="mode.mode" :label="transparentModeLabel(mode.mode)" :disabled="!mode.supported"><span>{{ transparentModeLabel(mode.mode) }}</span><small v-if="!mode.supported">{{ translateCapability(mode.reason) }}</small></el-option>
              </el-select>
              <div class="field-hint">{{ translateCapability(selectedTransparentMode?.prerequisites) }}</div>
              <div class="field-hint">DNS: {{ translateCapability(selectedTransparentMode?.dns_description) }}</div>
              <div class="field-hint">{{ t('当前生效：', 'Currently active: ') }}{{ transparentModeLabel(form.transparent_active ?? 'off') }}</div>
            </el-form-item>
            <el-form-item :label="t('TUN 全流量接管', 'TUN system traffic capture')"><el-switch v-model="form.enable_tun" :disabled="!transparentModes.find((mode) => mode.mode === 'tun')?.supported" :aria-label="t('TUN 全流量接管', 'TUN system traffic capture')" @change="changeTun" /><div class="field-hint">{{ t('接管 IPv4/IPv6 系统流量并劫持 TCP/UDP DNS；启用严格路由防止流量回落直连。需要管理员或 NET_ADMIN 权限及可用的 TUN 设备。路由规则仍决定哪些流量直连。', 'Captures IPv4/IPv6 system traffic and intercepts TCP/UDP DNS; strict routing prevents fallback to direct connections. Requires administrator or NET_ADMIN privileges and a TUN device. Routing rules still determine direct traffic.') }}</div></el-form-item>
            <el-form-item :label="t('TUN 网络栈', 'TUN network stack')"><el-select v-model="form.tun_stack" :disabled="!form.enable_tun"><el-option v-for="stack in ['system', 'gvisor', 'mixed']" :key="stack" :value="stack" :label="stack" /></el-select></el-form-item>
            <el-divider />
            <el-form-item :label="t('系统代理', 'System proxy')"><el-switch v-model="form.system_proxy" :disabled="!form.system_proxy_supported" /><div class="field-hint">{{ form.system_proxy_supported ? t('将系统代理指向本地入口，退出或启动失败时恢复原设置。只接管遵循系统代理设置的应用；要接管设备的全部流量，请启用 TUN。', 'Points the system proxy to the local inbound and restores settings on exit or startup failure. Only applications using system proxy settings are affected; enable TUN for all device traffic.') : t('当前环境不支持系统代理接管；Windows 或带 GNOME D-Bus 会话的 Linux 可用。要接管设备的全部流量，请启用 TUN。', 'System proxy is unavailable in this environment. It is available on Windows or Linux with a GNOME D-Bus session. Enable TUN for all device traffic.') }}</div></el-form-item>
            <router-link to="/kernel" class="settings-link">{{ t('查看运行状态与日志 →', 'View status and logs →') }}</router-link>
          </template>
          <template v-if="category === 'routing'">
            <el-form-item :label="t('路由分组', 'Routing preset')"><el-radio-group v-model="form.routing_mode" @change="applyRoutingMode"><el-radio-button v-for="mode in routingModes" :key="mode.value" :value="mode.value">{{ mode.label }}</el-radio-button></el-radio-group><div class="field-hint">{{ currentModeDescription }}</div></el-form-item>
            <el-form-item :label="t('未匹配流量出口', 'Default outbound')"><el-select v-model="form.default_outbound"><el-option v-for="outbound in outboundOptions" :key="outbound" :label="outbound === 'PROXY' ? t('使用当前选择的代理节点', 'Use selected proxy node') : outbound === 'DIRECT' ? t('直接连接（DIRECT）', 'Direct connection (DIRECT)') : outbound === 'REJECT' ? t('阻断连接（REJECT）', 'Block connection (REJECT)') : outbound" :value="outbound" /></el-select><div class="field-hint">{{ t('流量未命中下方规则时使用此出口。代理出口会跟随当前启用的节点。', 'Used when traffic matches none of the rules below. Proxy outbound follows the active node.') }}</div></el-form-item>
            <div class="rules-heading"><span class="type-p-card-title">{{ t('自定义规则', 'Custom rules') }} <span class="muted mono">{{ routingForms.length }}</span></span><el-button size="small" @click="openAddRule"><UiIcon name="plus" :size="16" />{{ t('添加规则', 'Add rule') }}</el-button></div>
            <p class="field-hint rule-hint">{{ t('从上到下匹配。切换分组会保留自定义规则；大陆分流范围取决于已配置的域名与 IP 规则。', 'Rules are matched from top to bottom. Custom rules are retained when switching presets; mainland bypass depends on configured domain and IP rules.') }}</p>
            <el-radio-group v-model="routingEditorMode" size="small" @change="syncRoutingTextFromList"><el-radio-button value="list">{{ t('列表', 'List') }}</el-radio-button><el-radio-button value="text">{{ t('文本', 'Text') }}</el-radio-button></el-radio-group>
            <template v-if="routingEditorMode === 'text'">
              <div class="routing-text-wrap"><div class="line-numbers"><span v-for="line in routingTextLineCount" :key="line">{{ line }}</span></div><el-input v-model="routingText" type="textarea" :rows="14" class="routing-text" @input="validateRoutingEditor" /></div>
              <div v-for="error in routingTextErrors" :key="`${error.line}-${error.reason}`" class="routing-error">{{ error.line ? (appearance.language === 'en' ? `Line ${error.line}: ` : `第 ${error.line} 行：`) : '' }}{{ error.reason }}</div>
              <div class="routing-actions"><el-button size="small" @click="applyRoutingText">{{ t('回填列表', 'Apply to list') }}</el-button><el-button size="small" @click="downloadRoutingText">{{ t('导出文本', 'Export text') }}</el-button><el-button size="small" @click="chooseRoutingFile">{{ t('导入文本', 'Import text') }}</el-button><input ref="routingFileInput" type="file" accept=".txt,text/plain" hidden @change="readRoutingFile" /></div>
            </template>
            <div class="table-scroll"><el-table :data="routingForms" class="rules-table" size="small" :empty-text="t('暂无自定义规则', 'No custom rules')">
              <el-table-column type="index" label="#" width="44" />
              <el-table-column :label="t('匹配类型', 'Match type')" width="116"><template #default="{ row }">{{ matchLabel(row.match) }}</template></el-table-column>
              <el-table-column :label="t('来源', 'Source')" width="90"><template #default="{ row }">{{ row.source }}</template></el-table-column>
              <el-table-column :label="t('域名 / IP / 规则值', 'Domain / IP / rule value')" min-width="200"><template #default="{ row, $index }"><button type="button" class="rule-summary" :title="row.valuesText" @click="openEditRule($index)">{{ valueSummary(row) }}</button></template></el-table-column>
              <el-table-column :label="t('出口', 'Outbound')" width="100"><template #default="{ row }"><span class="neutral-badge">{{ row.outbound }}</span></template></el-table-column>
              <el-table-column :label="t('启用', 'Enabled')" width="70"><template #default="{ row }"><el-switch v-model="row.enabled" /></template></el-table-column>
              <el-table-column :label="t('操作', 'Actions')" width="150"><template #default="{ $index }"><el-button link :aria-label="t('上移规则', 'Move rule up')" :disabled="$index === 0" @click="moveRoutingRule($index, -1)"><UiIcon name="up" :size="16" /></el-button><el-button link :aria-label="t('下移规则', 'Move rule down')" :disabled="$index === routingForms.length - 1" @click="moveRoutingRule($index, 1)"><UiIcon name="down" :size="16" /></el-button><el-button link :aria-label="t('编辑规则', 'Edit rule')" @click="openEditRule($index)"><UiIcon name="edit" :size="16" /></el-button><el-button link :aria-label="t('删除规则', 'Delete rule')" @click="removeRoutingRule($index)"><UiIcon name="trash" :size="16" /></el-button></template></el-table-column>
            </el-table></div>
          </template>
          <template v-if="category === 'subscription'">
            <div class="subscription-info"><p>{{ t('更新周期与 User-Agent 在各订阅中设置。', 'Set the update interval and User-Agent for each subscription.') }}</p><router-link to="/subscriptions">{{ t('打开订阅管理', 'Open subscriptions') }}</router-link></div>
          </template>
          <template v-if="category === 'balance'"><LoadBalanceFields v-model="form.load_balance" /><p class="field-hint">{{ t('保存为负载均衡弹窗的默认值。已有运行实例保持本次启动时的设置，重新启动负载均衡后应用新设置。', 'Save defaults for the load balancing dialog. Running instances retain their current settings until restarted.') }}</p><router-link to="/nodes" class="settings-link">{{ t('前往节点管理，选择多个节点并启动 →', 'Go to nodes to select and start a group →') }}</router-link></template>
          <template v-if="category === 'appearance'">
            <el-form-item :label="appearance.language === 'en' ? 'Language' : '语言'">
              <el-radio-group :model-value="appearance.language" @update:model-value="appearance.setLanguage($event as 'zh-CN' | 'en')">
                <el-radio-button value="zh-CN">简体中文</el-radio-button>
                <el-radio-button value="en">English</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <p class="field-hint appearance-hint">{{ appearance.language === 'en' ? 'Theme changes apply immediately and are saved in this browser.' : '主题立即应用，并保存在当前浏览器中。' }}</p>
            <div class="theme-options">
              <button type="button" v-for="theme in (['light', 'dark'] as const)" :key="theme" class="theme-option" :class="{ chosen: appearance.theme === theme }" :aria-pressed="appearance.theme === theme" @click="appearance.setTheme(theme)">
                <div class="theme-preview" :data-theme="theme"><div class="preview-sidebar"><i /><i /><i /></div><div class="preview-main"><div class="preview-top" /><div class="preview-cards"><i /><i /></div><div class="preview-table"><i /><i /><i /></div></div></div>
                <span>{{ appearance.language === 'en' ? (theme === 'light' ? 'Light' : 'Dark') : (theme === 'light' ? '浅色' : '深色') }}<UiIcon v-if="appearance.theme === theme" name="check" :size="16" /></span>
              </button>
            </div>
          </template>
          <template v-if="category === 'about'">
            <p>DuoRelay {{ buildInfo.version || 'dev' }} · {{ variant }}{{ isLite ? t('（精简版 · mihomo 单核）', ' (lite · mihomo only)') : t('（完整版 · 双核）', ' (full · dual kernel)') }}</p>
            <p class="field-hint">{{ t('构建', 'Build') }} {{ buildInfo.commit || 'unknown' }}</p>
            <el-divider />
            <div class="update-panel">
              <div class="update-heading"><div><h3>{{ t('软件更新', 'Software updates') }}</h3><p class="field-hint">{{ updateInfo ? t('当前版本：', 'Current version: ') + updateInfo.current_version : t('检查 GitHub 上适用于当前平台和版本的正式发布。', 'Check GitHub for a stable release matching this platform and build variant.') }}</p></div><el-button :loading="updateLoading" :disabled="updateApplying" @click="checkForUpdates">{{ updateLoading ? t('检查中…', 'Checking…') : t('检查更新', 'Check for updates') }}</el-button></div>
              <el-alert v-if="updateError" :title="updateError" type="error" :closable="false" show-icon />
              <template v-if="updateInfo">
                <div v-if="updateInfo.update_available" class="update-release">
                  <strong>{{ updateInfo.release_name || ('DuoRelay ' + updateInfo.latest_version) }}</strong>
                  <span class="muted">{{ t('最新版本：', 'Latest version: ') }}{{ updateInfo.latest_version }}<template v-if="updateInfo.published_at"> · {{ updateInfo.published_at.slice(0, 10) }}</template></span>
                  <p v-if="updateInfo.reason" class="field-hint">{{ updateReason(updateInfo.reason) }}</p>
                  <pre v-if="updateInfo.release_notes" class="release-notes">{{ updateInfo.release_notes }}</pre>
                  <div class="update-actions"><el-button v-if="updateInfo.release_url" link type="primary" tag="a" :href="updateInfo.release_url" target="_blank" rel="noopener noreferrer">{{ t('查看发布说明', 'View release') }}</el-button><el-button type="primary" :disabled="!updateInfo.can_update || updateApplying" :loading="updateApplying" @click="applyUpdate">{{ updateApplying ? t('正在准备更新…', 'Preparing update…') : t('一键更新并重启', 'Update and restart') }}</el-button></div>
                </div>
                <el-alert v-else :title="t('当前已是最新版本', 'You are up to date')" type="success" :closable="false" show-icon />
              </template>
            </div>
          </template>
          <template v-if="category === 'advanced'">
            <el-form-item :label="t('日志级别', 'Log level')"><el-select v-model="form.log_level"><el-option v-for="level in ['debug', 'info', 'warn', 'error']" :key="level" :value="level" :label="level" /></el-select><div class="field-hint">{{ t('日常使用推荐 info；排查问题时选择 debug。', 'Use info for normal operation and debug for troubleshooting.') }}</div></el-form-item>
            <el-divider /><div class="reset-settings"><div><h3>{{ t('恢复默认配置', 'Restore defaults') }}</h3><p class="field-hint">{{ t('先填入默认值，保存后才会覆盖服务器配置。', 'Defaults are filled in first; saving overwrites the server configuration.') }}</p></div><el-button @click="handleReset">{{ t('填入默认值', 'Fill defaults') }}</el-button></div>
          </template>
        </el-form>
      </section>
    </div>
    <div v-if="category !== 'appearance' && category !== 'subscription' && category !== 'about'" class="save-bar"><span>{{ t('修改保存后，重新选择并启动节点以应用。', 'After saving, select and start the node again to apply changes.') }}</span><el-button type="primary" :disabled="store.loading" :loading="store.saving" @click="handleSave">{{ t('保存设置', 'Save settings') }}</el-button></div>
    <el-dialog v-model="ruleDialogVisible" :title="editingIndex < 0 ? t('添加路由规则', 'Add routing rule') : t('编辑路由规则', 'Edit routing rule')" width="560px">
      <el-form @submit.prevent label-position="top">
        <el-form-item :label="t('来源', 'Source')"><el-select v-model="editingRule.source" :disabled="editingRule.source !== 'user'"><el-option :label="t('用户规则', 'User rule')" value="user" /><el-option :label="t('绕过规则', 'Bypass rule')" value="bypass" /><el-option :label="t('内部路由', 'Internal route')" value="internal" /></el-select></el-form-item>
        <el-form-item :label="t('匹配类型', 'Match type')"><el-select v-model="editingRule.match"><el-option v-for="option in routingMatchOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
        <el-form-item :label="t('域名 / IP / 规则值', 'Domain / IP / rule value')"><el-input v-model="editingRule.valuesText" type="textarea" :rows="8" :placeholder="t('每行填写一条规则', 'One rule per line')" class="rule-input" /><div class="field-hint">{{ t('内容需要与匹配类型一致，例如域名后缀 example.com，或 CIDR 192.168.1.0/24。', 'Values must match the selected type, for example domain suffix example.com or CIDR 192.168.1.0/24.') }}</div></el-form-item>
        <el-form-item :label="t('流量出口', 'Outbound')"><el-select v-model="editingRule.outbound" filterable><el-option v-for="outbound in outboundOptions" :key="outbound" :label="outbound" :value="outbound" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="ruleDialogVisible = false">{{ t('取消', 'Cancel') }}</el-button><el-button type="primary" @click="saveRule">{{ t('确定', 'Confirm') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { defaultConfig, useConfigStore } from '@/stores/config'
import { useNodesStore } from '@/stores/nodes'
import { isLite, nodeUnavailableReason, variant } from '@/stores/variant'
import { ApiError, request } from '@/api/client'
import { useAppearanceStore } from '@/stores/appearance'
import UiIcon from '@/components/UiIcon.vue'
import LoadBalanceFields from '@/components/LoadBalanceFields.vue'
import { formatDate } from '@/utils/format'
import type { AppConfig, ProxyNode, RoutingMatch, RoutingMode, RoutingRule, UpdateInfo, UpdateProgress } from '@/types'
import { exportRouting, importRoutingText, validateRoutingText } from '@/api/config'

const store = useConfigStore()
const nodes = useNodesStore()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const buildInfo = ref<{ version?: string; commit?: string }>({})
void request<{ version?: string; commit?: string }>({ url: '/build' }).then(info => { buildInfo.value = info ?? {} }).catch(() => {})
const updateLoading = ref(false)
const updateApplying = ref(false)
const updateInfo = ref<UpdateInfo | null>(null)
const updateError = ref('')
let updatePollTimer: number | undefined
let updateRestartingSeen = false
function updateReason(reason: string) {
  if (appearance.language !== 'en') return reason
  const messages: Record<string, string> = {
    '当前平台和 full 变体没有匹配的发布包': 'No release package matches this platform and full build.',
    '当前平台和 lite 变体没有匹配的发布包': 'No release package matches this platform and lite build.',
    '当前程序目录不可写，请使用有权限的更新方式': 'The install directory is not writable. Use an update method with the required permissions.',
    '无法定位当前程序': 'Could not locate the current executable.',
    '无法解析当前程序安装路径': 'Could not resolve the current executable path.',
    '容器部署请通过更新镜像升级，不能在容器内替换程序': 'Update the container image; the executable cannot be updated inside the container.'
  }
  return messages[reason] ?? reason
}
async function checkForUpdates() {
  updateLoading.value = true
  updateError.value = ''
  try { updateInfo.value = await request<UpdateInfo>({ url: '/update/check', timeout: 35000 }) }
  catch (err) { updateError.value = err instanceof ApiError && err.why ? `${err.message}: ${err.why}` : err instanceof Error ? err.message : t('检查更新失败，请稍后重试', 'Could not check for updates; try again later') }
  finally { updateLoading.value = false }
}
async function applyUpdate() {
  if (!updateInfo.value?.can_update || !updateInfo.value.update_available) return
  try {
    await ElMessageBox.confirm(t('更新会停止并重启 DuoRelay 服务。', 'Updating will stop and restart the DuoRelay service.'), t('确认更新', 'Confirm update'), { type: 'warning', confirmButtonText: t('更新并重启', 'Update and restart'), cancelButtonText: t('取消', 'Cancel') })
    updateApplying.value = true
    const result = await request<UpdateProgress>({ url: '/update/apply', method: 'post' })
    if (result.state === 'failed') throw new Error(result.message || t('更新失败', 'Update failed'))
    ElMessage.success(result.message || t('更新已开始，服务正在重启', 'Update started; the service is restarting'))
    if (result.state === 'downloading') {
      updateRestartingSeen = false
      updatePollTimer = window.setInterval(async () => {
        try {
          const progress = await request<UpdateProgress>({ url: '/update/status' })
          if (progress.state === 'failed') {
            if (updatePollTimer) window.clearInterval(updatePollTimer)
            updatePollTimer = undefined
            updateApplying.value = false
            ElMessage.error(progress.message || t('更新失败', 'Update failed'))
          } else if (progress.state === 'restarting') {
            updateRestartingSeen = true
          } else if (progress.state === 'idle' && updateRestartingSeen) {
            if (updatePollTimer) window.clearInterval(updatePollTimer)
            updatePollTimer = undefined
            window.location.reload()
          }
        } catch {
          // Keep polling while the service manager restarts the updated process.
        }
      }, 1000)
    }
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') ElMessage.error(err instanceof ApiError && err.why ? `${err.message}: ${err.why}` : err instanceof Error ? err.message : t('更新失败', 'Update failed'))
    updateApplying.value = false
  }
}
onBeforeUnmount(() => { if (updatePollTimer) window.clearInterval(updatePollTimer) })
const category = ref('general')
const categories = computed(() => appearance.language === 'en' ? [
  { id: 'general', title: 'General', icon: 'settings', description: 'Listen address, ports and DNS' },
  { id: 'kernel', title: 'Kernel', icon: 'terminal', description: 'Kernel preference and TUN' },
  { id: 'routing', title: 'Routing', icon: 'nodes', description: 'Traffic routing and rules' },
  { id: 'balance', title: 'Load balancing', icon: 'nodes', description: 'Distribution and health check defaults' },
  { id: 'subscription', title: 'Subscriptions', icon: 'link', description: 'Subscription updates and imports' },
  { id: 'appearance', title: 'Appearance', icon: 'sun', description: 'Language and theme' },
  { id: 'about', title: 'About', icon: 'grid', description: 'Version and build variant' },
  { id: 'advanced', title: 'Advanced', icon: 'grid', description: 'Logs and configuration reset' }
] : [
  { id: 'general', title: '通用', icon: 'settings', description: '监听地址、端口与 DNS' },
  { id: 'kernel', title: '内核', icon: 'terminal', description: '双内核偏好与 TUN' },
  { id: 'routing', title: '路由规则', icon: 'nodes', description: '流量分流与规则管理' },
  { id: 'balance', title: '负载均衡', icon: 'nodes', description: '多出口分配策略与健康检查默认值' },
  { id: 'subscription', title: '订阅', icon: 'link', description: '订阅更新与内容导入' },
  { id: 'appearance', title: '外观', icon: 'sun', description: '浅色与深色主题' },
  { id: 'about', title: '关于', icon: 'grid', description: '版本与构建变体' },
  { id: 'advanced', title: '高级', icon: 'grid', description: '日志与配置恢复' }
])
const currentCategory = computed(() => categories.value.find(item => item.id === category.value)!)


const form = reactive<AppConfig>({ ...store.config, socks_listeners: (store.config.socks_listeners ?? []).map(listener => ({ ...listener })), load_balance: { ...store.config.load_balance } })
if (!form.socks_listeners) form.socks_listeners = []
function supportsSelectedKernel(node: ProxyNode) {
  return form.kernel === 'mihomo' ? Boolean(node.owner & 2) : Boolean(node.owner & 1)
}
const availableSOCKSNodes = computed(() => nodes.items.filter(node => {
  if (nodeUnavailableReason(node)) return false
  return supportsSelectedKernel(node)
}))
function addSOCKSListener() {
  if (form.socks_listeners.length >= 32) return
  const occupied = new Set([form.inbound_port, form.mixed_port, ...(form.socks_only ? [form.socks_port] : []), ...form.socks_listeners.map(item => item.port)])
  let port = 10809
  while (port <= 65535 && occupied.has(port)) port++
  if (port > 65535) { ElMessage.warning(t('没有可用端口', 'No available port')); return }
  form.socks_listeners.push({ port, node_id: availableSOCKSNodes.value[0]?.id ?? '' })
}
const dnsText = ref(store.config.dns_servers.join('\n'))
const routingModes = computed<Array<{ label: string; value: RoutingMode; description: string }>>(() => [
  { label: t('绕过大陆', 'Bypass mainland'), value: 'bypass_mainland', description: t('中国大陆及局域网直连，其余流量使用代理。', 'Connect directly to mainland China and LAN; proxy other traffic.') },
  { label: t('全局', 'Global'), value: 'global', description: t('所有未命中自定义规则的流量使用代理。', 'Proxy all traffic not matched by custom rules.') },
  { label: t('黑名单', 'Blocklist'), value: 'blacklist', description: t('默认直连，仅把自定义规则命中的流量送入代理。', 'Connect directly by default; proxy traffic matched by custom rules.') }
])
const routingMatchOptions = computed<Array<{ label: string; value: RoutingMatch }>>(() => [
  { label: t('域名', 'Domain'), value: 'domain' },
  { label: t('域名后缀', 'Domain suffix'), value: 'domain_suffix' },
  { label: t('域名关键词', 'Domain keyword'), value: 'domain_keyword' },
  { label: t('目标 CIDR', 'Destination CIDR'), value: 'ip_cidr' },
  { label: t('来源 CIDR', 'Source CIDR'), value: 'source_ip_cidr' },
  { label: t('目标端口', 'Destination port'), value: 'port' },
  { label: t('网络类型', 'Network type'), value: 'network' },
  { label: t('协议', 'Protocol'), value: 'protocol' },
  { label: 'GeoSite', value: 'geosite' },
  { label: 'GeoIP', value: 'geoip' }
])

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
  () => routingModes.value.find((mode) => mode.value === form.routing_mode)?.description ?? ''
)
const fallbackTransparentModes = [
  { mode: 'off', supported: true, prerequisites: '无需系统权限', dns_description: '不接管 DNS。' },
  { mode: 'redirect', supported: false, reason: '当前系统未报告可用能力', prerequisites: '需要 Linux nftables/iptables、iproute2 和 NET_ADMIN。', dns_description: '不接管 UDP DNS。' },
  { mode: 'tproxy', supported: false, reason: '当前系统未报告可用能力', prerequisites: '需要 Linux TPROXY、策略路由和 NET_ADMIN。', dns_description: '需要显式配置代理 DNS。' },
  { mode: 'tun', supported: false, reason: '当前系统未报告可用能力', prerequisites: '需要 /dev/net/tun 和 NET_ADMIN。', dns_description: '由内核 DNS hijack 接管。' }
] as AppConfig['transparent_modes']
const transparentModes = computed(() => (store.config.transparent_modes?.length ? store.config.transparent_modes : fallbackTransparentModes) as NonNullable<AppConfig['transparent_modes']>)
const selectedTransparentMode = computed(() => transparentModes.value.find((mode) => mode.mode === form.transparent_mode))
function transparentModeLabel(mode: string) { return ({ off: t('关闭', 'Off'), redirect: 'Linux redirect', tproxy: 'Linux TPROXY', tun: t('TUN 全流量', 'TUN system traffic') } as Record<string, string>)[mode] ?? mode }
function translateCapability(value?: string) {
  if (!value || appearance.language !== 'en') return value ?? ''
  const translations: Record<string, string> = {
    '无需系统权限': 'No system privileges required',
    '不接管 DNS。': 'Does not capture DNS.',
    '不接管 DNS；按应用自身代理设置运行。': 'Does not capture DNS; applications use their own proxy settings.',
    '首次使用需管理员权限、TUN 驱动和防火墙确认': 'First use requires administrator privileges, a TUN driver and firewall confirmation.',
    '由内核 TUN DNS hijack 接管；需确认防火墙与驱动。': 'Captured by kernel TUN DNS hijacking; confirm the firewall and driver.',
    'Windows 没有 Linux redirect 透明代理链': 'Linux redirect transparent proxy chains are unavailable on Windows.',
    'Windows 没有 Linux TPROXY 策略路由': 'Linux TPROXY policy routing is unavailable on Windows.',
    '不适用': 'Not applicable', '不适用。': 'Not applicable.',
    'macOS TUN 尚未完成真实系统验收': 'TUN has not been verified on a real macOS system.',
    '需在 macOS 真实系统验证': 'Requires verification on a real macOS system.',
    '尚未验证，不能声明已接管。': 'Not verified; capture cannot be claimed.',
    '仅 Linux 支持此模式': 'This mode is supported only on Linux.',
    '缺少 nftables/iptables 或 iproute2': 'Missing nftables/iptables or iproute2.',
    '/dev/net/tun 不可用；服务需 DeviceAllow 或容器需显式映射': '/dev/net/tun is unavailable; the service needs DeviceAllow or the container must explicitly map it.',
    '仅需 CAP_NET_ADMIN 与 CAP_NET_RAW，并可读写 /dev/net/tun；内核负责 TUN 路由和 DNS hijack。': 'Requires CAP_NET_ADMIN, CAP_NET_RAW and read/write access to /dev/net/tun; the kernel handles TUN routing and DNS hijacking.',
    'TUN 通过内核 DNS hijack 接管 TCP/UDP 53；内部 DNS 分流规则先于用户规则，避免 DNS 泄漏。': 'TUN intercepts TCP/UDP port 53 using kernel DNS hijacking; internal DNS routing rules precede user rules to prevent DNS leaks.',
    '当前系统未报告可用能力': 'The current system did not report this capability.',
    '需要 Linux nftables/iptables、iproute2 和 NET_ADMIN。': 'Requires Linux nftables/iptables, iproute2 and NET_ADMIN.',
    '不接管 UDP DNS。': 'Does not capture UDP DNS.',
    '需要 Linux TPROXY、策略路由和 NET_ADMIN。': 'Requires Linux TPROXY, policy routing and NET_ADMIN.',
    '需要显式配置代理 DNS。': 'Proxy DNS must be configured explicitly.',
    '需要 /dev/net/tun 和 NET_ADMIN。': 'Requires /dev/net/tun and NET_ADMIN.',
    '由内核 DNS hijack 接管。': 'Captured by kernel DNS hijacking.',
    'redirect 不接管 UDP DNS；使用系统 DNS 会产生 DNS 泄漏，面板会明确标记。': 'Redirect does not capture UDP DNS. Using system DNS causes a DNS leak, which the panel marks explicitly.',
    'TPROXY 可接管 TCP/UDP，但 DuoRelay 当前只建立代理流量链，DNS 仍需显式配置到代理 DNS，避免把系统 DNS 当成已接管。': 'TPROXY can capture TCP/UDP, but DuoRelay currently creates only a proxy traffic chain. Configure proxy DNS explicitly; system DNS is not automatically captured.'
  }
  if (translations[value]) return translations[value]
  const base = '需要 root 或 CAP_NET_ADMIN/CAP_NET_RAW；iproute2；'
  if (value.startsWith(base)) {
    return value.replace(base, 'Requires root or CAP_NET_ADMIN/CAP_NET_RAW; iproute2; ')
      .replace('nftables 或 iptables', 'nftables or iptables')
      .replace('redirect 只接管 TCP；DNS 不自动劫持，需使用内核 DNS 配置或系统明确指向代理 DNS。', 'redirect captures TCP only. DNS is not intercepted automatically; configure kernel DNS or point system DNS explicitly to proxy DNS.')
      .replace('内核需支持 TPROXY、fwmark 和策略路由。', 'The kernel must support TPROXY, fwmark and policy routing.')
  }
  return value
}

function chinaRules(): RoutingForm[] {
  return [
    { match: 'domain_suffix', valuesText: 'cn\n中国\nlocal', outbound: 'DIRECT' },
    { match: 'ip_cidr', valuesText: '10.0.0.0/8\n100.64.0.0/10\n172.16.0.0/12\n192.168.0.0/16', outbound: 'DIRECT' }
  ]
}

function syncFromStore(config: AppConfig) {
  Object.assign(form, config, { ...(isLite.value ? { kernel: 'mihomo' } : {}), socks_listeners: (config.socks_listeners ?? []).map(listener => ({ ...listener })), routing_mode: config.routing_mode || 'bypass_mainland', load_balance: { ...config.load_balance } })
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
    ElMessage.warning(t('请至少填写一条域名或 IP 规则', 'Enter at least one domain or IP rule'))
    return
  }
  if (!outbound) {
    ElMessage.warning(t('请选择流量出口', 'Select an outbound'))
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
  return routingMatchOptions.value.find((option) => option.value === match)?.label ?? match
}

function valueSummary(rule: RoutingForm): string {
  const values = parseValues(rule.valuesText)
  return appearance.language === 'en' ? (values.length > 2 ? `${values.slice(0, 2).join(', ')} and ${values.length - 2} more` : values.join(', ')) : (values.length > 2 ? `${values.slice(0, 2).join('、')} 等 ${values.length} 项` : values.join('、'))
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
  try { routingTextErrors.value = (await validateRoutingText(routingText.value)).errors ?? [] } catch (err) { routingTextErrors.value = [{ line: 0, reason: err instanceof Error ? err.message : t('文本校验失败', 'Text validation failed') }] }
}

async function applyRoutingText() {
  const result = await importRoutingText(routingText.value)
  routingTextErrors.value = result.errors ?? []
  if (routingTextErrors.value.length) { ElMessage.warning(t('文本存在错误，非法行未回填', 'Text contains errors; invalid lines were not applied')); return }
  routingForms.value = result.rules.map((rule) => ({ id: rule.id, source: rule.source ?? 'user', enabled: rule.enabled !== false, match: rule.match, valuesText: rule.values.join('\n'), outbound: rule.outbound }))
  ElMessage.success(t('文本已回填列表', 'Text applied to the list'))
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
    ElMessage.warning(t('独立 SOCKS5 端口不能与混合代理端口相同', 'The dedicated SOCKS5 port must differ from the mixed proxy ports'))
    return
  }
  const listenerPorts = new Set<number>()
  for (const listener of form.socks_listeners) {
    if (!Number.isInteger(listener.port) || listener.port < 1 || listener.port > 65535 || !listener.node_id || listenerPorts.has(listener.port) || [form.inbound_port, form.mixed_port, ...(form.socks_only ? [form.socks_port] : []), ...(form.transparent_mode === 'tproxy' ? [form.inbound_port + 1] : [])].includes(listener.port)) {
      ElMessage.warning(t('请检查 SOCKS5 端口：端口需有效、不重复、不冲突且已分配节点', 'Check SOCKS5 ports: each must be valid, unique, conflict-free and assigned to a node'))
      return
    }
    listenerPorts.add(listener.port)
  }
  const servers = dnsText.value.split('\n').map((line) => line.trim()).filter(Boolean)
  if (servers.length === 0) {
    ElMessage.warning(t('请至少配置一个 DNS 服务器', 'Configure at least one DNS server'))
    return
  }
  const rules = routingRules()
  if (rules.some((rule) => rule.values.length === 0 || !rule.outbound)) {
    ElMessage.warning(t('请完善路由规则后再保存', 'Complete the routing rules before saving'))
    return
  }
  if (routingEditorMode.value === 'text') {
    await validateRoutingEditor()
    if (routingTextErrors.value.length) { ElMessage.warning(t('请先修复文本规则错误', 'Fix routing text errors first')); return }
  }
  try {
    await store.save({ ...form, socks_listeners: form.socks_listeners.map(listener => ({ ...listener })), dns_servers: servers, routing_rules: rules })
    ElMessage.success(t('设置已保存；重新选择并启动节点后应用到内核', 'Settings saved; select and start a node again to apply them'))
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : t('保存设置失败', 'Failed to save settings'))
  }
}

function handleReset() {
  syncFromStore(defaultConfig())
  ElMessage.info(t('已恢复默认值，点击保存后生效', 'Defaults restored; save to apply them'))
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
.socks-listeners-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin: 8px 0 16px; }
.socks-listeners-heading h3 { font-size: var(--typography-p-card-title-font-size); font-weight: 500; }
.socks-listener-row { display: flex; align-items: center; gap: 12px; margin: 12px 0; flex-wrap: wrap; }
.socks-listener-row .el-select { flex: 1; min-width: 220px; }
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
.update-panel { display: flex; flex-direction: column; gap: 16px; max-width: 720px; }
.update-heading, .update-actions { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.update-heading h3 { font-size: var(--typography-p-card-title-font-size); font-weight: 500; }
.update-release { display: flex; flex-direction: column; gap: 8px; padding: 16px; border: 1px solid var(--hairline); border-radius: var(--rounded-md); background: var(--surface-2); }
.release-notes { max-height: 220px; overflow: auto; white-space: pre-wrap; color: var(--ink-subtle); font: var(--typography-p-caption-font-size)/1.6 var(--font-mono); }
@media (max-width: 900px) { .settings-layout { grid-template-columns: 140px minmax(0, 1fr); gap: 20px; } .settings-panel { padding: 24px; } .form-grid { grid-template-columns: 1fr; } }
@media (max-width: 768px) { .settings-layout { display: block; } .category-nav { flex-direction: row; position: static; overflow-x: auto; margin-bottom: 16px; } .category-nav button { white-space: nowrap; flex-shrink: 0; } .settings-panel { padding: 20px 16px; min-height: 380px; } .save-bar { align-self: stretch; justify-content: space-between; gap: 12px; } .save-bar span { max-width: 190px; } .theme-options { gap: 12px; } .theme-preview { height: 120px; } }
@media (max-width: 480px) { .save-bar { bottom: 76px; } }
</style>
