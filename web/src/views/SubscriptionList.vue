<template>
  <div class="subscription-list">
    <el-alert
      v-if="subscriptions.error"
      :title="subscriptions.error"
      type="error"
      show-icon
      :closable="false"
      class="page-alert"
    />

    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <div class="toolbar-info">
          <span class="total">共 {{ subscriptions.items.length }} 个订阅</span>
          <span class="usage">
            已用 {{ formatBytes(subscriptions.totalUsed) }} /
            {{ formatBytes(subscriptions.totalQuota) }}
          </span>
        </div>
        <div class="toolbar-actions">
          <el-button :loading="subscriptions.loading" @click="refreshList">刷新</el-button>
          <el-button type="primary" @click="openCreate">
            <el-icon><Plus /></el-icon>
            添加订阅
          </el-button>
        </div>
      </div>
    </el-card>

    <div v-if="subscriptions.items.length === 0 && !subscriptions.loading" class="empty-wrap">
      <el-empty description="暂无订阅，添加一个订阅链接开始使用">
        <el-button type="primary" @click="openCreate">添加订阅</el-button>
      </el-empty>
    </div>

    <div v-else v-loading="subscriptions.loading" class="sub-grid">
      <SubscriptionCard
        v-for="item in subscriptions.items"
        :key="item.id"
        :subscription="item"
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
      <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
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
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import SubscriptionCard from '@/components/SubscriptionCard.vue'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import { formatBytes } from '@/utils/format'
import type { Subscription, SubscriptionInput } from '@/types'

const subscriptions = useSubscriptionsStore()

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

.toolbar-card {
  border-radius: 10px;
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
  color: var(--text-secondary);
  font-size: 13px;
}

.total {
  font-weight: 600;
  color: var(--text-primary);
}

.toolbar-actions {
  display: flex;
  gap: 8px;
}

.sub-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
  min-height: 120px;
}

.empty-wrap {
  background: #fff;
  border: 1px solid var(--border-color);
  border-radius: 10px;
}

.interval-hint {
  margin-left: 10px;
  font-size: 12px;
  color: var(--text-secondary);
}

@media (max-width: 768px) {
  .sub-grid {
    grid-template-columns: 1fr;
  }
}
</style>
