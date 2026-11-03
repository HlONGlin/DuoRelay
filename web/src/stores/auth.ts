import { defineStore } from 'pinia'
import { localize } from './appearance'
import { ref } from 'vue'
import { getAuthStatus, login as loginRequest, logout as logoutRequest, register as registerRequest, type AuthStatus } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const status = ref<AuthStatus>({ configured: false, authenticated: false })
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchStatus() {
    loading.value = true
    error.value = null
    try {
      status.value = await getAuthStatus()
      return status.value
    } catch (err) {
      error.value = err instanceof Error ? err.message : localize('无法读取登录状态', 'Unable to read login status')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function login(username: string, password: string) {
    await loginRequest(username, password)
    status.value = { configured: true, authenticated: true, username }
  }

  async function register(username: string, password: string) {
    await registerRequest(username, password)
    status.value = { configured: true, authenticated: false }
  }

  async function logout() {
    await logoutRequest()
    status.value = { configured: true, authenticated: false }
  }

  return { status, loading, error, fetchStatus, login, register, logout }
})
