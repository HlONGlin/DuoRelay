import { ref } from 'vue'
import { defineStore } from 'pinia'

export type Theme = 'light' | 'dark'
const storageKey = 'duorelay-theme'

export function readTheme(): Theme {
  try { return localStorage.getItem(storageKey) === 'dark' ? 'dark' : 'light' }
  catch { return 'light' }
}

export function initializeTheme() {
  document.documentElement.dataset.theme = readTheme()
}

export const useAppearanceStore = defineStore('appearance', () => {
  const theme = ref<Theme>(readTheme())
  function setTheme(next: Theme) {
    theme.value = next
    document.documentElement.dataset.theme = next
    try { localStorage.setItem(storageKey, next) } catch { /* Theme still works without storage. */ }
  }
  function toggleTheme() { setTheme(theme.value === 'dark' ? 'light' : 'dark') }
  return { theme, setTheme, toggleTheme }
})
