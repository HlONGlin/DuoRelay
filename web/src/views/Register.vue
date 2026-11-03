<template>
  <main class="auth-page">
    <section class="auth-panel">
      <div class="auth-kicker">DuoRelay</div>
      <el-radio-group :model-value="appearance.language" size="small" @update:model-value="appearance.setLanguage($event as 'zh-CN' | 'en')"><el-radio-button value="zh-CN">简体中文</el-radio-button><el-radio-button value="en">English</el-radio-button></el-radio-group>
      <h1>{{ t('首次设置', 'Initial setup') }}</h1>
      <p class="auth-description">{{ t('创建本机管理员账号。账号只保存在当前 DuoRelay 数据库中。', 'Create a local administrator account. It is stored only in this DuoRelay database.') }}</p>
      <el-alert v-if="errorMessage" :title="errorMessage" type="error" :closable="false" show-icon />
      <el-form :model="form" label-position="top" @submit.prevent="submit">
        <el-form-item :label="t('管理员用户名', 'Administrator username')"><el-input v-model="form.username" autocomplete="username" :placeholder="t('例如 admin', 'e.g. admin')" /></el-form-item>
        <el-form-item :label="t('密码', 'Password')"><el-input v-model="form.password" type="password" show-password autocomplete="new-password" /></el-form-item>
        <el-form-item :label="t('确认密码', 'Confirm password')"><el-input v-model="form.confirm" type="password" show-password autocomplete="new-password" @keyup.enter="submit" /></el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" class="auth-submit">{{ t('创建账号', 'Create account') }}</el-button>
      </el-form>
      <p class="auth-link-row">{{ t('已有账号？', 'Already have an account? ') }}<router-link to="/login">{{ t('返回登录', 'Log in') }}</router-link></p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppearanceStore } from '@/stores/appearance'

const auth = useAuthStore()
const appearance = useAppearanceStore()
const t = (zh: string, en: string) => appearance.language === 'en' ? en : zh
const router = useRouter()
const loading = ref(false)
const errorMessage = ref('')
const form = reactive({ username: '', password: '', confirm: '' })

async function submit() {
  errorMessage.value = ''
  if (!form.username.trim() || !form.password) {
    errorMessage.value = t('请完整填写用户名和密码', 'Enter a username and password')
    return
  }
  if (form.password !== form.confirm) {
    errorMessage.value = t('两次输入的密码不一致', 'Passwords do not match')
    return
  }
  loading.value = true
  try {
    await auth.register(form.username, form.password)
    await router.replace('/login')
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : t('注册失败', 'Registration failed')
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
.auth-description { color: var(--ink-subtle); margin-bottom: 24px; line-height: 1.6; }
.auth-submit { width: 100%; margin-top: 8px; }
.auth-link-row { margin-top: 20px; text-align: center; color: var(--ink-subtle); font-size: 13px; }
@media (max-width: 480px) { .auth-panel { padding: 28px 20px; } }
</style>
