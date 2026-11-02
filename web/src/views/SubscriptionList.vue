<template>
  <div class="subscription-list page-stack">
    <div class="page-heading"><h1>订阅管理</h1></div>
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
          <span class="total">共 {{ subscriptions.items.length }} 个订阅</span>
          <span class="usage">
            已用 {{ formatBytes(subscriptions.totalUsed) }} /
            {{ subscriptions.totalQuota ? formatBytes(subscriptions.totalQuota) : '配额未提供' }}
          </span>
        </div>
        <div class="toolbar-actions">
          <el-button :loading="subscriptions.loading" @click="refreshList">刷新</el-button>
          <el-button plain @click="router.push({ path: '/nodes', query: { import: '1' } })">
            导入节点链接
          </el-button>
          <el-button type="primary" @click="openCreate">
            <el-icon><Plus /></el-icon>
            添加订阅
          </el-button>
        </div>
      </div>
    </div>

    <div v-if="subscriptions.items.length === 0 && !subscriptions.loading" class="empty-wrap">
      <EmptyState title="暂无订阅" description="使用上方「添加订阅」导入订阅地址。" />
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
      :title="editingId ? '编辑订阅' : '添加订阅'"
      width="520px"
      destroy-on-close
    >
      <el-form @submit.prevent ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如：机场 A" />
        </el-form-item>
        <el-form-item label="订阅链接" prop="url">
          <el-input
            v-model="form.url"
            type="textarea"
            :rows="3"
            placeholder="https://example.com/api/v1/client/subscribe?token=..."
          />
          <p class="interval-hint">支持 v2rayN 链接订阅、Clash、sing-box、SIP008 及常见客户端节点配置，自动识别内容格式。</p>
        </el-form-item>
        <el-form-item label="User-Agent">
          <el-input v-model="form.user_agent" placeholder="可选，默认使用内置 UA" />
        </el-form-item>
        <el-form-item label="更新间隔">
          <el-input-number v-model="form.interval" :min="0" :max="168" />
          <span class="interval-hint">小时，0 表示手动更新</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">
          {{ editingId ? '保存' : '添加' }}
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
import { formatBytes } from '@/utils/format'
import type { Subscription, SubscriptionInput } from '@/types'

const subscriptions = useSubscriptionsStore()
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

const rules: FormRules = {
  name: [{ required: true, message: '请输入订阅名称', trigger: 'blur' }],
  url: [
    { required: true, message: '请输入订阅链接', trigger: 'blur' },
    {
      pattern: /^https?:\/\//i,
      message: '订阅链接必须以 http:// 或 https:// 开头',
      trigger: 'blur'
    }
  ]
}

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
      ElMessage.success('订阅已更新')
    } else {
      await subscriptions.add(payload)
      ElMessage.success('订阅已添加')
    }
    dialogVisible.value = false
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '保存订阅失败')
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
      added === undefined ? '订阅已更新' : `订阅已更新，新增 ${added} 个节点`
    )
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '更新订阅失败')
  }
}

async function handleRemove(subscription: Subscription) {
  try {
    await ElMessageBox.confirm(
      `确定删除订阅「${subscription.name}」及其节点吗？`,
      '删除订阅',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
    )
    await subscriptions.remove(subscription.id)
    ElMessage.success('订阅已删除')
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err instanceof Error ? err.message : '删除订阅失败')
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
