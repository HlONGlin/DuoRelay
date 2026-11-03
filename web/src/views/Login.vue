<template>
  <main class="auth-page">
    <section class="auth-panel">
      <div class="auth-kicker">DuoRelay</div>
      <el-radio-group :model-value="appearance.language" size="small" @update:model-value="appearance.setLanguage($event as 'zh-CN' | 'en')"><el-radio-button value="zh-CN">简体中文</el-radio-button><el-radio-button value="en">English</el-radio-button></el-radio-group>
      <h1>{{ t('登录', 'Log in') }}</h1>
      <p class="auth-description">{{ t('登录后管理节点、订阅和代理内核。', 'Log in to manage nodes, subscriptions and proxy kernels.') }}</p>
      <el-alert v-if="errorMessage" :title="errorMessage" type="error" :closable="false" show-icon />
      <el-form :model="form" label-position="top" @submit.prevent="submit">
        <el-form-item :label="t('用户名', 'Username')"><el-input v-model="form.username" autocomplete="username" /></el-form-item>
        <el-form-item :label="t('密码', 'Password')"><el-input v-model="form.password" type="password" show-password autocomplete="current-password" @keyup.enter="submit" /></el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" class="auth-submit">{{ t('登录', 'Log in') }}</el-button>
      </el-form>
      <p v-if="!auth.status.configured" class="auth-link-row">{{ t('首次使用？', 'First time? ') }}<router-link to="/register">{{ t('创建管理员账号', 'Create an administrator account') }}</router-link></p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppearanceStore } from '@/stores/appearance'

const auth = useAuthStore()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const errorMessage = ref('')
const form = reactive({ username: '', password: '' })

const redirect = computed(() => typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/') ? route.query.redirect : '/')

async function submit() {
  errorMessage.value = ''
  if (!form.username.trim() || !form.password) {
    errorMessage.value = t('请输入用户名和密码', 'Enter your username and password')
    return
  }
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    await router.replace(redirect.value)
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : t('登录失败', 'Login failed')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page { min-height: 100vh; display: grid; place-items: center; padding: 24px; background: var(--canvas); }
.auth-panel { width: min(100%, 420px); padding: 36px; background: var(--surface-1); border: 1px solid var(--hairline); border-radius: var(--rounded-md); box-shadow: var(--el-box-shadow); }
.auth-kicker { color: var(--primary); font-size: 13px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; margin-bottom: 18px; }
.auth-panel > .el-radio-group { margin-bottom: 18px; }
h1 { font-size: 28px; margin-bottom: 8px; }
.auth-description { color: var(--ink-subtle); margin-bottom: 24px; }
.auth-submit { width: 100%; margin-top: 8px; }
.auth-link-row { margin-top: 20px; text-align: center; color: var(--ink-subtle); font-size: 13px; }
@media (max-width: 480px) { .auth-panel { padding: 28px 20px; } }
</style>
