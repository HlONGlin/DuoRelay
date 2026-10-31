<template>
  <div class="settings">
    <el-alert
      v-if="store.error"
      :title="store.error"
      type="warning"
      show-icon
      :closable="false"
      class="page-alert"
    />

    <el-card v-loading="store.loading" shadow="never" class="settings-card">
      <template #header>
        <div class="card-header">
          <span>基础设置</span>
          <span v-if="store.savedAt" class="saved-at">已保存 {{ formatDate(store.savedAt) }}</span>
        </div>
      </template>

      <el-form :model="form" label-width="140px" class="settings-form">
        <el-divider content-position="left">内核</el-divider>
        <el-form-item label="首选内核">
          <el-radio-group v-model="form.kernel">
            <el-radio-button value="sing-box">sing-box</el-radio-button>
            <el-radio-button value="mihomo">mihomo</el-radio-button>
          </el-radio-group>
          <div class="field-hint">sing-box 与 mihomo 双内核并存，按协议能力自动路由</div>
        </el-form-item>

        <el-divider content-position="left">端口</el-divider>
        <el-form-item label="监听地址">
          <el-input v-model="form.inbound_host" class="short-input" />
        </el-form-item>
        <el-form-item label="混合代理端口">
          <el-input-number v-model="form.inbound_port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item label="Clash API 端口">
          <el-input-number v-model="form.mixed_port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item label="允许局域网">
          <el-switch v-model="form.allow_lan" />
        </el-form-item>

        <el-divider content-position="left">DNS</el-divider>
        <el-form-item label="DNS 服务器">
          <el-input
            v-model="dnsText"
            type="textarea"
            :rows="4"
            placeholder="每行一个 DNS 服务器，例如 223.5.5.5"
          />
        </el-form-item>
        <el-form-item label="Fake-IP">
          <el-switch v-model="form.dns_fake_ip" />
          <div class="field-hint">启用后使用 Fake-IP 模式提升解析速度</div>
        </el-form-item>

        <el-divider content-position="left">高级</el-divider>
        <el-form-item label="日志级别">
          <el-select v-model="form.log_level" class="short-input">
            <el-option label="debug" value="debug" />
            <el-option label="info" value="info" />
            <el-option label="warn" value="warn" />
            <el-option label="error" value="error" />
          </el-select>
        </el-form-item>
        <el-form-item label="TUN 模式">
          <el-switch v-model="form.enable_tun" />
        </el-form-item>
        <el-form-item label="TUN 栈">
          <el-select v-model="form.tun_stack" class="short-input" :disabled="!form.enable_tun">
            <el-option label="system" value="system" />
            <el-option label="gvisor" value="gvisor" />
            <el-option label="mixed" value="mixed" />
          </el-select>
        </el-form-item>
        <el-form-item label="默认出站">
          <el-input v-model="form.default_outbound" class="short-input" />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="store.saving" @click="handleSave">保存设置</el-button>
          <el-button @click="handleReset">恢复默认</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useConfigStore } from '@/stores/config'
import { formatDate } from '@/utils/format'
import type { AppConfig } from '@/types'

const store = useConfigStore()

const form = reactive<AppConfig>({ ...store.config })
const dnsText = ref(store.config.dns_servers.join('\n'))

function syncFromStore(config: AppConfig) {
  Object.assign(form, config)
  dnsText.value = config.dns_servers.join('\n')
}

async function handleSave() {
  const servers = dnsText.value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)

  if (servers.length === 0) {
    ElMessage.warning('请至少配置一个 DNS 服务器')
    return
  }

  try {
    await store.save({ ...form, dns_servers: servers })
    ElMessage.success('设置已保存')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '保存设置失败')
  }
}

function handleReset() {
  store.resetLocal()
  syncFromStore(store.config)
  ElMessage.info('已恢复默认值，点击保存后生效')
}

onMounted(async () => {
  await store.fetchConfig()
  if (!store.error) syncFromStore(store.config)
})
</script>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-alert {
  margin: 0;
}

.settings-card {
  border-radius: 10px;
  max-width: 860px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
}

.saved-at {
  font-size: 12px;
  font-weight: 400;
  color: var(--text-secondary);
}

.settings-form {
  max-width: 620px;
}

.short-input {
  width: 220px;
}

.field-hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

@media (max-width: 768px) {
  .settings-form {
    max-width: 100%;
  }

  .short-input {
    width: 100%;
  }
}
</style>
