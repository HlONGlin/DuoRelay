<template>
  <el-dialog :model-value="modelValue" :title="t('创建节点', 'Create node')" width="680px" destroy-on-close class="node-create-dialog" @update:model-value="emit('update:modelValue', $event)">
    <div class="protocol-families" role="tablist" :aria-label="t('协议分类', 'Protocol families')">
      <button
        v-for="category in categories"
        :key="category.id"
        type="button"
        role="tab"
        :aria-selected="activeCategory === category.id"
        :class="{ selected: activeCategory === category.id }"
        @click="selectCategory(category.id)"
      >{{ t(category.label, category.labelEn) }}</button>
    </div>

    <el-form label-position="top" class="node-create-form" @submit.prevent="save">
      <el-form-item :label="t('协议', 'Protocol')">
        <el-select v-model="protocol" class="full-width" @change="selectProtocol">
          <el-option v-for="item in currentProtocols" :key="item" :label="protocolLabel(item)" :value="item" />
        </el-select>
      </el-form-item>

      <div class="form-grid">
        <el-form-item :label="t('节点名称', 'Name')" required><el-input v-model="name" maxlength="256" :placeholder="t('例如：香港 01', 'e.g. Hong Kong 01')" /></el-form-item>
        <el-form-item v-if="requiresServer" :label="t('服务器地址', 'Host / IP')" required><el-input v-model="host" autocomplete="off" /></el-form-item>
        <el-form-item v-if="requiresServer" :label="t('端口', 'Port')" required><el-input-number v-model="port" :min="1" :max="65535" controls-position="right" class="full-width" /></el-form-item>
      </div>

      <template v-if="activeFields.length">
        <div class="section-heading">{{ t('认证与协议参数', 'Authentication and protocol settings') }}</div>
        <div class="form-grid">
          <el-form-item v-for="field in activeFields" :key="field.key" :label="t(field.label, field.labelEn)" :required="field.required" :class="{ 'span-two': field.wide }">
            <el-select v-if="field.type === 'select'" v-model="proto[field.key]" class="full-width">
              <el-option v-for="option in field.options" :key="option" :label="option" :value="option" />
            </el-select>
            <el-input-number v-else-if="field.type === 'number'" v-model="proto[field.key]" :min="field.min ?? 0" :max="field.max ?? 2147483647" controls-position="right" class="full-width" />
            <el-switch v-else-if="field.type === 'switch'" v-model="proto[field.key]" />
            <el-input v-else-if="field.type === 'textarea'" v-model="proto[field.key]" type="textarea" :rows="5" spellcheck="false" />
            <el-input v-else v-model="proto[field.key]" :type="field.type === 'password' ? 'password' : 'text'" :show-password="field.type === 'password'" :autocomplete="field.type === 'password' ? 'new-password' : 'off'" :placeholder="field.type === 'list' ? t('多个值用逗号分隔', 'Separate values with commas') : undefined" />
          </el-form-item>
        </div>
      </template>
      <el-form-item v-else :label="t('协议参数（JSON）', 'Protocol options (JSON)')" required>
        <el-input v-model="protoJson" type="textarea" :rows="5" spellcheck="false" />
        <span class="field-hint">{{ t('填写该协议需要的内核参数；只有不需要额外参数时才留空 {}。', 'Enter the kernel options required by this protocol; use {} only when no additional options are needed.') }}</span>
      </el-form-item>

      <template v-if="requiresServer">
      <div class="section-heading">{{ t('传输与 TLS', 'Transport and TLS') }}</div>
      <div class="form-grid">
        <el-form-item :label="t('传输方式', 'Network')">
          <el-select v-model="transportType" class="full-width">
            <el-option v-for="item in transportOptions" :key="item" :label="item.toUpperCase()" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="hasTransportDetails" :label="t('路径 / 服务名', 'Path / service name')"><el-input v-model="transportPath" placeholder="/path" /></el-form-item>
        <el-form-item v-if="hasTransportDetails" :label="t('传输 Host', 'Transport host')"><el-input v-model="transportHost" /></el-form-item>
        <el-form-item v-if="protocol === 'openvpn'" :label="t('TLS', 'TLS')"><el-input model-value="TLS" disabled /></el-form-item>
        <el-form-item v-else :label="t('TLS', 'TLS')">
          <el-select v-model="securityType" class="full-width">
            <el-option :label="t('关闭', 'Disabled')" value="none" />
            <el-option label="TLS" value="tls" />
            <el-option v-if="protocol === 'vless'" label="Reality" value="reality" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="securityType !== 'none' && protocol !== 'openvpn'" :label="t('SNI / Server name', 'SNI / Server name')"><el-input v-model="sni" /></el-form-item>
        <el-form-item v-if="securityType !== 'none' && protocol !== 'openvpn'" :label="t('ALPN（逗号分隔）', 'ALPN (comma separated)')"><el-input v-model="alpn" placeholder="h2, http/1.1" /></el-form-item>
        <el-form-item v-if="securityType !== 'none' && protocol !== 'openvpn'" :label="t('指纹', 'Fingerprint')"><el-input v-model="fingerprint" placeholder="chrome" /></el-form-item>
      </div>
      </template>

      <el-collapse class="advanced-options">
        <el-collapse-item :title="t('高级参数（JSON）', 'Advanced options (JSON)')" name="advanced">
          <el-form-item v-if="activeFields.length" :label="t('额外协议字段', 'Additional protocol fields')">
            <el-input v-model="protoExtraJson" type="textarea" :rows="4" spellcheck="false" />
          </el-form-item>
          <el-form-item :label="t('传输选项', 'Transport options')">
            <el-input v-model="transportOptsJson" type="textarea" :rows="3" spellcheck="false" />
          </el-form-item>
          <el-form-item v-if="securityType !== 'none' && protocol !== 'openvpn'" :label="t('TLS 选项（Reality 公钥等）', 'TLS options (Reality public key, etc.)')">
            <el-input v-model="securityOptsJson" type="textarea" :rows="3" spellcheck="false" />
          </el-form-item>
          <el-form-item :label="t('其他内核参数', 'Other kernel options')">
            <el-input v-model="extraJson" type="textarea" :rows="3" spellcheck="false" />
          </el-form-item>
        </el-collapse-item>
      </el-collapse>

      <div class="section-heading">{{ t('管理信息', 'Organization') }}</div>
      <div class="form-grid">
        <el-form-item :label="t('分组', 'Group')"><el-input v-model="group" /></el-form-item>
        <el-form-item :label="t('标签（逗号分隔）', 'Tags (comma separated)')"><el-input v-model="tags" /></el-form-item>
        <el-form-item :label="t('收藏', 'Favorite')"><el-switch v-model="favorite" /></el-form-item>
      </div>

      <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">{{ t('取消', 'Cancel') }}</el-button>
      <el-button type="primary" :loading="saving" @click="save">{{ t('保存节点', 'Save node') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { createNode } from '@/api/nodes'
import type { CreateNodeInput } from '@/api/nodes'
import { useAppearanceStore } from '@/stores/appearance'
import { protocolLabel } from '@/utils/format'

type Field = {
  key: string
  label: string
  labelEn: string
  type?: 'text' | 'password' | 'number' | 'switch' | 'select' | 'list' | 'textarea'
  options?: string[]
  required?: boolean
  wide?: boolean
  min?: number
  max?: number
  defaultValue?: string | number | boolean
}

const categories = [
  { id: 'v2ray', label: 'V2Ray', labelEn: 'V2Ray', protocols: ['vmess', 'vless'] },
  { id: 'ss', label: 'SS', labelEn: 'SS', protocols: ['shadowsocks'] },
  { id: 'ssr', label: 'SSR', labelEn: 'SSR', protocols: ['shadowsocksr'] },
  { id: 'trojan', label: 'Trojan', labelEn: 'Trojan', protocols: ['trojan'] },
  { id: 'hysteria', label: 'Hysteria', labelEn: 'Hysteria', protocols: ['hysteria', 'hysteria2'] },
  { id: 'tuic', label: 'TUIC', labelEn: 'TUIC', protocols: ['tuic'] },
  { id: 'http', label: 'HTTP / SOCKS', labelEn: 'HTTP / SOCKS', protocols: ['http', 'socks5'] },
  { id: 'other', label: '其他', labelEn: 'Other', protocols: ['anytls', 'snell', 'ssh', 'wireguard', 'openvpn', 'masque', 'tailscale', 'shadowtls', 'naive', 'tor', 'openconnect', 'mieru', 'sudoku', 'shadowquic', 'trusttunnel', 'rematch', 'easytier', 'zerotier'] }
]

const fields: Record<string, Field[]> = {
  vmess: [
    { key: 'uuid', label: 'UUID', labelEn: 'UUID', required: true, wide: true },
    { key: 'alter_id', label: 'AlterID', labelEn: 'Alter ID', type: 'number', max: 65535, defaultValue: 0 },
    { key: 'security', label: '加密方式', labelEn: 'Encryption', type: 'select', options: ['auto', 'none', 'aes-128-gcm', 'chacha20-poly1305', 'zero'], defaultValue: 'auto' }
  ],
  vless: [
    { key: 'uuid', label: 'UUID', labelEn: 'UUID', required: true, wide: true },
    { key: 'flow', label: 'Flow', labelEn: 'Flow', type: 'select', options: ['', 'xtls-rprx-vision', 'xtls-rprx-vision-udp443'], defaultValue: '' },
    { key: 'encryption', label: '加密方式', labelEn: 'Encryption', type: 'select', options: ['none'], defaultValue: 'none' }
  ],
  shadowsocks: [
    { key: 'method', label: '加密方式', labelEn: 'Cipher', type: 'select', required: true, options: ['aes-128-gcm', 'aes-192-gcm', 'aes-256-gcm', 'chacha20-ietf-poly1305', 'xchacha20-ietf-poly1305', '2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305', 'none'], defaultValue: 'aes-256-gcm' },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true },
    { key: 'plugin', label: '插件', labelEn: 'Plugin' },
    { key: 'plugin_opts', label: '插件参数', labelEn: 'Plugin options', wide: true },
    { key: 'udp', label: '启用 UDP', labelEn: 'Enable UDP', type: 'switch', defaultValue: true }
  ],
  shadowsocksr: [
    { key: 'method', label: '加密方式', labelEn: 'Cipher', required: true },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true },
    { key: 'protocol', label: '协议', labelEn: 'Protocol', required: true },
    { key: 'protocol_param', label: '协议参数', labelEn: 'Protocol parameter' },
    { key: 'obfs', label: '混淆方式', labelEn: 'Obfuscation', required: true },
    { key: 'obfs-param', label: '混淆参数', labelEn: 'Obfuscation parameter' },
    { key: 'protocol-param', label: '协议参数', labelEn: 'Protocol parameter' }
  ],
  trojan: [
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true, wide: true },
    { key: 'udp', label: '启用 UDP', labelEn: 'Enable UDP', type: 'switch', defaultValue: true }
  ],
  hysteria: [
    { key: 'auth', label: '认证字符串', labelEn: 'Authentication', type: 'password', required: true, wide: true },
    { key: 'up_mbps', label: '上传带宽（Mbps）', labelEn: 'Upload (Mbps)' },
    { key: 'down_mbps', label: '下载带宽（Mbps）', labelEn: 'Download (Mbps)' }
  ],
  hysteria2: [
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true, wide: true },
    { key: 'obfs', label: '混淆方式', labelEn: 'Obfuscation' },
    { key: 'obfs_password', label: '混淆密码', labelEn: 'Obfuscation password', type: 'password' },
    { key: 'port_hopping', label: '端口跳跃', labelEn: 'Port hopping', wide: true }
  ],
  tuic: [
    { key: 'uuid', label: 'UUID', labelEn: 'UUID', required: true },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true },
    { key: 'congestion_control', label: '拥塞控制', labelEn: 'Congestion control', type: 'select', options: ['cubic', 'new_reno', 'bbr'], defaultValue: 'cubic' },
    { key: 'udp_relay_mode', label: 'UDP 中继模式', labelEn: 'UDP relay mode', type: 'select', options: ['native', 'quic'], defaultValue: 'native' }
  ],
  http: [
    { key: 'username', label: '用户名', labelEn: 'Username' },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password' }
  ],
  socks5: [
    { key: 'username', label: '用户名', labelEn: 'Username' },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password' }
  ],
  anytls: [
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true, wide: true },
    { key: 'idle_session_check_interval', label: '空闲检查间隔（秒）', labelEn: 'Idle check interval (s)', type: 'number' },
    { key: 'idle_session_timeout', label: '空闲超时（秒）', labelEn: 'Idle timeout (s)', type: 'number' },
    { key: 'min_idle_session', label: '最小空闲会话数', labelEn: 'Minimum idle sessions', type: 'number' }
  ],
  snell: [
    { key: 'psk', label: 'PSK', labelEn: 'PSK', type: 'password', required: true },
    { key: 'version', label: '版本', labelEn: 'Version', type: 'number', defaultValue: 4 },
    { key: 'obfs', label: '混淆方式', labelEn: 'Obfuscation' },
    { key: 'obfs_host', label: '混淆 Host', labelEn: 'Obfuscation host' }
  ],
  ssh: [
    { key: 'username', label: '用户名', labelEn: 'Username', required: true },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password' },
    { key: 'private_key', label: '私钥（高级）', labelEn: 'Private key (advanced)', type: 'password', wide: true }
  ],
  wireguard: [
    { key: 'private_key', label: '本地私钥', labelEn: 'Local private key', type: 'password', required: true, wide: true },
    { key: 'public_key', label: '对端公钥', labelEn: 'Peer public key', required: true, wide: true },
    { key: 'address', label: '本地地址', labelEn: 'Local addresses', type: 'list', required: true, wide: true },
    { key: 'allowed_ips', label: '允许的 IP', labelEn: 'Allowed IPs', type: 'list', required: true, wide: true },
    { key: 'preshared_key', label: '预共享密钥', labelEn: 'Pre-shared key', type: 'password', wide: true }
  ],
  shadowtls: [
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true },
    { key: 'version', label: '版本', labelEn: 'Version', type: 'number', defaultValue: 3 },
    { key: 'host', label: '握手目标 Host', labelEn: 'Handshake host', wide: true }
  ],
  naive: [
    { key: 'username', label: '用户名', labelEn: 'Username', required: true },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true },
    { key: 'insecure_concurrency', label: '并发数', labelEn: 'Concurrency', type: 'number' },
    { key: 'quic_congestion_control', label: 'QUIC 拥塞控制', labelEn: 'QUIC congestion control', type: 'select', options: ['', 'bbr', 'bbr2', 'cubic', 'reno'] }
  ],
  mieru: [
    { key: 'username', label: '用户名', labelEn: 'Username', required: true },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true },
    { key: 'transport', label: '传输协议', labelEn: 'Transport protocol', type: 'select', options: ['TCP', 'UDP'], defaultValue: 'TCP' }
  ],
  sudoku: [
    { key: 'key', label: '密钥', labelEn: 'Key', type: 'password', required: true },
    { key: 'aead_method', label: 'AEAD 算法', labelEn: 'AEAD method', type: 'select', options: ['chacha20-poly1305', 'aes-128-gcm', 'none'], defaultValue: 'chacha20-poly1305' },
    { key: 'table_type', label: '混淆表', labelEn: 'Table type', type: 'select', options: ['prefer_entropy', 'prefer_ascii', 'up_ascii_down_entropy', 'up_entropy_down_ascii'], defaultValue: 'prefer_entropy' }
  ],
  shadowquic: [
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true },
    { key: 'congestion_controller', label: '拥塞控制', labelEn: 'Congestion controller', type: 'select', options: ['', 'bbr', 'bbr_meta_v1', 'bbr_meta_v2', 'cubic', 'new_reno'] }
  ],
  openvpn: [
    { key: 'username', label: '用户名', labelEn: 'Username' },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password' },
    { key: 'config', label: 'OpenVPN 配置指令', labelEn: 'OpenVPN config directives', type: 'textarea', wide: true }
  ],
  masque: [
    { key: 'private_key', label: '私钥', labelEn: 'Private key', type: 'password', required: true },
    { key: 'public_key', label: '公钥', labelEn: 'Public key', required: true },
    { key: 'uri', label: 'URI', labelEn: 'URI', wide: true },
    { key: 'network', label: '网络', labelEn: 'Network', type: 'select', options: ['', 'ip', 'ip4', 'ip6', 'ip4-ip6', 'ip6-ip4'] },
    { key: 'ip', label: 'IPv4', labelEn: 'IPv4' },
    { key: 'ipv6', label: 'IPv6', labelEn: 'IPv6' }
  ],
  tailscale: [
    { key: 'auth_key', label: 'Auth key', labelEn: 'Auth key', type: 'password', wide: true },
    { key: 'control_url', label: '控制服务器', labelEn: 'Control URL', wide: true },
    { key: 'hostname', label: '设备名称', labelEn: 'Hostname' },
    { key: 'exit_node', label: '出口节点', labelEn: 'Exit node' }
  ],
  tor: [],
  openconnect: [],
  trusttunnel: [
    { key: 'username', label: '用户名', labelEn: 'Username', required: true },
    { key: 'password', label: '密码', labelEn: 'Password', type: 'password', required: true }
  ],
  rematch: [
    { key: 'target_rematch_name', label: '目标 Rematch 名称', labelEn: 'Target Rematch name' },
    { key: 'target_sub_rule', label: '目标子规则', labelEn: 'Target sub-rule' }
  ],
  easytier: [
    { key: 'network_name', label: '网络名称', labelEn: 'Network name', required: true },
    { key: 'network_secret', label: '网络密钥', labelEn: 'Network secret', type: 'password' },
    { key: 'hostname', label: '设备名称', labelEn: 'Hostname' },
  ],
  zerotier: [
    { key: 'network', label: 'Network ID', labelEn: 'Network ID', required: true, wide: true },
    { key: 'identity_secret', label: '身份密钥', labelEn: 'Identity secret', type: 'password', wide: true },
    { key: 'state_dir', label: '状态目录', labelEn: 'State directory', wide: true }
  ]
}

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (event: 'update:modelValue', value: boolean): void; (event: 'saved'): void }>()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const activeCategory = ref('v2ray')
const protocol = ref('vmess')
const name = ref(''), host = ref(''), port = ref(443)
const proto = ref<Record<string, unknown>>({ uuid: '', alter_id: 0, security: 'auto' })
const protoJson = ref('{}'), protoExtraJson = ref('{}')
const transportType = ref('tcp'), transportPath = ref(''), transportHost = ref(''), transportOptsJson = ref('{}')
const securityType = ref('none'), sni = ref(''), alpn = ref(''), fingerprint = ref(''), securityOptsJson = ref('{}'), extraJson = ref('{}')
const group = ref(''), tags = ref(''), favorite = ref(false), error = ref(''), saving = ref(false)

const currentProtocols = computed(() => categories.find(category => category.id === activeCategory.value)?.protocols ?? [])
const activeFields = computed(() => fields[protocol.value] ?? [])
const requiresServer = computed(() => !['tailscale', 'easytier', 'zerotier', 'rematch'].includes(protocol.value))
const hasTransportDetails = computed(() => ['ws', 'grpc', 'http', 'h2', 'xhttp'].includes(transportType.value))
const transportOptions = computed(() => {
  if (protocol.value === 'openvpn') return ['udp', 'tcp']
  if (protocol.value === 'vless') return ['tcp', 'ws', 'grpc', 'http', 'h2', 'xhttp']
  if (['vmess', 'trojan'].includes(protocol.value)) return ['tcp', 'ws', 'grpc', 'http', 'h2']
  if (['hysteria', 'hysteria2', 'tuic', 'shadowquic', 'masque'].includes(protocol.value)) return ['quic']
  if (protocol.value === 'naive') return ['tcp', 'quic']
  return ['tcp']
})

function emptyProto(protocolId: string) {
  return Object.fromEntries((fields[protocolId] ?? []).filter(field => field.defaultValue !== undefined).map(field => [field.key, field.defaultValue])) as Record<string, unknown>
}

function resetProtocolOptions(protocolId: string) {
  proto.value = emptyProto(protocolId)
  protoJson.value = '{}'
  protoExtraJson.value = '{}'
  port.value = ({ shadowsocks: 8388, socks5: 1080, http: 80, ssh: 22, wireguard: 51820, openvpn: 1194 } as Record<string, number>)[protocolId] ?? 443
  transportType.value = ['hysteria', 'hysteria2', 'tuic', 'shadowquic', 'masque'].includes(protocolId) ? 'quic' : protocolId === 'openvpn' ? 'udp' : 'tcp'
  transportPath.value = ''
  transportHost.value = ''
  transportOptsJson.value = '{}'
  securityType.value = ['trojan', 'hysteria', 'hysteria2', 'tuic', 'anytls', 'shadowtls', 'naive', 'shadowquic', 'masque', 'trusttunnel', 'openvpn', 'openconnect'].includes(protocolId) ? 'tls' : 'none'
  sni.value = ''
  alpn.value = ''
  fingerprint.value = ''
  securityOptsJson.value = '{}'
}

function selectCategory(id: string) {
  activeCategory.value = id
  const nextProtocol = categories.find(category => category.id === id)?.protocols[0]
  if (nextProtocol) {
    protocol.value = nextProtocol
    resetProtocolOptions(nextProtocol)
  }
}

function selectProtocol(id: string) {
  resetProtocolOptions(id)
}

function parseObject(raw: string, label: string): Record<string, unknown> {
  let value: unknown
  try { value = JSON.parse(raw) } catch { throw new Error(t(`${label} JSON 格式错误`, `Invalid JSON in ${label}`)) }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(t(`${label} 必须是 JSON 对象`, `${label} must be a JSON object`))
  return value as Record<string, unknown>
}

watch(() => props.modelValue, open => {
  if (!open) return
  activeCategory.value = 'v2ray'
  protocol.value = 'vmess'
  name.value = ''
  host.value = ''
  port.value = 443
  resetProtocolOptions('vmess')
  group.value = ''
  tags.value = ''
  favorite.value = false
  error.value = ''
}, { immediate: true })

async function save() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    const cleanName = name.value.trim(), cleanHost = host.value.trim()
    if (!cleanName) throw new Error(t('请填写节点名称', 'Enter a node name'))
    if (requiresServer.value && (!cleanHost || /[\s/\r\n\0]/.test(cleanHost))) throw new Error(t('请填写有效的服务器地址', 'Enter a valid server host'))
    if (requiresServer.value && (!Number.isInteger(port.value) || port.value < 1 || port.value > 65535)) throw new Error(t('端口范围应为 1–65535', 'Port must be between 1 and 65535'))

    let nodeProto: Record<string, unknown>
    if (activeFields.value.length) {
      for (const field of activeFields.value.filter(field => field.required)) {
        if (proto.value[field.key] == null || String(proto.value[field.key]).trim() === '') throw new Error(t(`请填写${field.label}`, `Enter ${field.labelEn}`))
      }
      if (protocol.value === 'rematch' && !String(proto.value.target_rematch_name ?? '').trim() && !String(proto.value.target_sub_rule ?? '').trim()) throw new Error(t('请填写目标 Rematch 名称或目标子规则', 'Enter a target Rematch name or sub-rule'))
      nodeProto = { ...parseObject(protoExtraJson.value, t('额外协议字段', 'additional protocol fields')), ...proto.value }
    } else {
      nodeProto = parseObject(protoJson.value, t('协议参数', 'protocol options'))
    }
    for (const key of ['address', 'local_address', 'allowed_ips']) {
      if (typeof nodeProto[key] === 'string') nodeProto[key] = (nodeProto[key] as string).split(',').map(value => value.trim()).filter(Boolean)
    }

    const transportOpts = parseObject(transportOptsJson.value, t('传输选项', 'transport options'))
    const securityOpts = securityType.value === 'none' ? {} : parseObject(securityOptsJson.value, t('TLS 选项', 'TLS options'))
    const extra = parseObject(extraJson.value, t('其他内核参数', 'other kernel options'))
    if (['trojan', 'hysteria', 'hysteria2', 'tuic', 'anytls', 'shadowtls', 'naive', 'shadowquic', 'masque', 'trusttunnel', 'openvpn', 'openconnect'].includes(protocol.value) && securityType.value !== 'tls') {
      throw new Error(t('此协议需要启用 TLS', 'This protocol requires TLS'))
    }
    if (protocol.value === 'vless' && securityType.value === 'reality' && typeof securityOpts.public_key !== 'string') {
      throw new Error(t('Reality 配置需要在高级 TLS 选项中填写 public_key', 'Reality requires public_key in advanced TLS options'))
    }
    const input: CreateNodeInput = {
      name: cleanName,
      protocol: protocol.value,
      server: requiresServer.value ? { host: cleanHost, port: port.value } : { host: '', port: 0 },
      transport: { type: transportType.value, ...(transportPath.value ? { path: transportPath.value } : {}), ...(transportHost.value ? { host: transportHost.value } : {}), ...(Object.keys(transportOpts).length ? { opts: transportOpts } : {}) },
      security: { type: securityType.value, ...(sni.value.trim() ? { sni: sni.value.trim() } : {}), ...(alpn.value.trim() ? { alpn: alpn.value.split(',').map(value => value.trim()).filter(Boolean) } : {}), ...(fingerprint.value.trim() ? { fingerprint: fingerprint.value.trim() } : {}), ...(Object.keys(securityOpts).length ? { opts: securityOpts } : {}) },
      proto: nodeProto,
      ...(Object.keys(extra).length ? { extra } : {}),
      group_id: group.value.trim(),
      tags: tags.value.split(/[,，]/).map(value => value.trim()).filter(Boolean),
      favorite: favorite.value
    }
    await createNode(input)
    emit('saved')
    emit('update:modelValue', false)
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : t('创建节点失败', 'Failed to create node')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.protocol-families { display: flex; gap: 4px; overflow-x: auto; margin: -4px 0 18px; border-bottom: 1px solid var(--hairline); scrollbar-width: thin; }
.protocol-families button { flex: 0 0 auto; min-height: 42px; padding: 0 14px; border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--ink-muted); font: inherit; cursor: pointer; }
.protocol-families button:hover { color: var(--primary); }
.protocol-families button.selected { color: var(--primary); border-bottom-color: var(--primary); }
.node-create-form { max-height: min(68vh, 720px); overflow-y: auto; padding: 0 2px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 16px; }
.span-two { grid-column: 1 / -1; }
.full-width { width: 100%; }
.section-heading { margin: 6px 0 14px; padding-top: 14px; border-top: 1px solid var(--hairline); color: var(--ink); font-size: var(--typography-p-body-sm-font-size); font-weight: 600; }
.field-hint { display: block; margin-top: 5px; color: var(--ink-subtle); font-size: var(--typography-p-caption-font-size); line-height: 1.5; }
.advanced-options { margin: 2px 0 16px; }
.advanced-options :deep(.el-collapse-item__content) { padding-bottom: 4px; }
@media (max-width: 600px) { .form-grid { grid-template-columns: minmax(0, 1fr); gap: 0; } .span-two { grid-column: auto; } .node-create-form { max-height: 62vh; } }
</style>
