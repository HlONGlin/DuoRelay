<template>
  <div class="page-stack">
    <div class="page-heading"><div><h1>设计系统验证</h1><p>此页面仅在开发环境提供，用于检查设计变量、双轨字阶与交互状态。</p></div></div>
    <section class="panel"><div class="panel-heading"><h2>字体与密度</h2></div><p class="type-headline">营销轨标题 / DuoRelay</p><p class="type-p-headline preview-line">产品轨标题 / 连接概览</p><p class="type-p-body preview-line">中文正文使用产品字阶，数字 <span class="mono">128.5 MB / 88 ms</span> 使用等宽字体。</p></section>
    <div class="theme-grid"><section v-for="theme in ['light','dark']" :key="theme" :data-theme="theme" class="panel theme-sample">
      <div class="panel-heading"><h2>{{ theme === 'light' ? '浅色主题' : '深色主题' }}</h2><span class="neutral-badge">AnyTLS</span></div>
      <div class="sample-stat"><span class="muted type-p-caption">节点总数</span><strong class="type-p-display mono">1,024</strong></div>
      <div class="sample-row" v-for="delay in [88, 180, 320, -1]" :key="delay"><span>示例节点</span><DelayBadge :delay="delay" /></div>
      <div class="preview-line actions"><el-button type="primary" @click="ElMessage.success('样式验证完成')">主操作</el-button><el-button @click="dialog = true">查看弹窗</el-button></div>
      <el-input class="preview-line" placeholder="检查输入框与焦点" />
    </section></div>
    <el-dialog v-model="dialog" title="弹窗样式验证" width="420px"><p>检查边框、表面层级与主题颜色。</p><template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" @click="dialog = false">确定</el-button></template></el-dialog>
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import DelayBadge from '@/components/DelayBadge.vue'
const dialog = ref(false)
</script>
<style scoped>
.preview-line { margin-top: var(--spacing-lg); }
.theme-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--spacing-lg); }
.theme-sample { color: var(--ink); }
.sample-stat { display: flex; flex-direction: column; gap: var(--spacing-xs); margin-bottom: var(--spacing-lg); }
.sample-row { display: flex; justify-content: space-between; height: 44px; align-items: center; border-top: 1px solid var(--hairline); }
@media (max-width: 768px) { .theme-grid { grid-template-columns: 1fr; } }
</style>
