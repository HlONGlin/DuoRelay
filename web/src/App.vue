<template>
  <el-config-provider :locale="appearance.language === 'en' ? en : zhCn">
    <AppShell />
  </el-config-provider>
</template>

<script setup lang="ts">
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import { useAppearanceStore } from '@/stores/appearance'
import AppShell from '@/components/AppShell.vue'
const appearance = useAppearanceStore()
</script>
