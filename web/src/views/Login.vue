<template>
  <main class="auth-page">
    <section class="auth-panel">
      <div class="auth-kicker">DuoRelay</div>
      <h1>登录</h1>
      <p class="auth-description">登录后管理节点、订阅和代理内核。</p>
      <el-alert v-if="errorMessage" :title="errorMessage" type="error" :closable="false" show-icon />
      <el-form :model="form" label-position="top" @submit.prevent="submit">
        <el-form-item label="用户名"><el-input v-model="form.username" autocomplete="username" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" type="password" show-password autocomplete="current-password" @keyup.enter="submit" /></el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" class="auth-submit">登录</el-button>
      </el-form>
      <p v-if="!auth.status.configured" class="auth-link-row">首次使用？<router-link to="/register">创建管理员账号</router-link></p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const errorMessage = ref('')
const form = reactive({ username: '', password: '' })

const redirect = computed(() => typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/') ? route.query.redirect : '/')

async function submit() {
  errorMessage.value = ''
  if (!form.username.trim() || !form.password) {
    errorMessage.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    await router.replace(redirect.value)
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page { min-height: 100vh; display: grid; place-items: center; padding: 24px; background: var(--canvas); }
.auth-panel { width: min(100%, 420px); padding: 36px; background: var(--surface-1); border: 1px solid var(--hairline); border-radius: var(--rounded-md); box-shadow: var(--el-box-shadow); }
.auth-kicker { color: var(--primary); font-size: 13px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; margin-bottom: 18px; }
h1 { font-size: 28px; margin-bottom: 8px; }
.auth-description { color: var(--ink-subtle); margin-bottom: 24px; }
.auth-submit { width: 100%; margin-top: 8px; }
.auth-link-row { margin-top: 20px; text-align: center; color: var(--ink-subtle); font-size: 13px; }
@media (max-width: 480px) { .auth-panel { padding: 28px 20px; } }
</style>
