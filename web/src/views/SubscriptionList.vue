<template>
  <div class="subscription-list page-stack">
    <div class="page-heading"><h1>{{ t('订阅管理', 'Subscriptions') }}</h1></div>
    <el-alert
      v-if="subscriptions.error"
      :title="subscriptions.error"
      type="error"
      show-icon
      :closable="false"
      class="page-alert"
    />

    <div class="subscription-toolbar">
      <div class="toolbar">
        <div class="toolbar-info">
          <span class="total">{{ appearance.language === 'en' ? `${subscriptions.items.length} subscriptions` : `共 ${subscriptions.items.length} 个订阅` }}</span>
          <span class="usage">
            {{ t('已用', 'Used') }} {{ formatBytes(subscriptions.totalUsed) }} /
            {{ subscriptions.totalQuota ? formatBytes(subscriptions.totalQuota) : t('配额未提供', 'No quota provided') }}
          </span>
        </div>
        <div class="toolbar-actions">
          <el-button :loading="subscriptions.loading" @click="refreshList">{{ t('刷新', 'Refresh') }}</el-button>
          <el-button plain @click="router.push({ path: '/nodes', query: { import: '1' } })">
            {{ t('导入节点链接', 'Import node links') }}
          </el-button>
          <el-button type="primary" @click="openCreate">
            <el-icon><Plus /></el-icon>
            {{ t('添加订阅', 'Add subscription') }}
          </el-button>
        </div>
      </div>
    </div>

    <div v-if="subscriptions.items.length === 0 && !subscriptions.loading" class="empty-wrap">
      <EmptyState :title="t('暂无订阅', 'No subscriptions yet')" :description="t('使用上方「添加订阅」导入订阅地址。', 'Add a subscription to import its URL.')" />
    </div>

    <div v-else v-loading="subscriptions.loading" class="sub-grid">
      <SubscriptionCard
        v-for="item in subscriptions.items"
        :key="item.id"
        :subscription="item"
        :node-count="nodes.error ? undefined : nodeCounts.get(item.id) ?? 0"
        :updating="subscriptions.updatingId === item.id"
        @refresh="handleRefresh"
        @edit="openEdit"
        @remove="handleRemove"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? t('编辑订阅', 'Edit subscription') : t('添加订阅', 'Add subscription')"
      width="520px"
      destroy-on-close
    >
      <el-form @submit.prevent ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item :label="t('名称', 'Name')" prop="name">
          <el-input v-model="form.name" :placeholder="t('例如：机场 A', 'e.g. Provider A')" />
        </el-form-item>
        <el-form-item :label="t('订阅链接', 'Subscription URL')" prop="url">
          <el-input
            v-model="form.url"
            type="textarea"
            :rows="3"
            placeholder="https://example.com/api/v1/client/subscribe?token=..."
          />
          <p class="interval-hint">{{ t('支持 v2rayN 链接订阅、Clash、sing-box、SIP008 及常见客户端节点配置，自动识别内容格式。', 'Supports v2rayN subscriptions, Clash, sing-box, SIP008 and common client configurations. Formats are detected automatically.') }}</p>
        </el-form-item>
        <el-form-item label="User-Agent">
          <el-input v-model="form.user_agent" :placeholder="t('可选，默认使用内置 UA', 'Optional; defaults to the built-in user agent')" />
        </el-form-item>
        <el-form-item :label="t('更新间隔', 'Update interval')">
          <el-input-number v-model="form.interval" :min="0" :max="168" />
          <span class="interval-hint">{{ t('小时，0 表示手动更新', 'hours; 0 means manual updates') }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('取消', 'Cancel') }}</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">
          {{ editingId ? t('保存', 'Save') : t('添加', 'Add') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import EmptyState from '@/components/EmptyState.vue'
import SubscriptionCard from '@/components/SubscriptionCard.vue'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import { useNodesStore } from '@/stores/nodes'
import { useAppearanceStore } from '@/stores/appearance'
import { formatBytes } from '@/utils/format'
import type { Subscription, SubscriptionInput } from '@/types'

const subscriptions = useSubscriptionsStore()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const nodes = useNodesStore()
const nodeCounts = computed(() => {
  const counts = new Map<string, number>()
  for (const node of nodes.items) {
    const id = node.source?.subscription_id
    if (id) counts.set(id, (counts.get(id) ?? 0) + 1)
  }
  return counts
})
const router = useRouter()

const dialogVisible = ref(false)
const submitting = ref(false)
const editingId = ref('')
const formRef = ref<FormInstance>()

const form = reactive<SubscriptionInput>({
  name: '',
  url: '',
  user_agent: '',
  interval: 24
})

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('请输入订阅名称', 'Enter a subscription name'), trigger: 'blur' }],
  url: [
    { required: true, message: t('请输入订阅链接', 'Enter a subscription URL'), trigger: 'blur' },
    {
      pattern: /^https?:\/\//i,
      message: t('订阅链接必须以 http:// 或 https:// 开头', 'The subscription URL must start with http:// or https://'),
      trigger: 'blur'
    }
  ]
}))

function resetForm() {
  editingId.value = ''
  form.name = ''
  form.url = ''
  form.user_agent = ''
  form.interval = 24
  formRef.value?.clearValidate()
}

function openCreate() {
  resetForm()
  dialogVisible.value = true
}

function openEdit(subscription: Subscription) {
  editingId.value = subscription.id
  form.name = subscription.name
  form.url = subscription.url
  form.user_agent = subscription.user_agent ?? ''
  form.interval = subscription.interval ?? 24
  dialogVisible.value = true
}

async function submitForm() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    const payload: SubscriptionInput = {
      name: form.name.trim(),
      url: form.url.trim(),
      user_agent: form.user_agent?.trim() || undefined,
      interval: form.interval ?? 0
    }
    if (editingId.value) {
      await subscriptions.edit(editingId.value, payload)
      ElMessage.success(t('订阅已更新', 'Subscription updated'))
    } else {
      await subscriptions.add(payload)
      ElMessage.success(t('订阅已添加', 'Subscription added'))
    }
    dialogVisible.value = false
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : t('保存订阅失败', 'Failed to save subscription'))
  } finally {
    submitting.value = false
  }
}

async function handleRefresh(subscription: Subscription) {
  try {
    const result = await subscriptions.refresh(subscription.id)
    await nodes.fetchAllNodes()
    const added = result?.nodes_added
    ElMessage.success(
      added === undefined ? t('订阅已更新', 'Subscription updated') : (appearance.language === 'en' ? `Subscription updated; ${added} nodes added` : `订阅已更新，新增 ${added} 个节点`)
    )
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : t('更新订阅失败', 'Failed to update subscription'))
  }
}

async function handleRemove(subscription: Subscription) {
  try {
    await ElMessageBox.confirm(
      appearance.language === 'en' ? `Delete subscription "${subscription.name}" and its nodes?` : `确定删除订阅「${subscription.name}」及其节点吗？`,
      t('删除订阅', 'Delete subscription'),
      { type: 'warning', confirmButtonText: t('删除', 'Delete'), cancelButtonText: t('取消', 'Cancel') }
    )
    await subscriptions.remove(subscription.id)
    ElMessage.success(t('订阅已删除', 'Subscription deleted'))
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err instanceof Error ? err.message : t('删除订阅失败', 'Failed to delete subscription'))
    }
  }
}

function refreshList() {
  void subscriptions.fetchSubscriptions()
  void nodes.fetchAllNodes()
}

onMounted(refreshList)
</script>

<style scoped>
.subscription-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-alert {
  margin: 0;
}

.subscription-toolbar {
  padding-bottom: 16px;
  border-bottom: 1px solid var(--hairline);
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar-info {
  display: flex;
  align-items: center;
  gap: 16px;
  color: var(--ink-subtle);
  font-size: var(--typography-p-caption-font-size);
}

.total {
  color: var(--ink);
  font-weight: 600;
}

.toolbar-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.sub-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
  min-height: 120px;
}

.interval-hint {
  margin-left: 10px;
  color: var(--ink-subtle);
  font-size: var(--typography-p-caption-font-size);
}

@media (max-width: 768px) {
  .sub-grid {
    grid-template-columns: 1fr;
  }
}
</style>
