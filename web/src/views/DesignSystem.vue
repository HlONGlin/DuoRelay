<template>
  <div class="page-stack">
    <div class="page-heading"><div><h1>{{ t('设计系统验证', 'Design system') }}</h1><p>{{ t('此页面仅在开发环境提供，用于检查设计变量、双轨字阶与交互状态。', 'Available only in development to review design tokens, type scales and interaction states.') }}</p></div></div>
    <section class="panel"><div class="panel-heading"><h2>{{ t('字体与密度', 'Typography and density') }}</h2></div><p class="type-headline">{{ t('营销轨标题', 'Marketing headline') }} / DuoRelay</p><p class="type-p-headline preview-line">{{ t('产品轨标题', 'Product headline') }} / {{ t('连接概览', 'Connection overview') }}</p><p class="type-p-body preview-line">{{ t('中文正文使用产品字阶，数字', 'Body copy uses the product type scale; values') }} <span class="mono">128.5 MB / 88 ms</span> {{ t('使用等宽字体。', 'use a monospace font.') }}</p></section>
    <div class="theme-grid"><section v-for="theme in ['light','dark']" :key="theme" :data-theme="theme" class="panel theme-sample">
      <div class="panel-heading"><h2>{{ theme === 'light' ? t('浅色主题', 'Light theme') : t('深色主题', 'Dark theme') }}</h2><span class="neutral-badge">AnyTLS</span></div>
      <div class="sample-stat"><span class="muted type-p-caption">{{ t('节点总数', 'Total nodes') }}</span><strong class="type-p-display mono">1,024</strong></div>
      <div class="sample-row" v-for="delay in [88, 180, 320, -1]" :key="delay"><span>{{ t('示例节点', 'Sample node') }}</span><DelayBadge :delay="delay" /></div>
      <div class="preview-line actions"><el-button type="primary" @click="ElMessage.success(t('样式验证完成', 'Style check complete'))">{{ t('主操作', 'Primary action') }}</el-button><el-button @click="dialog = true">{{ t('查看弹窗', 'View dialog') }}</el-button></div>
      <el-input class="preview-line" :placeholder="t('检查输入框与焦点', 'Check input and focus states')" />
    </section></div>
    <el-dialog v-model="dialog" :title="t('弹窗样式验证', 'Dialog style check')" width="420px"><p>{{ t('检查边框、表面层级与主题颜色。', 'Review borders, surface layers and theme colors.') }}</p><template #footer><el-button @click="dialog = false">{{ t('取消', 'Cancel') }}</el-button><el-button type="primary" @click="dialog = false">{{ t('确定', 'OK') }}</el-button></template></el-dialog>
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useAppearanceStore } from '@/stores/appearance'
import DelayBadge from '@/components/DelayBadge.vue'
const dialog = ref(false)
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
</script>
<style scoped>
.preview-line { margin-top: var(--spacing-lg); }
.theme-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--spacing-lg); }
.theme-sample { color: var(--ink); }
.sample-stat { display: flex; flex-direction: column; gap: var(--spacing-xs); margin-bottom: var(--spacing-lg); }
.sample-row { display: flex; justify-content: space-between; height: 44px; align-items: center; border-top: 1px solid var(--hairline); }
@media (max-width: 768px) { .theme-grid { grid-template-columns: 1fr; } }
</style>
