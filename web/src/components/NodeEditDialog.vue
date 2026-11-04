<template>
  <el-dialog :model-value="modelValue" :title="t('编辑节点', 'Edit node')" width="660px" destroy-on-close @update:model-value="emit('update:modelValue', $event)">
    <el-alert v-if="node?.source?.type === 'subscription'" :title="t('订阅刷新可能覆盖连接参数；收藏和标签会保留。', 'Subscription refreshes may replace connection settings. Favorites and tags are retained.')" type="info" :closable="false" />
    <el-form label-position="top" @submit.prevent="save">
      <el-form-item :label="t('节点名称', 'Node name')"><el-input v-model="name" maxlength="256" /></el-form-item>
      <template v-if="!isComposition"><el-form-item :label="t('服务器', 'Server')"><el-input v-model="host" /></el-form-item>
      <el-form-item :label="t('端口', 'Port')"><el-input-number v-model="port" :min="1" :max="65535" /></el-form-item></template>
      <el-form-item :label="t('分组', 'Group')"><el-input v-model="group" /></el-form-item>
      <el-form-item :label="t('标签（逗号分隔）', 'Tags (comma separated)')"><el-input v-model="tags" /></el-form-item>
      <el-form-item :label="t('收藏', 'Favorite')"><el-switch v-model="favorite" /></el-form-item>
      <el-collapse v-if="!isComposition"><el-collapse-item :title="t('协议、传输和 TLS 参数（JSON）', 'Protocol, transport and TLS settings (JSON)')"><el-input v-model="advanced" type="textarea" :rows="14" spellcheck="false" /></el-collapse-item></el-collapse>
      <p v-if="isComposition">{{ t('组合成员和策略保存在此节点中；如需修改，请重新生成组合节点。', 'Members and strategy are stored in this node. Create a new composition node to change them.') }}</p>
      <p v-else>{{ t('保存后，重新启动节点以应用连接参数。', 'Restart the node after saving to apply connection settings.') }}</p>
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
    </el-form>
    <template #footer><el-button @click="emit('update:modelValue', false)">{{ t('取消', 'Cancel') }}</el-button><el-button type="primary" :loading="saving" @click="save">{{ t('保存节点', 'Save node') }}</el-button></template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { updateNode } from '@/api/nodes'
import type { ProxyNode } from '@/types'
import { useAppearanceStore } from '@/stores/appearance'
const props = defineProps<{ modelValue: boolean; node: ProxyNode | null }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: boolean): void; (e: 'saved'): void }>()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const isComposition = computed(() => Boolean(props.node?.composition))
const name = ref(''), host = ref(''), port = ref(443), group = ref(''), tags = ref(''), favorite = ref(false), advanced = ref(''), error = ref(''), saving = ref(false)
watch(() => props.modelValue, open => {
  if (!open || !props.node) return
  const n = props.node
  name.value = n.name; host.value = n.server.host; port.value = n.server.port; group.value = n.group_id ?? ''; tags.value = (n.tags ?? []).join(', '); favorite.value = n.favorite ?? false
  advanced.value = JSON.stringify({ proto: n.proto ?? {}, transport: n.transport ?? { type: 'tcp' }, security: n.security ?? { type: 'none' }, extra: n.extra ?? {} }, null, 2)
  error.value = ''
}, { immediate: true })
async function save() {
  if (!props.node || saving.value) return
  saving.value = true; error.value = ''
  try {
    if (!name.value.trim()) throw new Error(t('请填写节点名称', 'Enter a node name'))
    const data: Record<string, unknown> = { name: name.value.trim(), group_id: group.value, tags: tags.value.split(/[,，]/).map(v => v.trim()).filter(Boolean), favorite: favorite.value }
    if (!isComposition.value) {
      const parsed = JSON.parse(advanced.value) as Record<string, unknown>
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object' || Object.keys(parsed).some(k => !['proto', 'transport', 'security', 'extra'].includes(k))) throw new Error(t('JSON 仅允许 proto、transport、security、extra 字段', 'JSON allows only proto, transport, security and extra'))
      Object.assign(data, parsed, { server: { host: host.value.trim(), port: port.value } })
    }
    await updateNode(props.node.id, data)
    emit('saved'); emit('update:modelValue', false)
  } catch (err) { error.value = err instanceof Error ? err.message : t('保存失败', 'Save failed') }
  finally { saving.value = false }
}
</script>
